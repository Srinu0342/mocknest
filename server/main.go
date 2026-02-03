package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"mocknest/server/appdata"
	"mocknest/server/generator"
	"mocknest/server/handler"
)

func main() {
	generator.GenerateMappings()
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
		port = "8080"
	}

	log.Println("listening on port:", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
