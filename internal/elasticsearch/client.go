package elasticsearch

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"github.com/truemilk/trivelastic/internal/config"
	"github.com/truemilk/trivelastic/internal/logger"
)

const (
	maxRetries    = 3
	retryInterval = 1 * time.Second
)

type Client struct {
	config *config.ElasticsearchConfig
	client *http.Client
	log    zerolog.Logger
}

func NewClient(cfg *config.ElasticsearchConfig) *Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	return &Client{
		config: cfg,
		client: &http.Client{Transport: tr},
		log:    logger.GetLogger("elasticsearch"),
	}
}

func (c *Client) IndexDocument(data map[string]interface{}) error {
	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshaling data: %w", err)
	}

	esURL := fmt.Sprintf("%s/%s/_doc", c.config.URL, c.config.Index)
	c.log.Debug().
		Str("url", esURL).
		RawJSON("body", body).
		Msg("Preparing to index document")

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := c.sendRequest(esURL, body); err != nil {
			lastErr = err
			c.log.Warn().
				Err(err).
				Int("attempt", attempt).
				Int("max_retries", maxRetries).
				Msg("Indexing attempt failed")

			if attempt < maxRetries {
				time.Sleep(retryInterval)
				continue
			}
			break
		}
		c.log.Info().
			Int("attempt", attempt).
			Str("index", c.config.Index).
			Msg("Document indexed successfully")
		return nil
	}

	c.log.Error().
		Err(lastErr).
		Str("url", esURL).
		Str("index", c.config.Index).
		Msg("All indexing attempts failed")

	return fmt.Errorf("all retries failed: %w", lastErr)
}

func (c *Client) sendRequest(url string, body []byte) error {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("ApiKey %s", c.config.APIKey))

	c.log.Debug().
		Str("url", url).
		Msg("Sending request to Elasticsearch")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			c.log.Error().
				Err(err).
				Int("status_code", resp.StatusCode).
				Msg("Failed to read error response body")
			return fmt.Errorf("elasticsearch error: status=%d, failed to read response", resp.StatusCode)
		}

		c.log.Error().
			Int("status_code", resp.StatusCode).
			RawJSON("response", respBody).
			Msg("Elasticsearch request failed")

		return fmt.Errorf("elasticsearch error: status=%d, response=%s", resp.StatusCode, string(respBody))
	}

	return nil
}
