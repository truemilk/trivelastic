package worker

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/rs/zerolog"
	"github.com/truemilk/trivelastic/internal/elasticsearch"
	"github.com/truemilk/trivelastic/internal/logger"
	"github.com/truemilk/trivelastic/pkg/sanitizer"
)

type Request struct {
	W    http.ResponseWriter
	R    *http.Request
	Done chan bool
}

type Pool struct {
	requests chan *Request
	es       *elasticsearch.Client
	log      zerolog.Logger
}

func NewPool(numWorkers int) *Pool {
	pool := &Pool{
		requests: make(chan *Request, numWorkers),
		log:      logger.GetLogger("worker_pool"),
	}

	pool.log.Info().
		Int("workers", numWorkers).
		Msg("Initializing worker pool")

	// Start worker pool
	for i := 0; i < numWorkers; i++ {
		go pool.worker(i)
	}

	return pool
}

func (p *Pool) SetElasticsearchClient(client *elasticsearch.Client) {
	p.es = client
	p.log.Info().Msg("Elasticsearch client configured for worker pool")
}

func (p *Pool) Submit(w http.ResponseWriter, r *http.Request) {
	p.log.Debug().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Msg("Submitting request to worker pool")

	done := make(chan bool)
	req := &Request{
		W:    w,
		R:    r,
		Done: done,
	}
	p.requests <- req
	<-done // Wait for request to be processed
}

func (p *Pool) worker(id int) {
	log := p.log.With().Int("worker_id", id).Logger()
	log.Debug().Msg("Worker started")

	for req := range p.requests {
		log.Debug().Msg("Processing new request")
		p.processRequest(req, log)
	}
}

func (p *Pool) processRequest(req *Request, log zerolog.Logger) {
	defer func() {
		req.Done <- true
	}()

	w, r := req.W, req.R

	// Set response header to JSON
	w.Header().Set("Content-Type", "application/json")

	// Only process POST requests with JSON
	if r.Method != http.MethodPost {
		log.Warn().
			Str("method", r.Method).
			Msg("Invalid HTTP method")
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the raw JSON body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().
			Err(err).
			Msg("Failed to read request body")
		http.Error(w, "Error reading body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Log the raw JSON at debug level
	log.Debug().
		RawJSON("raw_json", body).
		Msg("Received JSON payload")

	// Parse the JSON into a map
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		log.Error().
			Err(err).
			Msg("Failed to parse JSON")
		http.Error(w, "Error parsing JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Sanitize the JSON
	cleanData := sanitizer.SanitizeJSON(data)
	log.Debug().
		Interface("clean_data", cleanData).
		Msg("JSON sanitized")

	// Forward to Elasticsearch
	if err := p.es.IndexDocument(cleanData); err != nil {
		log.Error().
			Err(err).
			Msg("Failed to index document in Elasticsearch")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "warning",
			"message": "Request processed but failed to store in Elasticsearch",
			"data":    cleanData,
		})
		return
	}

	log.Info().Msg("Request processed successfully")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Data processed successfully",
		"data":    cleanData,
	})
}
