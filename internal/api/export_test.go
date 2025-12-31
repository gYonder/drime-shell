package api

// ExtractAPIErrorForTest exposes extractAPIError for testing purposes
func ExtractAPIErrorForTest(body []byte) string {
	return extractAPIError(body)
}
