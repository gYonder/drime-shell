package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mikael.mansson2/drime-shell/internal/api"
	"github.com/stretchr/testify/assert"
)

func TestHTTPClient_GetUserFolders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/users/1/folders", r.URL.Path)
		assert.Equal(t, "0", r.URL.Query().Get("workspaceId"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"folders": [{"id": 100, "name": "Photos", "type": "folder"}]}`))
	}))
	defer server.Close()

	client := api.NewHTTPClient(server.URL, "dummy-token")
	client.BaseRetryDelay = 1 * time.Millisecond

	folders, err := client.GetUserFolders(context.Background(), 1, 0)
	assert.NoError(t, err)
	assert.Len(t, folders, 1)
	assert.Equal(t, "Photos", folders[0].Name)
}
