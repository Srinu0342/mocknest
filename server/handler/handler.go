package handler

import "time"

func Handler(method string) map[string]any {
	switch method {
	case "POST":
		return handlePostCall()
	case "GET":
		return handleGetCall()
	case "PUT":
		value, error := handlePutCall()
		if error != nil {
			return map[string]any{
				"error": error.Error(),
			}
		}
		return value
	}
	return responseCreator("UNKNOWN", "unknown method")
}

func responseCreator(method string, body any) map[string]any {
	response := map[string]any{
		"check":  "ok",
		"method": method,
		"body":   body,
		"time":   time.Now(),
	}

	return response
}
