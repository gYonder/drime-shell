package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mikael.mansson2/drime-shell/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// GetWorkspace Tests
// ============================================================================

func TestHTTPClient_GetWorkspace_WithMembers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/workspace/123", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"workspace": {
				"id": 123,
				"name": "Team Project",
				"owner_id": 100,
				"members_count": 3,
				"members": [
					{
						"id": 1,
						"member_id": 100,
						"email": "owner@example.com",
						"display_name": "John Owner",
						"role_name": "Workspace Owner",
						"role_id": 3,
						"is_owner": true,
						"model_type": "member"
					},
					{
						"id": 2,
						"member_id": 101,
						"email": "admin@example.com",
						"display_name": "Jane Admin",
						"role_name": "Workspace Admin",
						"role_id": 2,
						"is_owner": false,
						"model_type": "member"
					}
				],
				"invites": [
					{
						"id": 500,
						"email": "pending@example.com",
						"role_id": 1,
						"role_name": "Workspace Member",
						"workspace_id": 123,
						"model_type": "invite"
					}
				]
			},
			"status": "success"
		}`))
	}))
	defer server.Close()

	client := api.NewHTTPClient(server.URL, "test-token")
	client.BaseRetryDelay = 1 * time.Millisecond

	ws, err := client.GetWorkspace(context.Background(), 123)

	require.NoError(t, err)
	assert.Equal(t, int64(123), ws.ID)
	assert.Equal(t, "Team Project", ws.Name)
	assert.Len(t, ws.Members, 2)
	assert.Len(t, ws.Invites, 1)

	// Verify owner
	assert.Equal(t, "owner@example.com", ws.Members[0].Email)
	assert.True(t, ws.Members[0].IsOwner)
	assert.Equal(t, "Workspace Owner", ws.Members[0].RoleName)

	// Verify admin
	assert.Equal(t, "admin@example.com", ws.Members[1].Email)
	assert.False(t, ws.Members[1].IsOwner)

	// Verify pending invite
	assert.Equal(t, "pending@example.com", ws.Invites[0].Email)
	assert.Equal(t, "invite", ws.Invites[0].ModelType)
}

func TestHTTPClient_GetWorkspace_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := api.NewHTTPClient(server.URL, "test-token")
	client.BaseRetryDelay = 1 * time.Millisecond

	_, err := client.GetWorkspace(context.Background(), 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ============================================================================
// GetWorkspaceStats Tests
// ============================================================================

func TestHTTPClient_GetWorkspaceStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/workspace_files", r.URL.Path)
		assert.Equal(t, "123", r.URL.Query().Get("workspaceId"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"files": 42,
			"size": 1073741824,
			"status": "success"
		}`))
	}))
	defer server.Close()

	client := api.NewHTTPClient(server.URL, "test-token")
	client.BaseRetryDelay = 1 * time.Millisecond

	stats, err := client.GetWorkspaceStats(context.Background(), 123)

	require.NoError(t, err)
	assert.Equal(t, 42, stats.Files)
	assert.Equal(t, int64(1073741824), stats.Size) // 1 GB
}

// ============================================================================
// DeleteWorkspace Tests (with Laravel method spoofing)
// ============================================================================

func TestHTTPClient_DeleteWorkspace_MethodSpoofing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Laravel uses POST with _method=DELETE query parameter
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "DELETE", r.URL.Query().Get("_method"))
		assert.Equal(t, "/workspace/123", r.URL.Path)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "success"}`))
	}))
	defer server.Close()

	client := api.NewHTTPClient(server.URL, "test-token")
	client.BaseRetryDelay = 1 * time.Millisecond

	err := client.DeleteWorkspace(context.Background(), 123)

	assert.NoError(t, err)
}

func TestHTTPClient_DeleteWorkspace_Forbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	client := api.NewHTTPClient(server.URL, "test-token")
	client.BaseRetryDelay = 1 * time.Millisecond

	err := client.DeleteWorkspace(context.Background(), 123)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permission")
}
