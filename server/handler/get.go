package handler

func handleGetCall() map[string]any {
	return responseCreator("GET", "get handled")
}
