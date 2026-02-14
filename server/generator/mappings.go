package generator

import (
	"encoding/json"
	"log"

	"github.com/Srinu0342/mocknest/server/appdata"
)

func GenerateMappings() {
	log.Println("Loading mocks into runtime index...")

	data, error := loadMocks("mocks")
	if error != nil {
		log.Fatal("Failed to load mocks:", error)
	}

	appdata.Global.Reset()
	appdata.ResetMappings()

	loaded := 0
	for i, item := range data {
		// loadMocks currently unmarshals into map[string]any.
		// Re-marshal to JSON and unmarshal into the strict Mapping struct.
		b, err := json.Marshal(item)
		if err != nil {
			log.Printf("Skipping mock[%d]: marshal failed: %v", i, err)
			continue
		}

		var m appdata.Mapping
		if err := json.Unmarshal(b, &m); err != nil {
			log.Printf("Skipping mock[%d]: invalid mapping json: %v", i, err)
			continue
		}

		if err := appdata.Global.Add(m); err != nil {
			log.Printf("Skipping mapping id=%q: %v", m.ID, err)
			continue
		}
		appdata.RegisterMapping(m)
		loaded++
	}

	log.Printf("Mappings loaded: %d/%d (runtime index count=%d) done", loaded, len(data), appdata.Global.Count())
}
