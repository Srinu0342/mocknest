package generator

import "log"

func GenerateMappings() {
	log.Println("Generating mappings...")

	data, error := loadMocks("mocks")
	if error != nil {
		log.Fatal("Failed to load mocks:", error)
	}
	log.Printf("Add data returned %s\n", data)

	log.Println("Mappings generation completed.")
}
