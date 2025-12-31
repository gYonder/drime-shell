package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrTokenExpired is returned when the API returns a 401 Unauthorized response,
// indicating the token has expired or is invalid.
var ErrTokenExpired = errors.New("authentication token expired or invalid")

// MaxPerPage is the maximum number of items to request per page to avoid pagination.
// Use this constant for all paginated API calls.
const MaxPerPage int64 = 9999999999

type GetUserFoldersResponse struct {
	Data []FileEntry `json:"data"`
}

type HTTPClient struct {
	Client         *http.Client
	BaseURL        string
	Token          string
	BaseRetryDelay time.Duration
	MaxRetries     int
}

func NewHTTPClient(baseURL, token string) *HTTPClient {
	return &HTTPClient{
		BaseURL:        baseURL,
		Token:          token,
		Client:         &http.Client{Timeout: 40 * time.Second},
		BaseRetryDelay: 500 * time.Millisecond,
		MaxRetries:     10,
	}
}

// DoWithRetry executes a request with exponential backoff and jitter
// NOTE: For POST/PUT requests with bodies, the body must be a *bytes.Reader or *bytes.Buffer
// so it can be reset for retries. Otherwise, retries after body consumption will fail.
func (c *HTTPClient) DoWithRetry(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	// Save the body for potential retries
	var bodyBytes []byte
	if req.Body != nil && req.Method != "GET" && req.Method != "HEAD" {
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		req.Body.Close()
	}

	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		// Reset body for each attempt
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			req.ContentLength = int64(len(bodyBytes))
		}

		resp, err = c.Client.Do(req)

		// Check for success or non-retriable errors
		if err == nil {
			// 401 Unauthorized - token expired, don't retry
			if resp.StatusCode == http.StatusUnauthorized {
				resp.Body.Close()
				return nil, ErrTokenExpired
			}
			if resp.StatusCode < 500 {
				return resp, nil
			}
			// 5xx errors are retriable
			resp.Body.Close()
		}

		// Check for SSL/TLS errors - provide helpful hints
		if err != nil && isSSLError(err) {
			return nil, fmt.Errorf("%w\n\nSSL/TLS error hint: %s", err, getSSLErrorHint(err))
		}

		// Calculate delay with exponential backoff and jitter
		if attempt < c.MaxRetries {
			backoff := float64(c.BaseRetryDelay) * math.Pow(2, float64(attempt))
			jitter := rand.Float64() * 0.25 * backoff
			sleepDuration := time.Duration(backoff + jitter)

			// Cap at 30 seconds
			if sleepDuration > 30*time.Second {
				sleepDuration = 30 * time.Second
			}

			select {
			case <-time.After(sleepDuration):
				continue
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("request failed after %d retries: %w", c.MaxRetries, err)
	}
	return nil, fmt.Errorf("server returned %d after %d retries", resp.StatusCode, c.MaxRetries)
}

// isSSLError checks if an error is SSL/TLS related
func isSSLError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToUpper(err.Error())
	sslPatterns := []string{"SSL", "TLS", "CERTIFICATE", "UNEXPECTED_EOF", "CONNECTION RESET", "HANDSHAKE"}
	for _, pattern := range sslPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}
	return false
}

// getSSLErrorHint returns a helpful hint for SSL errors
func getSSLErrorHint(err error) string {
	errStr := strings.ToUpper(err.Error())
	if strings.Contains(errStr, "UNEXPECTED_EOF") || strings.Contains(errStr, "CONNECTION RESET") {
		return "The server closed the connection unexpectedly. Try: reducing parallel workers, checking your network connection, or temporarily disabling VPN."
	}
	if strings.Contains(errStr, "CERTIFICATE") {
		return "Certificate verification failed. Check if your system certificates are up to date."
	}
	return "An SSL/TLS error occurred. Check your network connection and try again."
}

// extractAPIError extracts user-friendly error messages from API responses
func extractAPIError(body []byte) string {
	var errResp struct {
		Message string              `json:"message"`
		Errors  map[string][]string `json:"errors"`
	}
	if err := json.Unmarshal(body, &errResp); err != nil {
		return string(body)
	}
	// Return message if present
	if errResp.Message != "" {
		// If we also have field errors, append the first one
		if len(errResp.Errors) > 0 {
			for field, msgs := range errResp.Errors {
				if len(msgs) > 0 {
					return fmt.Sprintf("%s: %s - %s", errResp.Message, field, msgs[0])
				}
			}
		}
		return errResp.Message
	}
	// Return first field error if no message
	for field, msgs := range errResp.Errors {
		if len(msgs) > 0 {
			return fmt.Sprintf("%s: %s", field, msgs[0])
		}
	}
	return string(body)
}

func (c *HTTPClient) GetSpaceUsage(ctx context.Context, workspaceID int64) (*SpaceUsage, error) {
	q := url.Values{}
	q.Set("workspaceId", fmt.Sprintf("%d", workspaceID))
	status, body, err := c.do(ctx, http.MethodGet, "/user/space-usage", q, nil, true)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("GetSpaceUsage failed: %d %s", status, http.StatusText(status))
	}

	var res SpaceUsage
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *HTTPClient) Whoami(ctx context.Context) (*User, error) {
	status, body, err := c.do(ctx, http.MethodGet, "/cli/loggedUser", nil, nil, true)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("API error: %d %s", status, http.StatusText(status))
	}

	var result struct {
		User User `json:"user"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &result.User, nil
}

// GetUserFolders fetches all folders for a user. Uses MaxPerPage to avoid pagination.
func (c *HTTPClient) GetUserFolders(ctx context.Context, userID int64, workspaceID int64) ([]FileEntry, error) {
	q := url.Values{}
	q.Set("workspaceId", fmt.Sprintf("%d", workspaceID))
	q.Set("perPage", fmt.Sprintf("%d", MaxPerPage))
	path := fmt.Sprintf("/users/%d/folders", userID)
	status, body, err := c.do(ctx, http.MethodGet, path, q, nil, true)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("API error: %d %s - %s", status, http.StatusText(status), extractAPIError(body))
	}

	var result struct {
		Folders []FileEntry `json:"folders"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result.Folders, nil
}

// GetFolderPath returns the ancestor chain for a folder (root to folder).
// This is useful for resolving the full path of an entry when the cache doesn't have it.
func (c *HTTPClient) GetFolderPath(ctx context.Context, folderHash string, workspaceID int64) ([]FileEntry, error) {
	q := url.Values{}
	q.Set("workspaceId", fmt.Sprintf("%d", workspaceID))
	path := fmt.Sprintf("/folders/%s/path", folderHash)
	status, body, err := c.do(ctx, http.MethodGet, path, q, nil, true)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("GetFolderPath failed: %d %s - %s", status, http.StatusText(status), extractAPIError(body))
	}

	var result struct {
		Path []FileEntry `json:"path"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result.Path, nil
}

func (c *HTTPClient) ListByParentID(ctx context.Context, parentID *int64) ([]FileEntry, error) {
	return c.ListByParentIDWithOptions(ctx, parentID, ListOptions(0))
}
