package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Srinu0342/mocknest/server/appdata"
)

// Handler is the main entrypoint for matching an HTTP request against the
// loaded mock mappings. It returns the HTTP status, headers, body, and the
// matched mapping ID.
func Handler(req appdata.IncomingRequest) (int, map[string]string, any, string) {
	mapping, ok := appdata.Global.FindBestMatch(req)

	var (
		status    int
		headers   map[string]string
		respBody  any
		mappingID string
	)

	if !ok {
		// No mapping matched: return a simple 404 JSON body.
		status = httpStatusNotFound()
		headers = map[string]string{
			"Content-Type": "application/json",
		}
		respBody = map[string]any{
			"error":  "no mock mapping found",
			"method": req.Method,
			"url":    req.URL,
		}
	} else {
		mappingID = mapping.ID
		resp := mapping.Response
		status = resp.Status
		if status == 0 {
			status = 200
		}

		// Optional artificial delay for simulating latency.
		if resp.FixedDelayMs > 0 {
			time.Sleep(time.Duration(resp.FixedDelayMs) * time.Millisecond)
		}

		if resp.Webhook != nil {
			if err := sendWebhookWithRetry(mapping.ID, resp.Webhook); err != nil {
				log.Printf("mapping=%q webhook error: %v", mapping.ID, err)
			}
		}

		headers = make(map[string]string, len(resp.Headers))
		for k, v := range resp.Headers {
			headers[k] = v
		}
		// Ensure Content-Type is set for JSON responses if not provided.
		if _, ok := headers["Content-Type"]; !ok {
			headers["Content-Type"] = "application/json"
		}
		respBody = resp.Body
	}

	return status, headers, respBody, mappingID
}

func sendWebhookWithRetry(mappingID string, cfg *appdata.Webhook) error {
	if cfg == nil {
		return nil
	}

	if strings.TrimSpace(cfg.URL) == "" {
		return fmt.Errorf("webhook URL is required")
	}

	method := strings.ToUpper(strings.TrimSpace(cfg.Method))
	if method == "" {
		method = http.MethodPost
	}

	retries := cfg.Retries
	if retries < 0 {
		retries = 0
	}
	if retries > 3 {
		retries = 3
	}
	attempts := retries + 1

	timeout := 5 * time.Second
	if cfg.TimeoutMs > 0 {
		timeout = time.Duration(cfg.TimeoutMs) * time.Millisecond
	}

	var bodyBytes []byte
	if cfg.Body != nil {
		var err error
		bodyBytes, err = json.Marshal(cfg.Body)
		if err != nil {
			return fmt.Errorf("failed to marshal webhook body: %w", err)
		}
	}

	client := &http.Client{Timeout: timeout}
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		var body io.Reader
		if bodyBytes != nil {
			body = bytes.NewReader(bodyBytes)
		}
		req, err := http.NewRequest(method, cfg.URL, body)
		if err != nil {
			return err
		}

		for k, v := range cfg.Headers {
			if strings.TrimSpace(k) == "" {
				continue
			}
			req.Header.Set(k, v)
		}
		if cfg.Body != nil && !hasContentTypeHeader(cfg.Headers) {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
		} else {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
		}

		if attempt <= retries {
			time.Sleep(200 * time.Millisecond)
		}
	}

	return fmt.Errorf("webhook failed after %d attempts: %w", attempts, lastErr)
}

func hasContentTypeHeader(headers map[string]string) bool {
	for k := range headers {
		if strings.EqualFold(k, "Content-Type") {
			return true
		}
	}
	return false
}

func httpStatusNotFound() int {
	// Avoid importing net/http just for the constant; keep it simple.
	return 404
}
