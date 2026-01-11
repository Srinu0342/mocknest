package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"mocknest/server/handler"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		method := handler.Handler(r.Method) // Example usage of the handler package

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

		response := map[string]any{
			"check":  "ok",
			"method": method,
			"path":   r.URL.Path,
			"query":  r.URL.Query(),
			"body":   body,
			"time":   time.Now(),
		}

		log.Println(r.URL.Path)
		log.Println(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(response); err != nil {
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
