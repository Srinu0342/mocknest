package handler

func Handler(method string) string {
	switch method {
	case "POST":
		return handlePostCall()
	case "GET":
		return handleGetCall()
	case "PUT":
		return handlePutCall()
	}
	return "method not supported"
}
