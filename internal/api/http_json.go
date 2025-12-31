package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (c *HTTPClient) buildURL(path string, query url.Values) string {
	base := strings.TrimRight(c.BaseURL, "/")
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	full := base + path
	if len(query) > 0 {
		full += "?" + query.Encode()
	}
	return full
}

func statusAllowed(status int, okStatuses []int) bool {
	if len(okStatuses) == 0 {
		return status < 400
	}
	for _, s := range okStatuses {
		if status == s {
			return true
		}
	}
	return false
}

func (c *HTTPClient) doWithHeaders(ctx context.Context, method, path string, query url.Values, headers map[string]string, in any, withAuth bool) (int, []byte, error) {
	var bodyReader io.Reader
	if in != nil {
		bodyBytes, err := json.Marshal(in)
		if err != nil {
			return 0, nil, err
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	urlStr := c.buildURL(path, query)
	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if withAuth {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.DoWithRetry(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body, nil
}

func (c *HTTPClient) do(ctx context.Context, method, path string, query url.Values, in any, withAuth bool) (int, []byte, error) {
	return c.doWithHeaders(ctx, method, path, query, nil, in, withAuth)
}

func (c *HTTPClient) doJSONWithHeaders(ctx context.Context, method, path string, query url.Values, headers map[string]string, in any, out any, withAuth bool, okStatuses ...int) error {
	status, body, err := c.doWithHeaders(ctx, method, path, query, headers, in, withAuth)
	if err != nil {
		return err
	}

	if !statusAllowed(status, okStatuses) {
		msg := extractAPIError(body)
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", status)
		}
		return fmt.Errorf("%s %s failed: %s", method, path, msg)
	}

	if out == nil {
		return nil
	}
	if len(body) > 0 && body[0] == '<' {
		return fmt.Errorf("%s %s failed: got HTML response (status %d)", method, path, status)
	}
	if len(body) == 0 {
		return fmt.Errorf("%s %s failed: empty response", method, path)
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%s %s failed to decode JSON: %w", method, path, err)
	}
	return nil
}

func (c *HTTPClient) doJSON(ctx context.Context, method, path string, query url.Values, in any, out any, withAuth bool, okStatuses ...int) error {
	return c.doJSONWithHeaders(ctx, method, path, query, nil, in, out, withAuth, okStatuses...)
}
