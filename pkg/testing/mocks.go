package testing

import (
	"bytes"
	"io"
	"net/http"
)

// MockRoundTripper intercepts outbound HTTP requests and returns pre-defined mock responses.
// This is used to offline-mock REST providers and Data Sources intercepting all live network calls.
type MockRoundTripper struct {
	ExpectedResponses map[string]HTTPResponseMock
}

// RoundTrip simulates HTTP calls by providing static YAML-driven mock data
func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	urlStr := req.URL.String()

	// Check if we have a mocked response for this URL
	if mockResp, exists := m.ExpectedResponses[urlStr]; exists {
		return &http.Response{
			StatusCode: mockResp.StatusCode,
			Body:       io.NopCloser(bytes.NewBufferString(mockResp.Body)),
			Header:     make(http.Header),
		}, nil
	}

	// Default 404 if not found in mock data
	return &http.Response{
		StatusCode: 404,
		Body:       io.NopCloser(bytes.NewBufferString(`{"error": "mock data not found"}`)),
	}, nil
}
