package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]any{
			"check":  "ok",
			"method": r.Method,
			"path":   r.URL.Path,
			"time":   time.Now(),
		}
		log.Println(r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "failed to encode json", http.StatusInternalServerError)
		}
	})

	log.Println("listening on port: 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
