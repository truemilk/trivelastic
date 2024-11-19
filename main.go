package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/truemilk/trivelastic/internal/logger"
)

// Add Elasticsearch configuration struct
type ElasticsearchConfig struct {
	URL    string
	APIKey string
	Index  string
}

// Get Elasticsearch config from environment variables
func getESConfig() (*ElasticsearchConfig, error) {
	url := os.Getenv("ES_URL")
	apiKey := os.Getenv("ES_API_KEY")
	index := os.Getenv("ES_INDEX")

	if url == "" || apiKey == "" || index == "" {
		return nil, fmt.Errorf("missing required environment variables: ES_URL, ES_API_KEY, ES_INDEX")
	}

	logger.Info("Elasticsearch configuration loaded",
		"url", url,
		"index", index,
	)

	return &ElasticsearchConfig{
		URL:    url,
		APIKey: apiKey,
		Index:  index,
	}, nil
}

// Add new helper function
func sanitizeJSON(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for key, value := range data {
		// Skip fields that are just dots
		if key == "." || key == ".." {
			continue
		}

		// Handle empty date fields
		if key == "lastModifiedDate" && (value == "" || value == nil) {
			continue // Skip empty date fields
		}

		// Recursively handle nested objects
		switch v := value.(type) {
		case map[string]interface{}:
			sanitized := sanitizeJSON(v)
			if len(sanitized) > 0 { // Only add non-empty objects
				result[key] = sanitized
			}
		case []interface{}:
			sanitized := sanitizeArray(v)
			if len(sanitized) > 0 { // Only add non-empty arrays
				result[key] = sanitized
			}
		case string:
			if v != "" { // Only add non-empty strings
				result[key] = value
			}
		default:
			if value != nil { // Only add non-nil values
				result[key] = value
			}
		}
	}
	return result
}

// Helper function for arrays
func sanitizeArray(arr []interface{}) []interface{} {
	result := make([]interface{}, 0, len(arr))
	for _, value := range arr {
		switch v := value.(type) {
		case map[string]interface{}:
			result = append(result, sanitizeJSON(v))
		case []interface{}:
			result = append(result, sanitizeArray(v))
		default:
			result = append(result, value)
		}
	}
	return result
}

// Request represents a job to be processed
type Request struct {
	w    http.ResponseWriter
	r    *http.Request
	done chan bool
}

// Add new constants for retry configuration
const (
	maxRetries    = 3
	retryInterval = 1 * time.Second
)

// processRequest handles the actual request processing
func processRequest(req *Request) {
	log := logger.GetLogger("request_processor")
	defer func() {
		req.done <- true
	}()

	w, r := req.w, req.r

	// Set response header to JSON
	w.Header().Set("Content-Type", "application/json")

	// Only process POST requests with JSON
	if r.Method != http.MethodPost {
		log.Warn().Str("method", r.Method).Msg("Invalid HTTP method")
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the raw JSON body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read request body")
		http.Error(w, "Error reading body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Log the raw JSON
	log.Debug().RawJSON("raw_json", body).Msg("Received JSON payload")

	// Parse the JSON into a map
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		log.Error().Err(err).Msg("Failed to parse JSON")
		http.Error(w, "Error parsing JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Sanitize the JSON
	cleanData := sanitizeJSON(data)
	log.Debug().Interface("clean_data", cleanData).Msg("JSON sanitized")

	// Convert back to JSON for Elasticsearch
	cleanBody, err := json.Marshal(cleanData)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal sanitized JSON")
		http.Error(w, "Error processing JSON: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Forward to Elasticsearch
	esConfig, err := getESConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get Elasticsearch configuration")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Create Elasticsearch request with cleaned data
	esURL := fmt.Sprintf("%s/%s/_doc", esConfig.URL, esConfig.Index)
	req2, err := http.NewRequest("POST", esURL, bytes.NewBuffer(cleanBody))
	if err != nil {
		log.Error().Err(err).Str("url", esURL).Msg("Failed to create Elasticsearch request")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set headers for Elasticsearch
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", fmt.Sprintf("ApiKey %s", esConfig.APIKey))

	// Create custom HTTP client with TLS configuration
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	esClient := &http.Client{Transport: tr}

	// Implement retry logic
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Debug().Int("attempt", attempt).Msg("Attempting Elasticsearch request")

		resp, err := esClient.Do(req2)
		if err != nil {
			lastErr = err
			log.Warn().Err(err).Int("attempt", attempt).Msg("Elasticsearch request failed")
			if attempt < maxRetries {
				time.Sleep(retryInterval)
				continue
			}
			break
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			// Read the error response body
			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Error().Err(err).Msg("Failed to read Elasticsearch error response")
			} else {
				log.Error().
					Int("status_code", resp.StatusCode).
					RawJSON("response", respBody).
					Msg("Elasticsearch error response")
			}

			if attempt < maxRetries {
				time.Sleep(retryInterval)
				continue
			}
			lastErr = fmt.Errorf("elasticsearch error: status=%d", resp.StatusCode)
			break
		}

		// Success case
		log.Info().Int("attempt", attempt).Msg("Successfully forwarded to Elasticsearch")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "success",
			"message": "Data processed successfully",
			"data":    data,
		})
		return
	}

	// If we get here, all retries failed
	log.Error().Err(lastErr).Msg("All retries failed for Elasticsearch request")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "warning",
		"message": "Request processed but failed to store in Elasticsearch",
		"data":    data,
	})
}

// worker processes requests from the request channel
func worker(requests <-chan *Request) {
	log := logger.GetLogger("worker")
	for req := range requests {
		log.Debug().Msg("Processing new request")
		processRequest(req)
	}
}

func handler(requests chan<- *Request) http.HandlerFunc {
	log := logger.GetLogger("handler")
	return func(w http.ResponseWriter, r *http.Request) {
		log.Debug().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Msg("Received request")

		done := make(chan bool)
		req := &Request{
			w:    w,
			r:    r,
			done: done,
		}
		requests <- req
		<-done // Wait for request to be processed
	}
}

func main() {
	// Initialize logger
	err := logger.Initialize(logger.Config{
		Level:      os.Getenv("LOG_LEVEL"),
		JSONFormat: os.Getenv("LOG_FORMAT") == "json",
	})
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	log := logger.GetLogger("main")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Create a buffered channel for requests
	numWorkers := runtime.NumCPU() * 2 // Use twice the number of CPUs as workers
	requests := make(chan *Request, numWorkers)

	// Start worker pool
	for i := 0; i < numWorkers; i++ {
		go worker(requests)
	}

	// Set up the HTTP server with the concurrent handler
	http.HandleFunc("/", handler(requests))

	log.Info().
		Str("port", port).
		Int("workers", numWorkers).
		Msg("Server starting")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal().Err(err).Msg("Server failed to start")
	}
}
