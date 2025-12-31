package api_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mikael.mansson2/drime-shell/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPClient_Upload_Simple_S3PresignFlow(t *testing.T) {
	// This test verifies the 3-step simple upload flow:
	// 1. POST /s3/simple/presign -> get presigned URL
	// 2. PUT to S3 URL -> upload file content
	// 3. POST /s3/entries -> create file entry

	var presignCalled, s3Called, entryCalled bool
	var s3ReceivedContent []byte

	// Mock S3 server (simulates the presigned URL endpoint)
	s3Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s3Called = true
		assert.Equal(t, "PUT", r.Method)
		s3ReceivedContent, _ = io.ReadAll(r.Body)
		w.Header().Set("ETag", `"abc123"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer s3Server.Close()

	// Mock API server
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/s3/simple/presign":
			presignCalled = true
			assert.Equal(t, "POST", r.Method)
			// Return the S3 mock server URL
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"url": "` + s3Server.URL + `/upload", "acl": "private", "key": "uploads/test-file.txt"}`))

		case "/s3/entries":
			entryCalled = true
			assert.Equal(t, "POST", r.Method)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "success", "fileEntry": {"id": 12345, "name": "test.txt", "type": "file", "hash": "MTIzNDV8"}}`))

		default:
			t.Errorf("Unexpected request to %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer apiServer.Close()

	client := api.NewHTTPClient(apiServer.URL, "test-token")
	client.BaseRetryDelay = 1 * time.Millisecond

	content := []byte("Hello, World!")
	reader := bytes.NewReader(content)

	entry, err := client.Upload(context.Background(), reader, "test.txt", nil, int64(len(content)), 0)

	require.NoError(t, err)
	assert.True(t, presignCalled, "Presign endpoint should be called")
	assert.True(t, s3Called, "S3 upload should be called")
	assert.True(t, entryCalled, "Entries endpoint should be called")
	assert.Equal(t, content, s3ReceivedContent, "S3 should receive the file content")
	assert.Equal(t, int64(12345), entry.ID)
	assert.Equal(t, "test.txt", entry.Name)
}

func TestHTTPClient_Upload_S3RetryOnFailure(t *testing.T) {
	// Test that S3 upload retries on failure
	s3Attempts := 0

	s3Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s3Attempts++
		if s3Attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("ETag", `"abc123"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer s3Server.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/s3/simple/presign":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"url": "` + s3Server.URL + `/upload", "acl": "private", "key": "uploads/test.txt"}`))
		case "/s3/entries":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "success", "fileEntry": {"id": 1, "name": "test.txt", "type": "file"}}`))
		}
	}))
	defer apiServer.Close()

	client := api.NewHTTPClient(apiServer.URL, "test-token")
	client.BaseRetryDelay = 1 * time.Millisecond

	content := []byte("test content")
	reader := bytes.NewReader(content)

	entry, err := client.Upload(context.Background(), reader, "test.txt", nil, int64(len(content)), 0)

	require.NoError(t, err)
	assert.Equal(t, 3, s3Attempts, "S3 should retry and succeed on 3rd attempt")
	assert.NotNil(t, entry)
}

func TestHTTPClient_Upload_PresignFailure(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/s3/simple/presign" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"message": "Invalid file type"}`))
			return
		}
	}))
	defer apiServer.Close()

	client := api.NewHTTPClient(apiServer.URL, "test-token")
	client.BaseRetryDelay = 1 * time.Millisecond
	client.MaxRetries = 0 // No retries for this test

	content := []byte("test")
	reader := bytes.NewReader(content)

	_, err := client.Upload(context.Background(), reader, "test.txt", nil, int64(len(content)), 0)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid file type")
}
