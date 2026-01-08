package api_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gYonder/drime-shell/internal/api"
	"github.com/stretchr/testify/assert"
)

func TestHTTPClient_Whoami_Retry(t *testing.T) {
	// Simulate unstable API: fails twice with 500, then succeeds
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"user": {"id": 1, "display_name": "Test User", "email": "test@example.com"}}`))
	}))
	defer server.Close()

	client := api.NewHTTPClient(server.URL, "dummy-token")
	// Speed up retries for test
	client.BaseRetryDelay = 1 * time.Millisecond

	user, err := client.Whoami(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, int64(1), user.ID)
	assert.Equal(t, "Test User", user.Name())
	assert.Equal(t, 3, attempts, "Expected 3 attempts (2 failures + 1 success)")
}

func TestHTTPClient_TokenExpired_Returns401(t *testing.T) {
	// Test that 401 returns ErrTokenExpired without retrying
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message": "Token expired"}`))
	}))
	defer server.Close()

	client := api.NewHTTPClient(server.URL, "expired-token")
	client.BaseRetryDelay = 1 * time.Millisecond

	_, err := client.Whoami(context.Background())

	assert.Error(t, err)
	assert.True(t, errors.Is(err, api.ErrTokenExpired), "Should return ErrTokenExpired")
	assert.Equal(t, 1, attempts, "Should not retry on 401")
}

func TestHTTPClient_GetFolderPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/folders/abc123/path", r.URL.Path)
		assert.Equal(t, "0", r.URL.Query().Get("workspaceId"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Return ancestor chain: root -> Photos -> 2024
		w.Write([]byte(`{"path": [
			{"id": 1, "name": "Root", "type": "folder"},
			{"id": 2, "name": "Photos", "type": "folder"},
			{"id": 3, "name": "2024", "type": "folder"}
		]}`))
	}))
	defer server.Close()

	client := api.NewHTTPClient(server.URL, "test-token")
	client.BaseRetryDelay = 1 * time.Millisecond

	path, err := client.GetFolderPath(context.Background(), "abc123", 0)

	assert.NoError(t, err)
	assert.Len(t, path, 3)
	assert.Equal(t, "Root", path[0].Name)
	assert.Equal(t, "Photos", path[1].Name)
	assert.Equal(t, "2024", path[2].Name)
}

func TestMaxPerPage_Constant(t *testing.T) {
	// Verify MaxPerPage is set to the expected value
	assert.Equal(t, int64(9999999999), api.MaxPerPage)
}

func TestHTTPClient_GetUserFolders_UsesMaxPerPage(t *testing.T) {
	var receivedPerPage string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPerPage = r.URL.Query().Get("perPage")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"folders": []}`))
	}))
	defer server.Close()

	client := api.NewHTTPClient(server.URL, "test-token")
	client.BaseRetryDelay = 1 * time.Millisecond

	_, err := client.GetUserFolders(context.Background(), 1, 0)

	assert.NoError(t, err)
	assert.Equal(t, "9999999999", receivedPerPage, "Should use MaxPerPage constant")
}

func TestExtractAPIError_WithMessage(t *testing.T) {
	// Test error extraction with message field
	body := []byte(`{"message": "File not found", "errors": {"path": ["Invalid path"]}}`)
	result := api.ExtractAPIErrorForTest(body)
	assert.Contains(t, result, "File not found")
	assert.Contains(t, result, "path")
}

func TestExtractAPIError_FieldErrorsOnly(t *testing.T) {
	// Test error extraction with only field errors
	body := []byte(`{"errors": {"name": ["Name is required"]}}`)
	result := api.ExtractAPIErrorForTest(body)
	assert.Contains(t, result, "name")
	assert.Contains(t, result, "Name is required")
}

func TestExtractAPIError_InvalidJSON(t *testing.T) {
	// Test fallback to raw body for invalid JSON
	body := []byte(`not json`)
	result := api.ExtractAPIErrorForTest(body)
	assert.Equal(t, "not json", result)
}
