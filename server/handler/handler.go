package handler

import (
	"time"

	"github.com/Srinu0342/mocknest/server/appdata"
)

// Handler is the main entrypoint for matching an HTTP request against the
// loaded mock mappings. It returns the HTTP status, headers, and body to send.
func Handler(req appdata.IncomingRequest) (int, map[string]string, any) {
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

	// Record the call in global in-memory history.
	appdata.RecordCall(appdata.CallRecord{
		Time:        time.Now(),
		Method:      req.Method,
		URL:         req.URL,
		Query:       req.Query,
		RequestBody: req.Body,
		MappingID:   mappingID,
		Status:      status,
	})

	return status, headers, respBody
}

func httpStatusNotFound() int {
	// Avoid importing net/http just for the constant; keep it simple.
	return 404
}
