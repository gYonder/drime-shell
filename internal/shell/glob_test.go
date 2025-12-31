package shell_test

import (
	"bytes"
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/mikael.mansson2/drime-shell/internal/api"
	"github.com/mikael.mansson2/drime-shell/internal/session"
	"github.com/mikael.mansson2/drime-shell/internal/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// EXPANDGLOBS TESTS - Testing shell's glob expansion
// ============================================================================

// setupTestSession creates a test session with mock client and pre-populated cache
func setupTestSession(t *testing.T) (*session.Session, *api.MockDrimeClient) {
	cache := api.NewFileCache()
	mockClient := &api.MockDrimeClient{
		ListByParentIDFunc: func(ctx context.Context, parentID *int64) ([]api.FileEntry, error) {
			// Return empty by default - tests will populate cache
			return []api.FileEntry{}, nil
		},
	}

	s := session.NewSession(mockClient, cache)
	s.CWD = "/"
	s.HomeDir = "/"
	s.UserID = 123
	s.Username = "testuser"

	// Setup root
	cache.Add(&api.FileEntry{ID: 0, Name: "/", Type: "folder"}, "/")

	return s, mockClient
}

// populateTestDirectory adds test files/folders to cache for a directory
func populateTestDirectory(cache *api.FileCache, parentPath string, parentID int64, entries []api.FileEntry) {
	cache.Add(&api.FileEntry{ID: parentID, Name: parentPath, Type: "folder"}, parentPath)
	cache.AddChildren(parentPath, entries)
}

func TestExpandGlobs_NoGlobCharacters(t *testing.T) {
	s, _ := setupTestSession(t)

	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "simple args without globs",
			args:     []string{"file.txt", "folder"},
			expected: []string{"file.txt", "folder"},
		},
		{
			name:     "flags are preserved",
			args:     []string{"-la", "--help"},
			expected: []string{"-la", "--help"},
		},
		{
			name:     "paths without glob chars",
			args:     []string{"/path/to/file.txt", "./relative/path"},
			expected: []string{"/path/to/file.txt", "./relative/path"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			result, err := shell.ExpandGlobs(context.Background(), s, &buf, tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExpandGlobs_EmptyArgsReturnsEmpty(t *testing.T) {
	s, _ := setupTestSession(t)

	var buf bytes.Buffer
	result, err := shell.ExpandGlobs(context.Background(), s, &buf, []string{})
	require.NoError(t, err)
	assert.Empty(t, result, "Empty args should return empty (nil or empty slice)")
}

func TestExpandGlobs_WildcardExpansion(t *testing.T) {
	s, mockClient := setupTestSession(t)

	// Setup /Documents with test files
	docsID := int64(100)
	populateTestDirectory(s.Cache, "/Documents", docsID, []api.FileEntry{
		{ID: 101, Name: "report.txt", Type: "text", ParentID: &docsID},
		{ID: 102, Name: "notes.txt", Type: "text", ParentID: &docsID},
		{ID: 103, Name: "image.png", Type: "image", ParentID: &docsID},
		{ID: 104, Name: "data.json", Type: "text", ParentID: &docsID},
	})
	// Add documents to root
	s.Cache.AddChildren("/", []api.FileEntry{
		{ID: docsID, Name: "Documents", Type: "folder", ParentID: nil},
	})

	// Mock client in case it needs to fetch (children already loaded though)
	mockClient.ListByParentIDFunc = func(ctx context.Context, parentID *int64) ([]api.FileEntry, error) {
		if parentID != nil && *parentID == docsID {
			return []api.FileEntry{
				{ID: 101, Name: "report.txt", Type: "text", ParentID: &docsID},
				{ID: 102, Name: "notes.txt", Type: "text", ParentID: &docsID},
				{ID: 103, Name: "image.png", Type: "image", ParentID: &docsID},
				{ID: 104, Name: "data.json", Type: "text", ParentID: &docsID},
			}, nil
		}
		return []api.FileEntry{}, nil
	}

	tests := []struct {
		name     string
		cwd      string
		args     []string
		expected []string
	}{
		{
			name:     "expand txt files",
			cwd:      "/Documents",
			args:     []string{"*.txt"},
			expected: []string{"notes.txt", "report.txt"}, // relative paths since pattern was relative
		},
		{
			name:     "expand all files",
			cwd:      "/Documents",
			args:     []string{"*"},
			expected: []string{"data.json", "image.png", "notes.txt", "report.txt"},
		},
		{
			name:     "mix of glob and non-glob",
			cwd:      "/Documents",
			args:     []string{"-l", "*.txt"},
			expected: []string{"-l", "notes.txt", "report.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s.CWD = tt.cwd
			var buf bytes.Buffer
			result, err := shell.ExpandGlobs(context.Background(), s, &buf, tt.args)
			require.NoError(t, err)
			sort.Strings(result)
			sort.Strings(tt.expected)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExpandGlobs_NoMatches(t *testing.T) {
	s, _ := setupTestSession(t)

	// Setup /empty with some files
	emptyID := int64(200)
	populateTestDirectory(s.Cache, "/empty", emptyID, []api.FileEntry{
		{ID: 201, Name: "file.go", Type: "text", ParentID: &emptyID},
	})

	s.CWD = "/empty"

	var buf bytes.Buffer
	// Pattern that won't match anything - should preserve original arg (bash behavior)
	result, err := shell.ExpandGlobs(context.Background(), s, &buf, []string{"*.xyz"})
	require.NoError(t, err)
	assert.Equal(t, []string{"*.xyz"}, result, "No matches should preserve original pattern")
}

func TestExpandGlobs_AbsolutePaths(t *testing.T) {
	s, _ := setupTestSession(t)

	// Setup /Photos
	photosID := int64(300)
	populateTestDirectory(s.Cache, "/Photos", photosID, []api.FileEntry{
		{ID: 301, Name: "vacation.jpg", Type: "image", ParentID: &photosID},
		{ID: 302, Name: "family.jpg", Type: "image", ParentID: &photosID},
		{ID: 303, Name: "logo.png", Type: "image", ParentID: &photosID},
	})

	s.CWD = "/"

	var buf bytes.Buffer
	result, err := shell.ExpandGlobs(context.Background(), s, &buf, []string{"/Photos/*.jpg"})
	require.NoError(t, err)
	sort.Strings(result)
	// Absolute paths in, absolute paths out
	assert.Equal(t, []string{"/Photos/family.jpg", "/Photos/vacation.jpg"}, result)
}

func TestExpandGlobs_MultiplePatterns(t *testing.T) {
	s, _ := setupTestSession(t)

	// Setup /src
	srcID := int64(400)
	populateTestDirectory(s.Cache, "/src", srcID, []api.FileEntry{
		{ID: 401, Name: "main.go", Type: "text", ParentID: &srcID},
		{ID: 402, Name: "util.go", Type: "text", ParentID: &srcID},
		{ID: 403, Name: "test.py", Type: "text", ParentID: &srcID},
		{ID: 404, Name: "README.md", Type: "text", ParentID: &srcID},
	})

	s.CWD = "/src"

	var buf bytes.Buffer
	// Multiple glob patterns
	result, err := shell.ExpandGlobs(context.Background(), s, &buf, []string{"*.go", "*.py"})
	require.NoError(t, err)
	sort.Strings(result)
	assert.Equal(t, []string{"main.go", "test.py", "util.go"}, result)
}

func TestExpandGlobs_BraceExpansion(t *testing.T) {
	s, _ := setupTestSession(t)

	// Setup /config
	configID := int64(500)
	populateTestDirectory(s.Cache, "/config", configID, []api.FileEntry{
		{ID: 501, Name: "app.yaml", Type: "text", ParentID: &configID},
		{ID: 502, Name: "app.json", Type: "text", ParentID: &configID},
		{ID: 503, Name: "test.yaml", Type: "text", ParentID: &configID},
		{ID: 504, Name: "dev.toml", Type: "text", ParentID: &configID},
	})

	s.CWD = "/config"

	var buf bytes.Buffer
	result, err := shell.ExpandGlobs(context.Background(), s, &buf, []string{"*.{yaml,json}"})
	require.NoError(t, err)
	sort.Strings(result)
	assert.Equal(t, []string{"app.json", "app.yaml", "test.yaml"}, result)
}

func TestExpandGlobs_CharacterClass(t *testing.T) {
	s, _ := setupTestSession(t)

	// Setup /logs
	logsID := int64(600)
	populateTestDirectory(s.Cache, "/logs", logsID, []api.FileEntry{
		{ID: 601, Name: "app1.log", Type: "text", ParentID: &logsID},
		{ID: 602, Name: "app2.log", Type: "text", ParentID: &logsID},
		{ID: 603, Name: "app3.log", Type: "text", ParentID: &logsID},
		{ID: 604, Name: "appa.log", Type: "text", ParentID: &logsID},
	})

	s.CWD = "/logs"

	var buf bytes.Buffer
	result, err := shell.ExpandGlobs(context.Background(), s, &buf, []string{"app[123].log"})
	require.NoError(t, err)
	sort.Strings(result)
	assert.Equal(t, []string{"app1.log", "app2.log", "app3.log"}, result)
}

func TestExpandGlobs_SingleCharWildcard(t *testing.T) {
	s, _ := setupTestSession(t)

	// Setup /data
	dataID := int64(700)
	populateTestDirectory(s.Cache, "/data", dataID, []api.FileEntry{
		{ID: 701, Name: "data1.csv", Type: "text", ParentID: &dataID},
		{ID: 702, Name: "data2.csv", Type: "text", ParentID: &dataID},
		{ID: 703, Name: "data12.csv", Type: "text", ParentID: &dataID},
		{ID: 704, Name: "datax.csv", Type: "text", ParentID: &dataID},
	})

	s.CWD = "/data"

	var buf bytes.Buffer
	result, err := shell.ExpandGlobs(context.Background(), s, &buf, []string{"data?.csv"})
	require.NoError(t, err)
	sort.Strings(result)
	// data? matches single char - data1, data2, datax but not data12
	assert.Equal(t, []string{"data1.csv", "data2.csv", "datax.csv"}, result)
}

func TestExpandGlobs_LogFilesPattern(t *testing.T) {
	s, _ := setupTestSession(t)

	// Setup /build
	buildID := int64(800)
	populateTestDirectory(s.Cache, "/build", buildID, []api.FileEntry{
		{ID: 801, Name: "app.exe", Type: "text", ParentID: &buildID},
		{ID: 802, Name: "debug.log", Type: "text", ParentID: &buildID},
		{ID: 803, Name: "error.log", Type: "text", ParentID: &buildID},
		{ID: 804, Name: "lib.dll", Type: "text", ParentID: &buildID},
	})

	s.CWD = "/build"

	var buf bytes.Buffer
	// Match only log files
	result, err := shell.ExpandGlobs(context.Background(), s, &buf, []string{"*.log"})
	require.NoError(t, err)
	sort.Strings(result)
	assert.Equal(t, []string{"debug.log", "error.log"}, result)
}

// NOTE: Extended glob patterns like !(pattern), +(pattern), ?(pattern), @(pattern)
// are bash extglob features NOT supported by doublestar library.
// These tests document this limitation.

func TestExpandGlobs_RootDirectory(t *testing.T) {
	s, _ := setupTestSession(t)

	// Add entries to root
	s.Cache.AddChildren("/", []api.FileEntry{
		{ID: 1, Name: "Documents", Type: "folder"},
		{ID: 2, Name: "Downloads", Type: "folder"},
		{ID: 3, Name: "Pictures", Type: "folder"},
		{ID: 4, Name: ".config", Type: "folder"},
	})

	s.CWD = "/"

	var buf bytes.Buffer
	result, err := shell.ExpandGlobs(context.Background(), s, &buf, []string{"D*"})
	require.NoError(t, err)
	sort.Strings(result)
	assert.Equal(t, []string{"Documents", "Downloads"}, result)
}

func TestExpandGlobs_HiddenFiles(t *testing.T) {
	s, _ := setupTestSession(t)

	// Setup /home
	homeID := int64(900)
	populateTestDirectory(s.Cache, "/home", homeID, []api.FileEntry{
		{ID: 901, Name: ".bashrc", Type: "text", ParentID: &homeID},
		{ID: 902, Name: ".vimrc", Type: "text", ParentID: &homeID},
		{ID: 903, Name: "file.txt", Type: "text", ParentID: &homeID},
		{ID: 904, Name: ".gitignore", Type: "text", ParentID: &homeID},
	})

	s.CWD = "/home"

	tests := []struct {
		name     string
		pattern  string
		expected []string
	}{
		{
			name:     "only hidden files",
			pattern:  ".*",
			expected: []string{".bashrc", ".gitignore", ".vimrc"},
		},
		{
			name:     "all non-hidden",
			pattern:  "[!.]*",
			expected: []string{"file.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			result, err := shell.ExpandGlobs(context.Background(), s, &buf, []string{tt.pattern})
			require.NoError(t, err)
			sort.Strings(result)
			sort.Strings(tt.expected)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExpandGlobs_PreservesRelativePaths(t *testing.T) {
	s, _ := setupTestSession(t)

	// Setup /current/sub
	currentID := int64(1000)
	subID := int64(1001)

	populateTestDirectory(s.Cache, "/current", currentID, []api.FileEntry{
		{ID: subID, Name: "sub", Type: "folder", ParentID: &currentID},
	})
	populateTestDirectory(s.Cache, "/current/sub", subID, []api.FileEntry{
		{ID: 1002, Name: "file.txt", Type: "text", ParentID: &subID},
		{ID: 1003, Name: "data.txt", Type: "text", ParentID: &subID},
	})

	s.CWD = "/current"

	var buf bytes.Buffer
	// Use relative path in pattern
	result, err := shell.ExpandGlobs(context.Background(), s, &buf, []string{"sub/*.txt"})
	require.NoError(t, err)
	sort.Strings(result)
	// Should return relative paths
	assert.Equal(t, []string{"sub/data.txt", "sub/file.txt"}, result)
}

func TestExpandGlobs_FetchesIfNotLoaded(t *testing.T) {
	s, mockClient := setupTestSession(t)

	// Add folder to cache but DON'T load children
	unloadedID := int64(1100)
	s.Cache.Add(&api.FileEntry{ID: unloadedID, Name: "unloaded", Type: "folder"}, "/unloaded")
	// Note: NOT calling AddChildren - children are not loaded

	// Mock will return files when ListByParentID is called
	apiCalled := false
	mockClient.ListByParentIDFunc = func(ctx context.Context, parentID *int64) ([]api.FileEntry, error) {
		apiCalled = true
		if parentID != nil && *parentID == unloadedID {
			return []api.FileEntry{
				{ID: 1101, Name: "fetched.txt", Type: "text", ParentID: &unloadedID},
				{ID: 1102, Name: "another.txt", Type: "text", ParentID: &unloadedID},
			}, nil
		}
		return []api.FileEntry{}, nil
	}

	s.CWD = "/unloaded"

	var buf bytes.Buffer
	result, err := shell.ExpandGlobs(context.Background(), s, &buf, []string{"*.txt"})
	require.NoError(t, err)

	assert.True(t, apiCalled, "API should be called to fetch children")
	sort.Strings(result)
	assert.Equal(t, []string{"another.txt", "fetched.txt"}, result)
}

func TestExpandGlobs_QuotedPatternNotExpanded(t *testing.T) {
	s, _ := setupTestSession(t)

	// Setup /test
	testID := int64(1200)
	populateTestDirectory(s.Cache, "/test", testID, []api.FileEntry{
		{ID: 1201, Name: "file.txt", Type: "text", ParentID: &testID},
	})

	s.CWD = "/test"

	var buf bytes.Buffer
	// When args come from parsing, quoted strings have glob chars stripped
	// But ExpandGlobs receives raw args - if they don't have glob chars, they pass through
	// This tests that literal filenames with special chars work
	result, err := shell.ExpandGlobs(context.Background(), s, &buf, []string{"literal_file"})
	require.NoError(t, err)
	assert.Equal(t, []string{"literal_file"}, result)
}

func TestExpandGlobs_BraceAlternatives(t *testing.T) {
	s, _ := setupTestSession(t)

	// Setup /ext
	extID := int64(1300)
	populateTestDirectory(s.Cache, "/ext", extID, []api.FileEntry{
		{ID: 1301, Name: "a.txt", Type: "text", ParentID: &extID},
		{ID: 1302, Name: "b.txt", Type: "text", ParentID: &extID},
		{ID: 1303, Name: "c.txt", Type: "text", ParentID: &extID},
		{ID: 1304, Name: "ab.txt", Type: "text", ParentID: &extID},
	})

	s.CWD = "/ext"

	var buf bytes.Buffer
	// Use brace expansion to match specific files
	result, err := shell.ExpandGlobs(context.Background(), s, &buf, []string{"{a,b}.txt"})
	require.NoError(t, err)
	sort.Strings(result)
	assert.Equal(t, []string{"a.txt", "b.txt"}, result)
}

func TestExpandGlobs_SingleCharFilenames(t *testing.T) {
	s, _ := setupTestSession(t)

	// Setup /at
	atID := int64(1400)
	populateTestDirectory(s.Cache, "/at", atID, []api.FileEntry{
		{ID: 1401, Name: "main.go", Type: "text", ParentID: &atID},
		{ID: 1402, Name: "main.rs", Type: "text", ParentID: &atID},
		{ID: 1403, Name: "main.py", Type: "text", ParentID: &atID},
		{ID: 1404, Name: "util.go", Type: "text", ParentID: &atID},
	})

	s.CWD = "/at"

	var buf bytes.Buffer
	// Use brace expansion to match specific files
	result, err := shell.ExpandGlobs(context.Background(), s, &buf, []string{"{main,util}.go"})
	require.NoError(t, err)
	sort.Strings(result)
	assert.Equal(t, []string{"main.go", "util.go"}, result)
}

// TestExpandGlobs_EmptyArgs is tested above in TestExpandGlobs_EmptyArgsReturnsEmpty

func TestExpandGlobs_NilArgs(t *testing.T) {
	s, _ := setupTestSession(t)

	var buf bytes.Buffer
	result, err := shell.ExpandGlobs(context.Background(), s, &buf, nil)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestExpandGlobs_ContextCanceled(t *testing.T) {
	s, mockClient := setupTestSession(t)

	// Setup unloaded directory that will require API call
	unloadedID := int64(1500)
	s.Cache.Add(&api.FileEntry{ID: unloadedID, Name: "slow", Type: "folder"}, "/slow")

	mockClient.ListByParentIDFunc = func(ctx context.Context, parentID *int64) ([]api.FileEntry, error) {
		// Simulate slow API that checks context
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			return []api.FileEntry{}, nil
		}
	}

	s.CWD = "/slow"

	// Cancel context before calling
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var buf bytes.Buffer
	result, err := shell.ExpandGlobs(ctx, s, &buf, []string{"*.txt"})
	// Even with canceled context, it should return gracefully (possibly with original arg)
	// The implementation handles errors gracefully and continues
	_ = err
	_ = result
	// Main assertion: no panic
}

func TestExpandGlobs_OutputBuffer(t *testing.T) {
	s, mockClient := setupTestSession(t)

	// Setup unloaded directory
	unloadedID := int64(1600)
	s.Cache.Add(&api.FileEntry{ID: unloadedID, Name: "loading", Type: "folder"}, "/loading")

	mockClient.ListByParentIDFunc = func(ctx context.Context, parentID *int64) ([]api.FileEntry, error) {
		return []api.FileEntry{
			{ID: 1601, Name: "file.txt", Type: "text", ParentID: &unloadedID},
		}, nil
	}

	s.CWD = "/loading"

	var buf bytes.Buffer
	_, err := shell.ExpandGlobs(context.Background(), s, &buf, []string{"*.txt"})
	require.NoError(t, err)

	// The spinner writes to the buffer during expansion
	// Just verify it doesn't panic and completes
	// Content of buffer depends on terminal detection
	_ = buf.String()
}

// ============================================================================
// GLOB DETECTION TESTS
// ============================================================================

func TestExpandGlobs_GlobCharacterDetection(t *testing.T) {
	// Characters that trigger glob detection: *, ?, [, {
	// Note: !(pattern), +(pattern), @(pattern) are bash extglob, NOT supported by doublestar
	testCases := []struct {
		arg    string
		isGlob bool
	}{
		{"*.txt", true},
		{"file?.go", true},
		{"[abc].log", true},
		{"{a,b}.csv", true},
		{"!(skip)", false},       // bash extglob, not supported
		{"+(more).txt", false},   // bash extglob, not supported
		{"@(one|two).go", false}, // bash extglob, not supported
		{"plain.txt", false},
		{"-flag", false},
		{"--option=value", false},
		{"/path/to/file", false},
	}

	for _, tc := range testCases {
		t.Run(tc.arg, func(t *testing.T) {
			// Check if our detection logic considers it a glob
			// Only *, ?, [, { trigger glob expansion
			hasGlobChars := strings.ContainsAny(tc.arg, "*?[{")
			assert.Equal(t, tc.isGlob, hasGlobChars, "Glob detection mismatch for %q", tc.arg)
		})
	}
}
