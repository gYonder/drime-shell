package api

import (
	"time"
)

// Tag represents a tag that can be applied to file entries
type Tag struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Workspace represents a Drime Cloud workspace
type Workspace struct {
	ID           int64             `json:"id"`
	Name         string            `json:"name"`
	Avatar       string            `json:"avatar"`
	OwnerID      int64             `json:"owner_id"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	MembersCount int               `json:"members_count"`
	CurrentUser  *WorkspaceMember  `json:"currentUser"`
	Owner        *WorkspaceMember  `json:"owner"`
	Members      []WorkspaceMember `json:"members"`
	Invites      []WorkspaceInvite `json:"invites"`
}

// WorkspaceMember represents a member of a workspace
type WorkspaceMember struct {
	ID          int64  `json:"id"`
	MemberID    int64  `json:"member_id"`
	WorkspaceID int64  `json:"workspace_id"`
	Email       string `json:"email"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Avatar      string `json:"avatar"`
	JoinedAt    string `json:"joined_at"`
	DisplayName string `json:"display_name"`
	RoleName    string `json:"role_name"`
	RoleID      int    `json:"role_id"`
	IsOwner     bool   `json:"is_owner"`
	ModelType   string `json:"model_type"`
	User        *User  `json:"user,omitempty"`
}

// WorkspaceInvite represents a pending workspace invitation
type WorkspaceInvite struct {
	ID          int64  `json:"id"`
	WorkspaceID int64  `json:"workspace_id"`
	Email       string `json:"email"`
	RoleID      int    `json:"role_id"`
	RoleName    string `json:"role_name"`
	CreatedAt   string `json:"created_at"`
	ModelType   string `json:"model_type"`
}

// WorkspaceRole represents a role in a workspace
type WorkspaceRole struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// WorkspaceStats represents usage statistics for a workspace
type WorkspaceStats struct {
	Files  int    `json:"files"`
	Size   int64  `json:"size"`
	Status string `json:"status"`
}

// User represents a Drime Cloud user
type User struct {
	DisplayName string `json:"display_name"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Email       string `json:"email"`
	ID          int64  `json:"id"`
	AccessToken string `json:"access_token,omitempty"` // Only populated after login
}

// Name returns the display name, or first name, or email as fallback
func (u *User) Name() string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	if u.FirstName != "" {
		return u.FirstName
	}
	return u.Email
}

// FileEntryUser represents user info embedded in file entries
type FileEntryUser struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	ID          int64  `json:"id"`
	OwnsEntry   bool   `json:"owns_entry"`
}

// Name returns the display name or email as fallback
func (u *FileEntryUser) Name() string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Email
}

// FileEntryPermissions represents the user's permissions on a file entry
type FileEntryPermissions struct {
	Update   bool `json:"files.update"`
	Create   bool `json:"files.create"`
	Download bool `json:"files.download"`
	Delete   bool `json:"files.delete"`
}

// FileEntry represents a file or folder in Drime Cloud
type FileEntry struct {
	ParentID    *int64               `json:"parent_id"`
	Users       []FileEntryUser      `json:"users"`
	Tags        []Tag                `json:"tags"`
	Name        string               `json:"name"`
	Type        string               `json:"type"` // "image", "folder", "text", "audio", "video", "pdf"
	Hash        string               `json:"hash"`
	Mime        string               `json:"mime"`
	DeletedAt   *time.Time           `json:"deleted_at"` // Set if item is in trash
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
	ID          int64                `json:"id"`
	Size        int64                `json:"file_size"`
	OwnerID     int64                `json:"owner_id"`
	WorkspaceID int64                `json:"workspace_id"`
	Permissions FileEntryPermissions `json:"permissions"`
	Public      bool                 `json:"public"`
	Tracked     int                  `json:"tracked"` // 0 or 1
	// Vault-specific fields (present when is_encrypted = 1)
	IsEncrypted int    `json:"is_encrypted"` // 1 = encrypted (vault), 0 = regular
	VaultID     int64  `json:"vault_id"`     // Links to vault record
	IV          string `json:"iv"`           // Per-file IV for AES-GCM decryption (base64)
}

// ShareableLink represents a public link for a file entry
type ShareableLink struct {
	ID                 int64      `json:"id"`
	Hash               string     `json:"hash"`
	UserID             int64      `json:"user_id"`
	EntryID            int64      `json:"entry_id"`
	AllowEdit          bool       `json:"allow_edit"`
	AllowDownload      bool       `json:"allow_download"`
	Password           *string    `json:"password"`
	ExpiresAt          *time.Time `json:"expires_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	Perso              int        `json:"perso"`                // 1 if personal link
	PersonnalLinkValue string     `json:"personnal_link_value"` // Custom link suffix
}

// FileRequestPayload represents the payload for creating a file request
type FileRequestPayload struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// LinkRestrictions defines access restrictions for a shareable link
type LinkRestrictions struct {
	Emails    []string `json:"emails,omitempty"`
	Domains   []string `json:"domains,omitempty"`
	Countries []string `json:"countries,omitempty"`
	Password  string   `json:"password,omitempty"`
}

// ShareableLinkRequest is the request body for creating/updating a shareable link
type ShareableLinkRequest struct {
	Request            *FileRequestPayload `json:"request,omitempty"`
	Password           *string             `json:"password,omitempty"`
	ExpiresAt          *string             `json:"expiresAt"` // ISO 8601 string, explicit null if nil
	AllowEdit          bool                `json:"allowEdit"`
	AllowDownload      bool                `json:"allowDownload"`
	Active             bool                `json:"active"`
	Restrictions       *LinkRestrictions   `json:"restrictions,omitempty"`
	GenerateNewHash    bool                `json:"generateNewHash"`
	TurnstileToken     string              `json:"turnstileToken,omitempty"`
	DisableWatermark   bool                `json:"disableWatermark"`
	HideViewerDownload bool                `json:"hideViewerDownload"`
	Title              string              `json:"title,omitempty"`
	Description        string              `json:"description,omitempty"`
	ShowSocialButtons  bool                `json:"showSocialButtons"`
	ShowAvatar         bool                `json:"showAvatar"`
	ShowFileName       bool                `json:"showFileName"`
	ShowFileSize       bool                `json:"showFileSize"`
	ShowFileCount      bool                `json:"showFileCount"`
	ShowBreadcrumbs    bool                `json:"showBreadcrumbs"`
	CustomDomainId     int                 `json:"customDomainId,omitempty"`
	Layout             string              `json:"layout,omitempty"`
	Theme              string              `json:"theme,omitempty"`
	CustomCSS          string              `json:"customCSS,omitempty"`
	GoogleAnalyticsId  string              `json:"gaId,omitempty"`
	FacebookPixelId    string              `json:"fbPixelId,omitempty"`
	BaiduAnalyticsId   string              `json:"baId,omitempty"`
	YandexMetrikaId    string              `json:"yandexId,omitempty"`
	AdSenseSlotId      string              `json:"adSlotId,omitempty"`
	AdSenseClientId    string              `json:"adClientId,omitempty"`
	AdScript           string              `json:"adScript,omitempty"`
	CustomHTML         string              `json:"customHTML,omitempty"`
	Meta               *map[string]string  `json:"meta,omitempty"`
	PersonalLink       bool                `json:"perso,omitempty"`                // true if personalized link
	PersonnalLinkValue string              `json:"personnal_link_value,omitempty"` // Custom link suffix
}

// FileRequest represents a file request entry
type FileRequest struct {
	ID              int64      `json:"id"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	FileName        string     `json:"file_name"`
	ShareHash       string     `json:"share_hash"`
	SubmittersCount int        `json:"submitters_count"`
	UploadsCount    int        `json:"uploads_count"`
	CreatedAt       time.Time  `json:"created_at"`
	ExpiresAt       *time.Time `json:"expires_at"`
}

// TrackedFile represents a file in the tracking list
type TrackedFile struct {
	ID          int64           `json:"id"`
	Name        string          `json:"name"`
	Type        string          `json:"type"`
	FileSize    int64           `json:"file_size"`
	ViewsNumber int             `json:"views_number"`
	DlNumber    int             `json:"dl_number"`
	Views       []TrackingEvent `json:"views"`
}

// TrackingEvent represents a single view or download event
type TrackingEvent struct {
	ID       int64   `json:"id"`
	FileID   int64   `json:"file_id"`
	IP       string  `json:"ip"`
	Date     string  `json:"date"`
	Action   string  `json:"action"` // "view" or "download"
	Location string  `json:"location"`
	UserID   *int64  `json:"user_id"`
	Email    *string `json:"email"`
}

// TrackingStatsResponse represents the response from /track/infos/{id}
type TrackingStatsResponse struct {
	Status string          `json:"status"`
	Views  []TrackingEvent `json:"views"`
}

// SetTrackingRequest represents the payload for toggling tracking
type SetTrackingRequest struct {
	FileID  int64 `json:"file_id"`
	Tracked bool  `json:"tracked"`
}

// IsStarred returns true if this entry has the "starred" tag
func (e *FileEntry) IsStarred() bool {
	for _, tag := range e.Tags {
		if tag.Name == "starred" {
			return true
		}
	}
	return false
}

// IsInTrash returns true if this entry is in trash
func (e *FileEntry) IsInTrash() bool {
	return e.DeletedAt != nil
}

// Owner returns the owner's display name
func (e *FileEntry) Owner() string {
	// First check the users array for the owner
	for _, u := range e.Users {
		if u.OwnsEntry {
			return u.Name()
		}
	}
	// Fallback: if no owner found in users, return empty
	return ""
}

// UnixMode returns a Unix-style permission string like "drwxr-xr-x" or "-rw-r--r--"
// Maps Drime permissions: download->r, update/create/delete->w, folders get x
func (e *FileEntry) UnixMode() string {
	var mode [10]byte

	// First char: type
	if e.Type == "folder" {
		mode[0] = 'd'
	} else {
		mode[0] = '-'
	}

	// Owner permissions (positions 1-3)
	if e.Permissions.Download {
		mode[1] = 'r'
	} else {
		mode[1] = '-'
	}
	if e.Permissions.Update || e.Permissions.Create || e.Permissions.Delete {
		mode[2] = 'w'
	} else {
		mode[2] = '-'
	}
	if e.Type == "folder" {
		mode[3] = 'x'
	} else {
		mode[3] = '-'
	}

	// Group permissions (positions 4-6) - mirror owner for simplicity
	mode[4] = mode[1]
	mode[5] = mode[2]
	mode[6] = mode[3]

	// Other permissions (positions 7-9) - read-only if downloadable
	if e.Permissions.Download {
		mode[7] = 'r'
	} else {
		mode[7] = '-'
	}
	mode[8] = '-'
	if e.Type == "folder" {
		mode[9] = 'x'
	} else {
		mode[9] = '-'
	}

	return string(mode[:])
}

// FileInfo is a unified interface for file/folder information
type FileInfo interface {
	Name() string
	Size() int64
	Mode() string // "drwxr-xr-x" etc
	ModTime() time.Time
	IsDir() bool
}

type SpaceUsage struct {
	Status    string `json:"status"`
	Used      int64  `json:"used"`
	Available int64  `json:"available"`
}

// APIError represents a structured error from the Drime API
type APIError struct {
	Message    string `json:"message"`
	Status     string `json:"status"`
	RetryAfter int    `json:"retry_after,omitempty"` // From Retry-After header
}

func (e *APIError) Error() string {
	return e.Message
}

// ValidateFile represents a file to be validated
type ValidateFile struct {
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	RelativePath string `json:"relativePath"`
}

// ValidateRequest represents the request body for /uploads/validate
type ValidateRequest struct {
	Files       []ValidateFile `json:"files"`
	WorkspaceID int64          `json:"workspaceId"`
}

// ValidateResponse represents the response from /uploads/validate
type ValidateResponse struct {
	Errors     bool     `json:"errors"`
	Duplicates []string `json:"duplicates"`
	Status     string   `json:"status"`
}

// GetAvailableNameRequest represents the request body for /entry/getAvailableName
type GetAvailableNameRequest struct {
	Name        string `json:"name"`
	ParentID    *int64 `json:"parentId"`
	WorkspaceID int64  `json:"workspaceId"`
}

// GetAvailableNameResponse represents the response from /entry/getAvailableName
type GetAvailableNameResponse struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// VaultMeta represents vault metadata for password verification
type VaultMeta struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"user_id"`
	Salt      string `json:"salt"`  // Base64-encoded PBKDF2 salt
	Check     string `json:"check"` // Base64-encoded encrypted verification value
	IV        string `json:"iv"`    // Base64-encoded IV for check decryption
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// VaultFileEntry extends FileEntry with vault-specific fields
// Note: These fields are also present in FileEntry from vault API responses
type VaultFileEntry struct {
	FileEntry
	IsEncrypted int    `json:"is_encrypted"` // 1 = encrypted, 0 = regular
	VaultID     int64  `json:"vault_id"`     // Links to vault record
	IV          string `json:"iv"`           // Per-file IV for AES-GCM decryption (base64)
}
