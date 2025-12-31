package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// UploadSession tracks the state of a directory upload for resumability
type UploadSession struct {
	CompletedFiles map[string]int64  `json:"completed_files"` // relativePath -> size
	FailedFiles    map[string]string `json:"failed_files"`    // relativePath -> error
	CreatedFolders map[string]int64  `json:"created_folders"` // relativePath -> folderID
	ID             string            `json:"id"`
	LocalPath      string            `json:"local_path"`
	RemotePath     string            `json:"remote_path"`
	BaseFolderPath string            `json:"base_folder_path"`
	filePath       string            `json:"-"`
	StartedAt      time.Time         `json:"started_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	mu             sync.Mutex        `json:"-"`
	BaseFolderID   int64             `json:"base_folder_id"`
	TotalFiles     int               `json:"total_files"`
}

// SessionsDir returns the directory where upload sessions are stored
func SessionsDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(homeDir, ".drime-shell", "upload-sessions")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

// GenerateSessionID creates a unique session ID from local and remote paths
func GenerateSessionID(localPath, remotePath string) string {
	absLocal, _ := filepath.Abs(localPath)
	data := fmt.Sprintf("%s:%s", absLocal, remotePath)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8]) // First 8 bytes = 16 hex chars
}

// NewUploadSession creates a new upload session
func NewUploadSession(localPath, remotePath string, totalFiles int) (*UploadSession, error) {
	sessionsDir, err := SessionsDir()
	if err != nil {
		return nil, err
	}

	absLocal, _ := filepath.Abs(localPath)
	id := GenerateSessionID(absLocal, remotePath)

	session := &UploadSession{
		ID:             id,
		LocalPath:      absLocal,
		RemotePath:     remotePath,
		StartedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		TotalFiles:     totalFiles,
		CompletedFiles: make(map[string]int64),
		FailedFiles:    make(map[string]string),
		CreatedFolders: make(map[string]int64),
		filePath:       filepath.Join(sessionsDir, id+".json"),
	}

	return session, session.Save()
}

// LoadUploadSession loads an existing session by ID
func LoadUploadSession(id string) (*UploadSession, error) {
	sessionsDir, err := SessionsDir()
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(sessionsDir, id+".json")
	return LoadUploadSessionFromFile(filePath)
}

// LoadUploadSessionFromFile loads a session from a specific file
func LoadUploadSessionFromFile(filePath string) (*UploadSession, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var session UploadSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	session.filePath = filePath
	if session.CompletedFiles == nil {
		session.CompletedFiles = make(map[string]int64)
	}
	if session.FailedFiles == nil {
		session.FailedFiles = make(map[string]string)
	}
	if session.CreatedFolders == nil {
		session.CreatedFolders = make(map[string]int64)
	}

	return &session, nil
}

// FindExistingSession looks for an existing session for the given paths
func FindExistingSession(localPath, remotePath string) (*UploadSession, error) {
	absLocal, _ := filepath.Abs(localPath)
	id := GenerateSessionID(absLocal, remotePath)
	return LoadUploadSession(id)
}

// Save persists the session state to disk
func (s *UploadSession) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0600)
}

// MarkFileCompleted marks a file as successfully uploaded
func (s *UploadSession) MarkFileCompleted(relativePath string, size int64) {
	s.mu.Lock()
	s.CompletedFiles[relativePath] = size
	delete(s.FailedFiles, relativePath) // Remove from failed if it was there
	s.mu.Unlock()
}

// MarkFileFailed marks a file as failed
func (s *UploadSession) MarkFileFailed(relativePath string, errMsg string) {
	s.mu.Lock()
	s.FailedFiles[relativePath] = errMsg
	s.mu.Unlock()
}

// MarkFolderCreated records a created folder's ID
func (s *UploadSession) MarkFolderCreated(relativePath string, folderID int64) {
	s.mu.Lock()
	s.CreatedFolders[relativePath] = folderID
	s.mu.Unlock()
}

// SetBaseFolderInfo sets the base folder information
func (s *UploadSession) SetBaseFolderInfo(folderID int64, folderPath string) {
	s.mu.Lock()
	s.BaseFolderID = folderID
	s.BaseFolderPath = folderPath
	s.mu.Unlock()
}

// IsFileCompleted checks if a file was already uploaded (by path and size)
func (s *UploadSession) IsFileCompleted(relativePath string, size int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	completedSize, ok := s.CompletedFiles[relativePath]
	return ok && completedSize == size
}

// IsFolderCreated checks if a folder was already created
func (s *UploadSession) IsFolderCreated(relativePath string) (int64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, ok := s.CreatedFolders[relativePath]
	return id, ok
}

// Progress returns the current progress
func (s *UploadSession) Progress() (completed, failed, total int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.CompletedFiles), len(s.FailedFiles), s.TotalFiles
}

// IsComplete returns true if all files have been processed
func (s *UploadSession) IsComplete() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.CompletedFiles)+len(s.FailedFiles) >= s.TotalFiles
}

// Delete removes the session file
func (s *UploadSession) Delete() error {
	return os.Remove(s.filePath)
}

// ListSessions returns all pending upload sessions
func ListSessions() ([]*UploadSession, error) {
	sessionsDir, err := SessionsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var sessions []*UploadSession
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		session, err := LoadUploadSessionFromFile(filepath.Join(sessionsDir, entry.Name()))
		if err != nil {
			continue // Skip corrupted sessions
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// CleanupOldSessions removes sessions older than the given duration
func CleanupOldSessions(maxAge time.Duration) error {
	sessions, err := ListSessions()
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-maxAge)
	for _, session := range sessions {
		if session.UpdatedAt.Before(cutoff) {
			_ = session.Delete()
		}
	}

	return nil
}
