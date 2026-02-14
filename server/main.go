package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/Srinu0342/mocknest/server/appdata"
	"github.com/Srinu0342/mocknest/server/generator"
	"github.com/Srinu0342/mocknest/server/handler"
)

func main() {
	generator.GenerateMappings()

	// Admin endpoints
	http.HandleFunc("/__admin/mocks", func(w http.ResponseWriter, r *http.Request) {
		mocks := appdata.GetAllMappings()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(mocks); err != nil {
			http.Error(w, "failed to encode mocks json", http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/__admin/history", func(w http.ResponseWriter, r *http.Request) {
		history := appdata.GetCallHistory()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(history); err != nil {
			http.Error(w, "failed to encode history json", http.StatusInternalServerError)
			return
		}
	})

	// Catch-all mock handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
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

		status, headers, respBody := handler.Handler(incoming)

		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)

		if err := json.NewEncoder(w).Encode(respBody); err != nil {
			http.Error(w, "failed to encode json", http.StatusInternalServerError)
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8342"
	}

	log.Println("listening on port:", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
