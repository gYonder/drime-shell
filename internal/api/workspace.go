package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// GetWorkspaces fetches all workspaces available to the user
func (c *HTTPClient) GetWorkspaces(ctx context.Context) ([]Workspace, error) {
	var result struct {
		Workspaces []Workspace `json:"workspaces"`
		Status     string      `json:"status"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/me/workspaces", nil, nil, &result, true); err != nil {
		return nil, err
	}

	return result.Workspaces, nil
}

// GetWorkspaceStats fetches file count and size for a workspace
func (c *HTTPClient) GetWorkspaceStats(ctx context.Context, workspaceID int64) (*WorkspaceStats, error) {
	var stats WorkspaceStats
	q := url.Values{}
	q.Set("workspaceId", fmt.Sprintf("%d", workspaceID))
	if err := c.doJSON(ctx, http.MethodGet, "/workspace_files", q, nil, &stats, true); err != nil {
		return nil, err
	}

	return &stats, nil
}

// Login authenticates with email/password and returns user with access token
func (c *HTTPClient) Login(ctx context.Context, email, password, deviceName string) (*User, error) {
	body := map[string]string{
		"email":       email,
		"password":    password,
		"device_name": deviceName,
	}
	status, respBody, err := c.do(ctx, http.MethodPost, "/auth/login", nil, body, false)
	if err != nil {
		return nil, err
	}

	if status == http.StatusUnauthorized {
		return nil, fmt.Errorf("invalid credentials")
	}
	if status == http.StatusUnprocessableEntity {
		var errResp struct {
			Message string            `json:"message"`
			Errors  map[string]string `json:"errors"`
		}
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			return nil, fmt.Errorf("login failed: %s", errResp.Message)
		}
		return nil, fmt.Errorf("login failed: validation error")
	}
	if status >= 400 {
		msg := extractAPIError(respBody)
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", status)
		}
		return nil, fmt.Errorf("login failed: %s", msg)
	}

	var result struct {
		Status string `json:"status"`
		User   User   `json:"user"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	return &result.User, nil
}

// CreateWorkspace creates a new workspace with the given name
func (c *HTTPClient) CreateWorkspace(ctx context.Context, name string) (*Workspace, error) {
	body := map[string]string{
		"name": name,
	}
	status, respBody, err := c.do(ctx, http.MethodPost, "/workspace", nil, body, true)
	if err != nil {
		return nil, err
	}

	if status == http.StatusUnprocessableEntity {
		var errResp struct {
			Message string                 `json:"message"`
			Errors  map[string]interface{} `json:"errors"`
		}
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			// Build error message from validation errors
			if len(errResp.Errors) > 0 {
				var errMsgs []string
				for field, val := range errResp.Errors {
					switch v := val.(type) {
					case string:
						errMsgs = append(errMsgs, fmt.Sprintf("%s: %s", field, v))
					case []interface{}:
						for _, msg := range v {
							errMsgs = append(errMsgs, fmt.Sprintf("%s: %v", field, msg))
						}
					default:
						errMsgs = append(errMsgs, fmt.Sprintf("%s: %v", field, v))
					}
				}
				return nil, fmt.Errorf("validation error: %s", strings.Join(errMsgs, "; "))
			}
			if errResp.Message != "" {
				return nil, fmt.Errorf("failed to create workspace: %s", errResp.Message)
			}
		}
		return nil, fmt.Errorf("failed to create workspace: validation error (%s)", string(respBody))
	}
	if status >= 400 {
		msg := extractAPIError(respBody)
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", status)
		}
		return nil, fmt.Errorf("CreateWorkspace failed: %s", msg)
	}

	var result struct {
		Status    string    `json:"status"`
		Workspace Workspace `json:"workspace"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	return &result.Workspace, nil
}

// UpdateWorkspace updates a workspace's name
func (c *HTTPClient) UpdateWorkspace(ctx context.Context, workspaceID int64, name string) (*Workspace, error) {
	body := map[string]string{
		"name": name,
	}
	path := fmt.Sprintf("/workspace/%d", workspaceID)
	status, respBody, err := c.do(ctx, http.MethodPut, path, nil, body, true)
	if err != nil {
		return nil, err
	}

	if status == http.StatusUnprocessableEntity {
		var errResp struct {
			Message string            `json:"message"`
			Errors  map[string]string `json:"errors"`
		}
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			return nil, fmt.Errorf("failed to update workspace: %s", errResp.Message)
		}
		return nil, fmt.Errorf("failed to update workspace: validation error")
	}
	if status >= 400 {
		msg := extractAPIError(respBody)
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", status)
		}
		return nil, fmt.Errorf("UpdateWorkspace failed: %s", msg)
	}

	var result struct {
		Status    string    `json:"status"`
		Workspace Workspace `json:"workspace"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	return &result.Workspace, nil
}

// DeleteWorkspace deletes a workspace by ID
// Uses Laravel method spoofing: POST with _method=DELETE query parameter
func (c *HTTPClient) DeleteWorkspace(ctx context.Context, workspaceID int64) error {
	q := url.Values{}
	q.Set("_method", "DELETE")
	path := fmt.Sprintf("/workspace/%d", workspaceID)
	status, respBody, err := c.do(ctx, http.MethodPost, path, q, nil, true)
	if err != nil {
		return err
	}

	if status == http.StatusNotFound {
		return fmt.Errorf("workspace not found")
	}
	if status == http.StatusForbidden {
		return fmt.Errorf("you don't have permission to delete this workspace")
	}
	if status != http.StatusOK && status != http.StatusNoContent {
		msg := extractAPIError(respBody)
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", status)
		}
		return fmt.Errorf("DeleteWorkspace failed: %s", msg)
	}

	return nil
}

// GetWorkspace fetches a single workspace with members and invites
func (c *HTTPClient) GetWorkspace(ctx context.Context, workspaceID int64) (*Workspace, error) {
	path := fmt.Sprintf("/workspace/%d", workspaceID)
	status, body, err := c.do(ctx, http.MethodGet, path, nil, nil, true)
	if err != nil {
		return nil, err
	}

	if status == http.StatusNotFound {
		return nil, fmt.Errorf("workspace not found")
	}
	if status >= 400 {
		msg := extractAPIError(body)
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", status)
		}
		return nil, fmt.Errorf("GetWorkspace failed: %s", msg)
	}

	var result struct {
		Workspace Workspace `json:"workspace"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result.Workspace, nil
}

// GetWorkspaceRoles fetches available roles for workspaces
func (c *HTTPClient) GetWorkspaceRoles(ctx context.Context) ([]WorkspaceRole, error) {
	var result struct {
		Roles []WorkspaceRole `json:"roles"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/workspace_roles", nil, nil, &result, true); err != nil {
		return nil, err
	}
	return result.Roles, nil
}

// InviteMember invites users to a workspace
func (c *HTTPClient) InviteMember(ctx context.Context, workspaceID int64, emails []string, roleID int) error {
	body := map[string]interface{}{
		"emails":  emails,
		"role_id": roleID,
	}
	path := fmt.Sprintf("/workspace/%d/invite", workspaceID)
	return c.doJSON(ctx, http.MethodPost, path, nil, body, nil, true)
}

// RemoveMember removes a member from a workspace
func (c *HTTPClient) RemoveMember(ctx context.Context, workspaceID int64, memberID int64) error {
	path := fmt.Sprintf("/workspace/%d/member/%d", workspaceID, memberID)
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil, nil, true)
}

// CancelInvite cancels a pending invitation
func (c *HTTPClient) CancelInvite(ctx context.Context, inviteID int64) error {
	path := fmt.Sprintf("/workspace/invite/%d", inviteID)
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil, nil, true)
}

// ChangeMemberRole changes the role of a member or invite
func (c *HTTPClient) ChangeMemberRole(ctx context.Context, workspaceID int64, memberID interface{}, roleID int, isInvite bool) error {
	typeStr := "member"
	if isInvite {
		typeStr = "invite"
	}
	body := map[string]interface{}{
		"role_id": roleID,
	}
	path := fmt.Sprintf("/workspace/%d/%s/%v/change-role", workspaceID, typeStr, memberID)
	return c.doJSON(ctx, http.MethodPost, path, nil, body, nil, true)
}
