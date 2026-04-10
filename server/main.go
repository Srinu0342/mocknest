package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Srinu0342/mocknest/server/appdata"
	"github.com/Srinu0342/mocknest/server/generator"
	"github.com/Srinu0342/mocknest/server/handler"
)

func main() {
	generator.GenerateMappings()

	// Admin endpoints
	http.HandleFunc("/__admin/mocks", func(w http.ResponseWriter, r *http.Request) {
		cid := newCorrelationID()
		lrw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		defer log.Printf("cid=%s method=%s path=%s mock=%s status=%d", cid, r.Method, r.URL.Path, "-", lrw.status)

		mocks := appdata.GetAllMappings()
		lrw.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(lrw).Encode(mocks); err != nil {
			http.Error(lrw, "failed to encode mocks json", http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/__admin/history", func(w http.ResponseWriter, r *http.Request) {
		cid := newCorrelationID()
		lrw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		defer log.Printf("cid=%s method=%s path=%s mock=%s status=%d", cid, r.Method, r.URL.Path, "-", lrw.status)

		history := appdata.GetCallHistory()
		lrw.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(lrw).Encode(history); err != nil {
			http.Error(lrw, "failed to encode history json", http.StatusInternalServerError)
			return
		}
	})

	// Catch-all mock handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		cid := newCorrelationID()
		lrw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		var mappingID string
		defer log.Printf("cid=%s method=%s path=%s mock=%s status=%d", cid, r.Method, r.URL.Path, mappingID, lrw.status)

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(lrw, "failed to read body", http.StatusBadRequest)
			return
		}

		var body any
		if len(bodyBytes) > 0 {
			if err := json.Unmarshal(bodyBytes, &body); err != nil {
				body = string(bodyBytes)
			}
		}

		incoming := appdata.IncomingRequest{
			Method: r.Method,
			// Use RequestURI so query string is visible for debugging; matching uses URL + Query.
			URL:   r.URL.Path,
			Query: r.URL.Query(),
			Body:  body,
		}

		status, headers, respBody, mappingID := handler.Handler(incoming)

		for k, v := range headers {
			lrw.Header().Set(k, v)
		}
		lrw.WriteHeader(status)

		if err := json.NewEncoder(lrw).Encode(respBody); err != nil {
			http.Error(lrw, "failed to encode json", http.StatusInternalServerError)
		}

		appdata.RecordCall(appdata.CallRecord{
			Time:          time.Now(),
			Method:        r.Method,
			URL:           r.URL.Path,
			Query:         r.URL.Query(),
			RequestBody:   body,
			MappingID:     mappingID,
			CorrelationID: cid,
			Status:        status,
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8342"
	}

	log.Println("listening on port:", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.status = code
	lrw.ResponseWriter.WriteHeader(code)
}

func newCorrelationID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
