package api

import (
	"context"
	"io"
)

// ListEntriesOptions controls filtering for file entry listings
type ListEntriesOptions struct {
	WorkspaceID int64
	DeletedOnly bool   // Only return trashed items
	StarredOnly bool   // Only return starred items
	TrackedOnly bool   // Only return tracked items
	Query       string // Search query
	// Paging & ordering. If PerPage is 0, the client chooses a sensible default.
	// If Page is 0, the client will auto-page and return all results.
	PerPage int64
	Page    int
	// Filters is a base64 encoded JSON string of filters
	Filters string
	// Backup matches the API query param. Use 0 for non-backup files.
	// If nil, the param is omitted and the server default applies.
	Backup *int
	// OrderBy is one of: updated_at, created_at, name, file_size.
	OrderBy string
	// OrderDir is one of: asc, desc.
	OrderDir string
}

// DrimeClient defines the interface for interacting with Drime Cloud
type DrimeClient interface {
	// Authentication
	Whoami(ctx context.Context) (*User, error)
	Login(ctx context.Context, email, password, deviceName string) (*User, error)

	// Workspaces
	GetWorkspaces(ctx context.Context) ([]Workspace, error)
	GetWorkspace(ctx context.Context, workspaceID int64) (*Workspace, error)
	GetWorkspaceStats(ctx context.Context, workspaceID int64) (*WorkspaceStats, error)
	CreateWorkspace(ctx context.Context, name string) (*Workspace, error)
	UpdateWorkspace(ctx context.Context, workspaceID int64, name string) (*Workspace, error)
	DeleteWorkspace(ctx context.Context, workspaceID int64) error

	// Workspace Members
	GetWorkspaceRoles(ctx context.Context) ([]WorkspaceRole, error)
	InviteMember(ctx context.Context, workspaceID int64, emails []string, roleID int) error
	RemoveMember(ctx context.Context, workspaceID int64, memberID int64) error
	CancelInvite(ctx context.Context, inviteID int64) error
	ChangeMemberRole(ctx context.Context, workspaceID int64, memberID interface{}, roleID int, isInvite bool) error

	// Navigation & Listing
	GetUserFolders(ctx context.Context, userID int64, workspaceID int64) ([]FileEntry, error)
	GetFolderPath(ctx context.Context, folderHash string, workspaceID int64) ([]FileEntry, error)
	ListByParentID(ctx context.Context, parentID *int64) ([]FileEntry, error)
	ListByParentIDWithOptions(ctx context.Context, parentID *int64, opts *ListEntriesOptions) ([]FileEntry, error)

	// Starring
	StarEntries(ctx context.Context, entryIDs []int64, workspaceID int64) error
	UnstarEntries(ctx context.Context, entryIDs []int64, workspaceID int64) error

	// Trash
	RestoreEntries(ctx context.Context, entryIDs []int64, workspaceID int64) error
	EmptyTrash(ctx context.Context, workspaceID int64) error

	// File Operations (Remote)

	// Transfers
	Upload(ctx context.Context, reader io.Reader, name string, parentID *int64, size int64, workspaceID int64) (*FileEntry, error)
	Download(ctx context.Context, hash string, w io.Writer, progress func(int64, int64)) (*FileEntry, error)
	DownloadWithOptions(ctx context.Context, hash string, w io.Writer, progress func(int64, int64), opts *DownloadOptions) (*FileEntry, error)

	// Management
	CreateFolder(ctx context.Context, name string, parentID *int64, workspaceID int64) (*FileEntry, error)
	DeleteEntries(ctx context.Context, entryIDs []int64, workspaceID int64) error
	DeleteEntriesForever(ctx context.Context, entryIDs []int64, workspaceID int64) error
	MoveEntries(ctx context.Context, entryIDs []int64, destinationParentID *int64, workspaceID int64, destinationWorkspaceID *int64) error
	CopyEntries(ctx context.Context, entryIDs []int64, destinationParentID *int64, workspaceID int64, destinationWorkspaceID *int64) ([]FileEntry, error)
	RenameEntry(ctx context.Context, entryID int64, newName string, workspaceID int64) (*FileEntry, error)
	GetSpaceUsage(ctx context.Context, workspaceID int64) (*SpaceUsage, error)
	ExtractEntry(ctx context.Context, entryID int64, parentID *int64, workspaceID int64) error
	GetEntry(ctx context.Context, entryID int64, workspaceID int64) (*FileEntry, error)
	Search(ctx context.Context, query string) ([]FileEntry, error)
	SearchWithOptions(ctx context.Context, query string, opts *ListEntriesOptions) ([]FileEntry, error)
	ValidateEntries(ctx context.Context, req ValidateRequest) (*ValidateResponse, error)
	GetAvailableName(ctx context.Context, req GetAvailableNameRequest) (*GetAvailableNameResponse, error)

	// Sharing
	CreateShareableLink(ctx context.Context, entryID int64, req ShareableLinkRequest) (*ShareableLink, error)
	CreateFileRequest(ctx context.Context, entryID int64, title, description string) (*ShareableLink, error)
	UpdateShareableLink(ctx context.Context, entryID int64, req ShareableLinkRequest) (*ShareableLink, error)
	DeleteShareableLink(ctx context.Context, entryID int64) error
	GetShareableLink(ctx context.Context, entryID int64) (*ShareableLink, error)
	ShareEntry(ctx context.Context, entryID int64, emails []string, permissions []string) error

	// File Requests
	ListFileRequests(ctx context.Context) ([]FileRequest, error)
	DeleteFileRequest(ctx context.Context, requestID int64) error

	// Tracking
	GetTrackedFiles(ctx context.Context) ([]TrackedFile, error)
	GetTrackingStats(ctx context.Context, entryID int64) (*TrackingStatsResponse, error)
	SetTracking(ctx context.Context, entryID int64, tracked bool) error

	// Vault operations
	GetVaultMetadata(ctx context.Context) (*VaultMeta, error)
	InitializeVault(ctx context.Context, salt, check, iv string) (*VaultMeta, error)
	GetVaultFolders(ctx context.Context, userID int64) ([]FileEntry, error)
	ListVaultEntries(ctx context.Context, folderHash string) ([]FileEntry, error)
	MoveVaultEntries(ctx context.Context, entryIDs []int64, destinationID *int64) error
	DeleteVaultEntries(ctx context.Context, entryIDs []int64) error
	CreateVaultFolder(ctx context.Context, name string, parentID *int64, vaultID int64) (*FileEntry, error)
	DownloadEncrypted(ctx context.Context, hash string, w io.Writer, progress func(int64, int64)) (*FileEntry, error)
	UploadToVault(ctx context.Context, encryptedContent []byte, name string, parentID *int64, vaultID int64, ivBase64 string) (*FileEntry, error)
} // End of interface

// MockDrimeClient is a mock implementation for testing
type MockDrimeClient struct {
	WhoamiFunc            func(ctx context.Context) (*User, error)
	LoginFunc             func(ctx context.Context, email, password, deviceName string) (*User, error)
	GetWorkspacesFunc     func(ctx context.Context) ([]Workspace, error)
	GetWorkspaceFunc      func(ctx context.Context, workspaceID int64) (*Workspace, error)
	GetWorkspaceStatsFunc func(ctx context.Context, workspaceID int64) (*WorkspaceStats, error)
	CreateWorkspaceFunc   func(ctx context.Context, name string) (*Workspace, error)
	UpdateWorkspaceFunc   func(ctx context.Context, workspaceID int64, name string) (*Workspace, error)
	DeleteWorkspaceFunc   func(ctx context.Context, workspaceID int64) error
	// Workspace Members
	GetWorkspaceRolesFunc func(ctx context.Context) ([]WorkspaceRole, error)
	InviteMemberFunc      func(ctx context.Context, workspaceID int64, emails []string, roleID int) error
	RemoveMemberFunc      func(ctx context.Context, workspaceID int64, memberID int64) error
	CancelInviteFunc      func(ctx context.Context, inviteID int64) error
	ChangeMemberRoleFunc  func(ctx context.Context, workspaceID int64, memberID interface{}, roleID int, isInvite bool) error

	GetUserFoldersFunc            func(ctx context.Context, userID int64, workspaceID int64) ([]FileEntry, error)
	GetFolderPathFunc             func(ctx context.Context, folderHash string, workspaceID int64) ([]FileEntry, error)
	ListByParentIDFunc            func(ctx context.Context, parentID *int64) ([]FileEntry, error)
	ListByParentIDWithOptionsFunc func(ctx context.Context, parentID *int64, opts *ListEntriesOptions) ([]FileEntry, error)
	StarEntriesFunc               func(ctx context.Context, entryIDs []int64, workspaceID int64) error
	UnstarEntriesFunc             func(ctx context.Context, entryIDs []int64, workspaceID int64) error
	RestoreEntriesFunc            func(ctx context.Context, entryIDs []int64, workspaceID int64) error
	EmptyTrashFunc                func(ctx context.Context, workspaceID int64) error
	UploadFunc                    func(ctx context.Context, reader io.Reader, name string, parentID *int64, size int64, workspaceID int64) (*FileEntry, error)
	DownloadFunc                  func(ctx context.Context, hash string, w io.Writer, progress func(int64, int64)) (*FileEntry, error)
	DownloadWithOptionsFunc       func(ctx context.Context, hash string, w io.Writer, progress func(int64, int64), opts *DownloadOptions) (*FileEntry, error)
	CreateFolderFunc              func(ctx context.Context, name string, parentID *int64, workspaceID int64) (*FileEntry, error)
	DeleteEntriesFunc             func(ctx context.Context, entryIDs []int64, workspaceID int64) error
	DeleteEntriesForeverFunc      func(ctx context.Context, entryIDs []int64, workspaceID int64) error
	MoveEntriesFunc               func(ctx context.Context, entryIDs []int64, destinationParentID *int64, workspaceID int64, destinationWorkspaceID *int64) error
	CopyEntriesFunc               func(ctx context.Context, entryIDs []int64, destinationParentID *int64, workspaceID int64, destinationWorkspaceID *int64) ([]FileEntry, error)
	RenameEntryFunc               func(ctx context.Context, entryID int64, newName string, workspaceID int64) (*FileEntry, error)
	GetSpaceUsageFunc             func(ctx context.Context, workspaceID int64) (*SpaceUsage, error)
	ExtractEntryFunc              func(ctx context.Context, entryID int64, parentID *int64, workspaceID int64) error
	GetEntryFunc                  func(ctx context.Context, entryID int64, workspaceID int64) (*FileEntry, error)
	SearchFunc                    func(ctx context.Context, query string) ([]FileEntry, error)
	SearchWithOptionsFunc         func(ctx context.Context, query string, opts *ListEntriesOptions) ([]FileEntry, error)
	ValidateEntriesFunc           func(ctx context.Context, req ValidateRequest) (*ValidateResponse, error)
	GetAvailableNameFunc          func(ctx context.Context, req GetAvailableNameRequest) (*GetAvailableNameResponse, error)
	// Vault mock functions
	GetVaultMetadataFunc   func(ctx context.Context) (*VaultMeta, error)
	InitializeVaultFunc    func(ctx context.Context, salt, check, iv string) (*VaultMeta, error)
	GetVaultFoldersFunc    func(ctx context.Context, userID int64) ([]FileEntry, error)
	ListVaultEntriesFunc   func(ctx context.Context, folderHash string) ([]FileEntry, error)
	MoveVaultEntriesFunc   func(ctx context.Context, entryIDs []int64, destinationID *int64) error
	DeleteVaultEntriesFunc func(ctx context.Context, entryIDs []int64) error
	CreateVaultFolderFunc  func(ctx context.Context, name string, parentID *int64, vaultID int64) (*FileEntry, error)
	DownloadEncryptedFunc  func(ctx context.Context, hash string, w io.Writer, progress func(int64, int64)) (*FileEntry, error)
	UploadToVaultFunc      func(ctx context.Context, encryptedContent []byte, name string, parentID *int64, vaultID int64, ivBase64 string) (*FileEntry, error)
	// Sharing mock functions
	CreateShareableLinkFunc func(ctx context.Context, entryID int64, req ShareableLinkRequest) (*ShareableLink, error)
	CreateFileRequestFunc   func(ctx context.Context, entryID int64, title, description string) (*ShareableLink, error)
	UpdateShareableLinkFunc func(ctx context.Context, entryID int64, req ShareableLinkRequest) (*ShareableLink, error)
	DeleteShareableLinkFunc func(ctx context.Context, entryID int64) error
	GetShareableLinkFunc    func(ctx context.Context, entryID int64) (*ShareableLink, error)
	ShareEntryFunc          func(ctx context.Context, entryID int64, emails []string, permissions []string) error
	// File Requests mock functions
	ListFileRequestsFunc  func(ctx context.Context) ([]FileRequest, error)
	DeleteFileRequestFunc func(ctx context.Context, requestID int64) error
	// Tracking mock functions
	GetTrackedFilesFunc  func(ctx context.Context) ([]TrackedFile, error)
	GetTrackingStatsFunc func(ctx context.Context, entryID int64) (*TrackingStatsResponse, error)
	SetTrackingFunc      func(ctx context.Context, entryID int64, tracked bool) error
}

func (m *MockDrimeClient) Whoami(ctx context.Context) (*User, error) {
	return m.WhoamiFunc(ctx)
}

func (m *MockDrimeClient) Login(ctx context.Context, email, password, deviceName string) (*User, error) {
	if m.LoginFunc != nil {
		return m.LoginFunc(ctx, email, password, deviceName)
	}
	return nil, nil
}

func (m *MockDrimeClient) GetWorkspaces(ctx context.Context) ([]Workspace, error) {
	if m.GetWorkspacesFunc != nil {
		return m.GetWorkspacesFunc(ctx)
	}
	return nil, nil
}

func (m *MockDrimeClient) GetWorkspace(ctx context.Context, workspaceID int64) (*Workspace, error) {
	if m.GetWorkspaceFunc != nil {
		return m.GetWorkspaceFunc(ctx, workspaceID)
	}
	return nil, nil
}

func (m *MockDrimeClient) GetWorkspaceRoles(ctx context.Context) ([]WorkspaceRole, error) {
	if m.GetWorkspaceRolesFunc != nil {
		return m.GetWorkspaceRolesFunc(ctx)
	}
	return nil, nil
}

func (m *MockDrimeClient) InviteMember(ctx context.Context, workspaceID int64, emails []string, roleID int) error {
	if m.InviteMemberFunc != nil {
		return m.InviteMemberFunc(ctx, workspaceID, emails, roleID)
	}
	return nil
}

func (m *MockDrimeClient) RemoveMember(ctx context.Context, workspaceID int64, memberID int64) error {
	if m.RemoveMemberFunc != nil {
		return m.RemoveMemberFunc(ctx, workspaceID, memberID)
	}
	return nil
}

func (m *MockDrimeClient) CancelInvite(ctx context.Context, inviteID int64) error {
	if m.CancelInviteFunc != nil {
		return m.CancelInviteFunc(ctx, inviteID)
	}
	return nil
}

func (m *MockDrimeClient) ChangeMemberRole(ctx context.Context, workspaceID int64, memberID interface{}, roleID int, isInvite bool) error {
	if m.ChangeMemberRoleFunc != nil {
		return m.ChangeMemberRoleFunc(ctx, workspaceID, memberID, roleID, isInvite)
	}
	return nil
}

func (m *MockDrimeClient) GetWorkspaceStats(ctx context.Context, workspaceID int64) (*WorkspaceStats, error) {
	if m.GetWorkspaceStatsFunc != nil {
		return m.GetWorkspaceStatsFunc(ctx, workspaceID)
	}
	return nil, nil
}

func (m *MockDrimeClient) CreateWorkspace(ctx context.Context, name string) (*Workspace, error) {
	if m.CreateWorkspaceFunc != nil {
		return m.CreateWorkspaceFunc(ctx, name)
	}
	return nil, nil
}

func (m *MockDrimeClient) UpdateWorkspace(ctx context.Context, workspaceID int64, name string) (*Workspace, error) {
	if m.UpdateWorkspaceFunc != nil {
		return m.UpdateWorkspaceFunc(ctx, workspaceID, name)
	}
	return nil, nil
}

func (m *MockDrimeClient) DeleteWorkspace(ctx context.Context, workspaceID int64) error {
	if m.DeleteWorkspaceFunc != nil {
		return m.DeleteWorkspaceFunc(ctx, workspaceID)
	}
	return nil
}

func (m *MockDrimeClient) GetUserFolders(ctx context.Context, userID int64, workspaceID int64) ([]FileEntry, error) {
	return m.GetUserFoldersFunc(ctx, userID, workspaceID)
}

func (m *MockDrimeClient) GetFolderPath(ctx context.Context, folderHash string, workspaceID int64) ([]FileEntry, error) {
	if m.GetFolderPathFunc != nil {
		return m.GetFolderPathFunc(ctx, folderHash, workspaceID)
	}
	return nil, nil
}

func (m *MockDrimeClient) ListByParentID(ctx context.Context, parentID *int64) ([]FileEntry, error) {
	return m.ListByParentIDFunc(ctx, parentID)
}

func (m *MockDrimeClient) ListByParentIDWithOptions(ctx context.Context, parentID *int64, opts *ListEntriesOptions) ([]FileEntry, error) {
	if m.ListByParentIDWithOptionsFunc != nil {
		return m.ListByParentIDWithOptionsFunc(ctx, parentID, opts)
	}
	return m.ListByParentIDFunc(ctx, parentID)
}

func (m *MockDrimeClient) StarEntries(ctx context.Context, entryIDs []int64, workspaceID int64) error {
	if m.StarEntriesFunc != nil {
		return m.StarEntriesFunc(ctx, entryIDs, workspaceID)
	}
	return nil
}

func (m *MockDrimeClient) UnstarEntries(ctx context.Context, entryIDs []int64, workspaceID int64) error {
	if m.UnstarEntriesFunc != nil {
		return m.UnstarEntriesFunc(ctx, entryIDs, workspaceID)
	}
	return nil
}

func (m *MockDrimeClient) RestoreEntries(ctx context.Context, entryIDs []int64, workspaceID int64) error {
	if m.RestoreEntriesFunc != nil {
		return m.RestoreEntriesFunc(ctx, entryIDs, workspaceID)
	}
	return nil
}

func (m *MockDrimeClient) EmptyTrash(ctx context.Context, workspaceID int64) error {
	if m.EmptyTrashFunc != nil {
		return m.EmptyTrashFunc(ctx, workspaceID)
	}
	return nil
}

func (m *MockDrimeClient) Upload(ctx context.Context, reader io.Reader, name string, parentID *int64, size int64, workspaceID int64) (*FileEntry, error) {
	return m.UploadFunc(ctx, reader, name, parentID, size, workspaceID)
}

func (m *MockDrimeClient) Download(ctx context.Context, hash string, w io.Writer, progress func(int64, int64)) (*FileEntry, error) {
	return m.DownloadFunc(ctx, hash, w, progress)
}

func (m *MockDrimeClient) DownloadWithOptions(ctx context.Context, hash string, w io.Writer, progress func(int64, int64), opts *DownloadOptions) (*FileEntry, error) {
	if m.DownloadWithOptionsFunc != nil {
		return m.DownloadWithOptionsFunc(ctx, hash, w, progress, opts)
	}
	// Fall back to regular download if not mocked
	return m.DownloadFunc(ctx, hash, w, progress)
}

func (m *MockDrimeClient) CreateFolder(ctx context.Context, name string, parentID *int64, workspaceID int64) (*FileEntry, error) {
	return m.CreateFolderFunc(ctx, name, parentID, workspaceID)
}

func (m *MockDrimeClient) DeleteEntries(ctx context.Context, entryIDs []int64, workspaceID int64) error {
	return m.DeleteEntriesFunc(ctx, entryIDs, workspaceID)
}

func (m *MockDrimeClient) DeleteEntriesForever(ctx context.Context, entryIDs []int64, workspaceID int64) error {
	if m.DeleteEntriesForeverFunc != nil {
		return m.DeleteEntriesForeverFunc(ctx, entryIDs, workspaceID)
	}
	return nil
}

func (m *MockDrimeClient) MoveEntries(ctx context.Context, entryIDs []int64, destinationParentID *int64, workspaceID int64, destinationWorkspaceID *int64) error {
	return m.MoveEntriesFunc(ctx, entryIDs, destinationParentID, workspaceID, destinationWorkspaceID)
}

func (m *MockDrimeClient) CopyEntries(ctx context.Context, entryIDs []int64, destinationParentID *int64, workspaceID int64, destinationWorkspaceID *int64) ([]FileEntry, error) {
	return m.CopyEntriesFunc(ctx, entryIDs, destinationParentID, workspaceID, destinationWorkspaceID)
}

func (m *MockDrimeClient) RenameEntry(ctx context.Context, entryID int64, newName string, workspaceID int64) (*FileEntry, error) {
	return m.RenameEntryFunc(ctx, entryID, newName, workspaceID)
}

func (m *MockDrimeClient) GetSpaceUsage(ctx context.Context, workspaceID int64) (*SpaceUsage, error) {
	return m.GetSpaceUsageFunc(ctx, workspaceID)
}

func (m *MockDrimeClient) ExtractEntry(ctx context.Context, entryID int64, parentID *int64, workspaceID int64) error {
	return m.ExtractEntryFunc(ctx, entryID, parentID, workspaceID)
}

func (m *MockDrimeClient) GetEntry(ctx context.Context, entryID int64, workspaceID int64) (*FileEntry, error) {
	return m.GetEntryFunc(ctx, entryID, workspaceID)
}

func (m *MockDrimeClient) Search(ctx context.Context, query string) ([]FileEntry, error) {
	return m.SearchFunc(ctx, query)
}

func (m *MockDrimeClient) SearchWithOptions(ctx context.Context, query string, opts *ListEntriesOptions) ([]FileEntry, error) {
	if m.SearchWithOptionsFunc != nil {
		return m.SearchWithOptionsFunc(ctx, query, opts)
	}
	return m.SearchFunc(ctx, query)
}

func (m *MockDrimeClient) ValidateEntries(ctx context.Context, req ValidateRequest) (*ValidateResponse, error) {
	if m.ValidateEntriesFunc != nil {
		return m.ValidateEntriesFunc(ctx, req)
	}
	return &ValidateResponse{Status: "success"}, nil
}

func (m *MockDrimeClient) GetAvailableName(ctx context.Context, req GetAvailableNameRequest) (*GetAvailableNameResponse, error) {
	if m.GetAvailableNameFunc != nil {
		return m.GetAvailableNameFunc(ctx, req)
	}
	return &GetAvailableNameResponse{Status: "success", Name: req.Name + " (1)"}, nil
}

// Vault mock implementations

func (m *MockDrimeClient) GetVaultMetadata(ctx context.Context) (*VaultMeta, error) {
	if m.GetVaultMetadataFunc != nil {
		return m.GetVaultMetadataFunc(ctx)
	}
	return nil, nil
}

func (m *MockDrimeClient) InitializeVault(ctx context.Context, salt, check, iv string) (*VaultMeta, error) {
	if m.InitializeVaultFunc != nil {
		return m.InitializeVaultFunc(ctx, salt, check, iv)
	}
	return nil, nil
}

func (m *MockDrimeClient) GetVaultFolders(ctx context.Context, userID int64) ([]FileEntry, error) {
	if m.GetVaultFoldersFunc != nil {
		return m.GetVaultFoldersFunc(ctx, userID)
	}
	return nil, nil
}

func (m *MockDrimeClient) ListVaultEntries(ctx context.Context, folderHash string) ([]FileEntry, error) {
	if m.ListVaultEntriesFunc != nil {
		return m.ListVaultEntriesFunc(ctx, folderHash)
	}
	return nil, nil
}

func (m *MockDrimeClient) MoveVaultEntries(ctx context.Context, entryIDs []int64, destinationID *int64) error {
	if m.MoveVaultEntriesFunc != nil {
		return m.MoveVaultEntriesFunc(ctx, entryIDs, destinationID)
	}
	return nil
}

func (m *MockDrimeClient) DeleteVaultEntries(ctx context.Context, entryIDs []int64) error {
	if m.DeleteVaultEntriesFunc != nil {
		return m.DeleteVaultEntriesFunc(ctx, entryIDs)
	}
	return nil
}

func (m *MockDrimeClient) CreateVaultFolder(ctx context.Context, name string, parentID *int64, vaultID int64) (*FileEntry, error) {
	if m.CreateVaultFolderFunc != nil {
		return m.CreateVaultFolderFunc(ctx, name, parentID, vaultID)
	}
	return nil, nil
}

func (m *MockDrimeClient) DownloadEncrypted(ctx context.Context, hash string, w io.Writer, progress func(int64, int64)) (*FileEntry, error) {
	if m.DownloadEncryptedFunc != nil {
		return m.DownloadEncryptedFunc(ctx, hash, w, progress)
	}
	return nil, nil
}

func (m *MockDrimeClient) UploadToVault(ctx context.Context, encryptedContent []byte, name string, parentID *int64, vaultID int64, ivBase64 string) (*FileEntry, error) {
	if m.UploadToVaultFunc != nil {
		return m.UploadToVaultFunc(ctx, encryptedContent, name, parentID, vaultID, ivBase64)
	}
	return nil, nil
}

// Sharing mock implementations

func (m *MockDrimeClient) CreateShareableLink(ctx context.Context, entryID int64, req ShareableLinkRequest) (*ShareableLink, error) {
	if m.CreateShareableLinkFunc != nil {
		return m.CreateShareableLinkFunc(ctx, entryID, req)
	}
	return nil, nil
}

func (m *MockDrimeClient) UpdateShareableLink(ctx context.Context, entryID int64, req ShareableLinkRequest) (*ShareableLink, error) {
	if m.UpdateShareableLinkFunc != nil {
		return m.UpdateShareableLinkFunc(ctx, entryID, req)
	}
	return nil, nil
}

func (m *MockDrimeClient) DeleteShareableLink(ctx context.Context, entryID int64) error {
	if m.DeleteShareableLinkFunc != nil {
		return m.DeleteShareableLinkFunc(ctx, entryID)
	}
	return nil
}

func (m *MockDrimeClient) GetShareableLink(ctx context.Context, entryID int64) (*ShareableLink, error) {
	if m.GetShareableLinkFunc != nil {
		return m.GetShareableLinkFunc(ctx, entryID)
	}
	return nil, nil
}

func (m *MockDrimeClient) ShareEntry(ctx context.Context, entryID int64, emails []string, permissions []string) error {
	if m.ShareEntryFunc != nil {
		return m.ShareEntryFunc(ctx, entryID, emails, permissions)
	}
	return nil
}

func (m *MockDrimeClient) CreateFileRequest(ctx context.Context, entryID int64, title, description string) (*ShareableLink, error) {
	if m.CreateFileRequestFunc != nil {
		return m.CreateFileRequestFunc(ctx, entryID, title, description)
	}
	return nil, nil
}

func (m *MockDrimeClient) ListFileRequests(ctx context.Context) ([]FileRequest, error) {
	if m.ListFileRequestsFunc != nil {
		return m.ListFileRequestsFunc(ctx)
	}
	return nil, nil
}

func (m *MockDrimeClient) DeleteFileRequest(ctx context.Context, requestID int64) error {
	if m.DeleteFileRequestFunc != nil {
		return m.DeleteFileRequestFunc(ctx, requestID)
	}
	return nil
}

// Tracking mock implementations

func (m *MockDrimeClient) GetTrackedFiles(ctx context.Context) ([]TrackedFile, error) {
	if m.GetTrackedFilesFunc != nil {
		return m.GetTrackedFilesFunc(ctx)
	}
	return nil, nil
}

func (m *MockDrimeClient) GetTrackingStats(ctx context.Context, entryID int64) (*TrackingStatsResponse, error) {
	if m.GetTrackingStatsFunc != nil {
		return m.GetTrackingStatsFunc(ctx, entryID)
	}
	return nil, nil
}

func (m *MockDrimeClient) SetTracking(ctx context.Context, entryID int64, tracked bool) error {
	if m.SetTrackingFunc != nil {
		return m.SetTrackingFunc(ctx, entryID, tracked)
	}
	return nil
}
