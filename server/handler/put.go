package handler

import (
	"log"
)

func handlePutCall() (map[string]any, error) {
	defer func() map[string]any {
		if r := recover(); r != nil {
			log.Println("Recovered from panic in handlePutCall:", r)
			return responseCreator("PUT", "put handler inside recover")
		}
		return nil
	}()

	return responseCreator("PUT", "put handled"), nil
}
