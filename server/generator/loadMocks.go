package generator

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type Mocks = map[string]any

func loadMocks(dir string) ([]Mocks, error) {
	var allData []Mocks

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if filepath.Ext(path) != ".json" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		var item Mocks
		if err := json.Unmarshal(data, &item); err != nil {
			return fmt.Errorf("Failed to unmarshal %s: %w", path, err)
		}

		allData = append(allData, item)
		return nil
	})

	if err != nil {
		return nil, err
	}

	fmt.Printf("Total mock items loaded: %d\n", len(allData))

	return allData, nil
}
