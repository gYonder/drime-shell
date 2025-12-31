package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// GetVaultMetadata fetches the vault metadata (salt, check, iv) for password verification.
// Returns nil, nil if no vault exists for the user.
func (c *HTTPClient) GetVaultMetadata(ctx context.Context) (*VaultMeta, error) {
	url := fmt.Sprintf("%s/vault", c.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 404 means no vault exists
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GetVaultMetadata failed: %s - %s", resp.Status, extractAPIError(body))
	}

	var result struct {
		Vault  VaultMeta `json:"vault"`
		Status string    `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Vault, nil
}

// InitializeVault creates a new vault with the provided encryption parameters.
// salt, check, and iv should be base64-encoded strings.
func (c *HTTPClient) InitializeVault(ctx context.Context, salt, check, iv string) (*VaultMeta, error) {
	reqBody := struct {
		Salt  string `json:"salt"`
		Check string `json:"check"`
		IV    string `json:"iv"`
	}{
		Salt:  salt,
		Check: check,
		IV:    iv,
	}

	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("%s/vault", c.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("InitializeVault failed: %s - %s", resp.Status, extractAPIError(respBody))
	}

	var result struct {
		Vault  VaultMeta `json:"vault"`
		Status string    `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Vault, nil
}

// GetVaultFolders fetches all folders in the vault. Uses MaxPerPage to avoid pagination.
func (c *HTTPClient) GetVaultFolders(ctx context.Context, userID int64) ([]FileEntry, error) {
	url := fmt.Sprintf("%s/users/%d/folders?vault=1&perPage=%d", c.BaseURL, userID, MaxPerPage)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GetVaultFolders failed: %s - %s", resp.Status, extractAPIError(body))
	}

	var result struct {
		Folders []FileEntry `json:"folders"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Folders, nil
}

// ListVaultEntries lists file entries in the vault, optionally filtered by parent folder.
// folderHash is the hash of the parent folder, or empty string for root.
// Uses MaxPerPage to fetch all entries in one request.
func (c *HTTPClient) ListVaultEntries(ctx context.Context, folderHash string) ([]FileEntry, error) {
	reqURL := fmt.Sprintf("%s/vault/file-entries?perPage=%d&backup=0&orderBy=updated_at&orderDir=desc&page=1", c.BaseURL, MaxPerPage)
	if folderHash != "" {
		// URL-encode the hash in case it contains special characters (e.g., base64 padding)
		reqURL += fmt.Sprintf("&folderId=%s&pageId=%s", url.QueryEscape(folderHash), url.QueryEscape(folderHash))
	}

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ListVaultEntries failed: %s - %s", resp.Status, extractAPIError(body))
	}

	var result struct {
		Pagination struct {
			Data []FileEntry `json:"data"`
		} `json:"pagination"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Pagination.Data, nil
}

// MoveVaultEntries moves entries within the vault.
func (c *HTTPClient) MoveVaultEntries(ctx context.Context, entryIDs []int64, destinationID *int64) error {
	reqBody := struct {
		EntryIDs      []int64 `json:"entryIds"`
		DestinationID *int64  `json:"destinationId"`
	}{
		EntryIDs:      entryIDs,
		DestinationID: destinationID,
	}

	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("%s/vault/file-entries/move", c.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.DoWithRetry(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("MoveVaultEntries failed: %s - %s", resp.Status, extractAPIError(respBody))
	}

	return nil
}

// DeleteVaultEntries permanently deletes entries from the vault.
// Vault does not have a trash - all deletes are permanent.
func (c *HTTPClient) DeleteVaultEntries(ctx context.Context, entryIDs []int64) error {
	reqBody := struct {
		EntryIDs      []int64 `json:"entryIds"`
		DeleteForever bool    `json:"deleteForever"`
	}{
		EntryIDs:      entryIDs,
		DeleteForever: true, // Vault always deletes permanently
	}

	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("%s/vault/delete-entries", c.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.DoWithRetry(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DeleteVaultEntries failed: %s - %s", resp.Status, extractAPIError(respBody))
	}

	return nil
}

// CreateVaultFolder creates a new folder in the vault.
// Uses the same /folders endpoint as regular folders, but with vaultId parameter.
func (c *HTTPClient) CreateVaultFolder(ctx context.Context, name string, parentID *int64, vaultID int64) (*FileEntry, error) {
	reqBody := struct {
		Name     string `json:"name"`
		ParentID *int64 `json:"parentId"`
		VaultID  int64  `json:"vaultId"`
	}{
		Name:     name,
		ParentID: parentID,
		VaultID:  vaultID,
	}

	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("%s/folders", c.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("CreateVaultFolder failed: %s - %s", resp.Status, extractAPIError(respBody))
	}

	var res CreateFolderResponse
	if err := json.Unmarshal(respBody, &res); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &res.Folder, nil
}

// DownloadEncrypted downloads an encrypted file from the vault.
// The returned FileEntry includes the IV needed for decryption.
func (c *HTTPClient) DownloadEncrypted(ctx context.Context, hash string, w io.Writer, progress func(int64, int64)) (*FileEntry, error) {
	url := fmt.Sprintf("%s/file-entries/download/%s?encrypted=true", c.BaseURL, hash)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("DownloadEncrypted failed: %s - %s", resp.Status, extractAPIError(body))
	}

	var entry FileEntry
	entry.Size = resp.ContentLength

	// Last-Modified
	if lastMod := resp.Header.Get("Last-Modified"); lastMod != "" {
		if t, err := http.ParseTime(lastMod); err == nil {
			entry.UpdatedAt = t
		}
	}

	// Wrap reader to track progress
	pw := &ProgressReader{
		Reader:     resp.Body,
		Total:      entry.Size,
		Current:    0,
		OnProgress: progress,
	}

	_, err = io.Copy(w, pw)
	if err != nil {
		return nil, err
	}

	return &entry, nil
}
