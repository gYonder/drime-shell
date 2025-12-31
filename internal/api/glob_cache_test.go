package api_test

import (
	"sort"
	"testing"

	"github.com/mikael.mansson2/drime-shell/internal/api"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// MATCHGLOB TESTS - Testing cache's glob matching with doublestar
// ============================================================================

func TestMatchGlob_BasicWildcard(t *testing.T) {
	cache := api.NewFileCache()

	// Setup: /docs directory with various files
	docsID := int64(100)
	cache.Add(&api.FileEntry{ID: docsID, Name: "docs", Type: "folder"}, "/docs")
	cache.AddChildren("/docs", []api.FileEntry{
		{ID: 101, Name: "readme.txt", Type: "text", ParentID: &docsID},
		{ID: 102, Name: "notes.txt", Type: "text", ParentID: &docsID},
		{ID: 103, Name: "image.png", Type: "image", ParentID: &docsID},
		{ID: 104, Name: "script.go", Type: "text", ParentID: &docsID},
		{ID: 105, Name: "data.json", Type: "text", ParentID: &docsID},
	})

	tests := []struct {
		name     string
		pattern  string
		expected []string
	}{
		{
			name:     "match all txt files",
			pattern:  "*.txt",
			expected: []string{"/docs/notes.txt", "/docs/readme.txt"},
		},
		{
			name:     "match all files",
			pattern:  "*",
			expected: []string{"/docs/data.json", "/docs/image.png", "/docs/notes.txt", "/docs/readme.txt", "/docs/script.go"},
		},
		{
			name:     "match go files",
			pattern:  "*.go",
			expected: []string{"/docs/script.go"},
		},
		{
			name:     "match json files",
			pattern:  "*.json",
			expected: []string{"/docs/data.json"},
		},
		{
			name:     "no match pattern",
			pattern:  "*.yaml",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := cache.MatchGlob("/docs", tt.pattern)
			sort.Strings(matches)
			sort.Strings(tt.expected)
			if len(tt.expected) == 0 {
				assert.Empty(t, matches)
			} else {
				assert.Equal(t, tt.expected, matches)
			}
		})
	}
}

func TestMatchGlob_SingleCharWildcard(t *testing.T) {
	cache := api.NewFileCache()

	rootID := int64(0)
	cache.Add(&api.FileEntry{ID: rootID, Name: "/", Type: "folder"}, "/")
	cache.AddChildren("/", []api.FileEntry{
		{ID: 1, Name: "file1.txt", Type: "text"},
		{ID: 2, Name: "file2.txt", Type: "text"},
		{ID: 3, Name: "file3.txt", Type: "text"},
		{ID: 4, Name: "file10.txt", Type: "text"},
		{ID: 5, Name: "note1.txt", Type: "text"},
	})

	tests := []struct {
		name     string
		pattern  string
		expected []string
	}{
		{
			name:     "single char wildcard matches",
			pattern:  "file?.txt",
			expected: []string{"/file1.txt", "/file2.txt", "/file3.txt"},
		},
		{
			name:     "? matches single digit only",
			pattern:  "file??.txt",
			expected: []string{"/file10.txt"},
		},
		{
			name:     "? at start",
			pattern:  "???e1.txt",
			expected: []string{"/file1.txt", "/note1.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := cache.MatchGlob("/", tt.pattern)
			sort.Strings(matches)
			sort.Strings(tt.expected)
			assert.Equal(t, tt.expected, matches)
		})
	}
}

func TestMatchGlob_CharacterClass(t *testing.T) {
	cache := api.NewFileCache()

	photosID := int64(200)
	cache.Add(&api.FileEntry{ID: photosID, Name: "photos", Type: "folder"}, "/photos")
	cache.AddChildren("/photos", []api.FileEntry{
		{ID: 201, Name: "img_001.jpg", Type: "image", ParentID: &photosID},
		{ID: 202, Name: "img_002.jpg", Type: "image", ParentID: &photosID},
		{ID: 203, Name: "img_003.png", Type: "image", ParentID: &photosID},
		{ID: 204, Name: "img_a01.jpg", Type: "image", ParentID: &photosID},
		{ID: 205, Name: "img_b01.jpg", Type: "image", ParentID: &photosID},
	})

	tests := []struct {
		name     string
		pattern  string
		expected []string
	}{
		{
			name:     "character range 0-2",
			pattern:  "img_00[1-2].jpg",
			expected: []string{"/photos/img_001.jpg", "/photos/img_002.jpg"},
		},
		{
			name:     "character list [ab]",
			pattern:  "img_[ab]01.jpg",
			expected: []string{"/photos/img_a01.jpg", "/photos/img_b01.jpg"},
		},
		{
			name:     "negated character class",
			pattern:  "img_00[!3].jpg",
			expected: []string{"/photos/img_001.jpg", "/photos/img_002.jpg"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := cache.MatchGlob("/photos", tt.pattern)
			sort.Strings(matches)
			sort.Strings(tt.expected)
			assert.Equal(t, tt.expected, matches)
		})
	}
}

func TestMatchGlob_BraceExpansion(t *testing.T) {
	cache := api.NewFileCache()

	srcID := int64(300)
	cache.Add(&api.FileEntry{ID: srcID, Name: "src", Type: "folder"}, "/src")
	cache.AddChildren("/src", []api.FileEntry{
		{ID: 301, Name: "main.go", Type: "text", ParentID: &srcID},
		{ID: 302, Name: "main.rs", Type: "text", ParentID: &srcID},
		{ID: 303, Name: "main.py", Type: "text", ParentID: &srcID},
		{ID: 304, Name: "util.go", Type: "text", ParentID: &srcID},
		{ID: 305, Name: "test.go", Type: "text", ParentID: &srcID},
		{ID: 306, Name: "config.yaml", Type: "text", ParentID: &srcID},
		{ID: 307, Name: "config.json", Type: "text", ParentID: &srcID},
	})

	tests := []struct {
		name     string
		pattern  string
		expected []string
	}{
		{
			name:     "brace expansion for extensions",
			pattern:  "*.{go,rs}",
			expected: []string{"/src/main.go", "/src/main.rs", "/src/test.go", "/src/util.go"},
		},
		{
			name:     "brace expansion for filenames",
			pattern:  "{main,util}.go",
			expected: []string{"/src/main.go", "/src/util.go"},
		},
		{
			name:     "brace expansion config files",
			pattern:  "config.{yaml,json}",
			expected: []string{"/src/config.json", "/src/config.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := cache.MatchGlob("/src", tt.pattern)
			sort.Strings(matches)
			sort.Strings(tt.expected)
			assert.Equal(t, tt.expected, matches)
		})
	}
}

func TestMatchGlob_HiddenFiles(t *testing.T) {
	cache := api.NewFileCache()

	buildID := int64(400)
	cache.Add(&api.FileEntry{ID: buildID, Name: "build", Type: "folder"}, "/build")
	cache.AddChildren("/build", []api.FileEntry{
		{ID: 401, Name: "app.exe", Type: "text", ParentID: &buildID},
		{ID: 402, Name: "lib.dll", Type: "text", ParentID: &buildID},
		{ID: 403, Name: "debug.log", Type: "text", ParentID: &buildID},
		{ID: 404, Name: "release.zip", Type: "text", ParentID: &buildID},
		{ID: 405, Name: ".gitignore", Type: "text", ParentID: &buildID},
	})

	tests := []struct {
		name     string
		pattern  string
		expected []string
	}{
		{
			name:     "match hidden files only",
			pattern:  ".*",
			expected: []string{"/build/.gitignore"},
		},
		{
			name:     "match log files",
			pattern:  "*.log",
			expected: []string{"/build/debug.log"},
		},
		{
			name:     "match exe files",
			pattern:  "*.exe",
			expected: []string{"/build/app.exe"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := cache.MatchGlob("/build", tt.pattern)
			sort.Strings(matches)
			sort.Strings(tt.expected)
			assert.Equal(t, tt.expected, matches)
		})
	}
}

// NOTE: Extended glob patterns like !(pattern), +(pattern), ?(pattern), @(pattern)
// are bash extglob features NOT supported by doublestar library.
// The doublestar library supports: *, **, ?, [], {}, character classes.
// If extended glob support is needed, consider implementing custom filtering
// or using a different glob library.

func TestMatchGlob_AlternativesWithBraces(t *testing.T) {
	// This tests the {a,b,c} syntax which IS supported by doublestar
	cache := api.NewFileCache()

	libID := int64(500)
	cache.Add(&api.FileEntry{ID: libID, Name: "lib", Type: "folder"}, "/lib")
	cache.AddChildren("/lib", []api.FileEntry{
		{ID: 501, Name: "a.txt", Type: "text", ParentID: &libID},
		{ID: 502, Name: "b.txt", Type: "text", ParentID: &libID},
		{ID: 503, Name: "c.txt", Type: "text", ParentID: &libID},
		{ID: 504, Name: "ab.txt", Type: "text", ParentID: &libID},
		{ID: 505, Name: "abc.txt", Type: "text", ParentID: &libID},
	})

	tests := []struct {
		name     string
		pattern  string
		expected []string
	}{
		{
			name:     "match a or b single chars",
			pattern:  "{a,b}.txt",
			expected: []string{"/lib/a.txt", "/lib/b.txt"},
		},
		{
			name:     "match with wildcard in braces",
			pattern:  "{a,ab}*.txt",
			expected: []string{"/lib/a.txt", "/lib/ab.txt", "/lib/abc.txt"},
		},
		{
			name:     "match all single char names",
			pattern:  "?.txt",
			expected: []string{"/lib/a.txt", "/lib/b.txt", "/lib/c.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := cache.MatchGlob("/lib", tt.pattern)
			sort.Strings(matches)
			sort.Strings(tt.expected)
			assert.Equal(t, tt.expected, matches)
		})
	}
}

func TestMatchGlob_RootDirectory(t *testing.T) {
	cache := api.NewFileCache()

	// Test matching at root "/" which has special path handling
	cache.Add(&api.FileEntry{ID: 0, Name: "/", Type: "folder"}, "/")
	cache.AddChildren("/", []api.FileEntry{
		{ID: 1, Name: "Documents", Type: "folder"},
		{ID: 2, Name: "Downloads", Type: "folder"},
		{ID: 3, Name: "Desktop", Type: "folder"},
		{ID: 4, Name: "notes.txt", Type: "text"},
	})

	tests := []struct {
		name     string
		pattern  string
		expected []string
	}{
		{
			name:     "match folders starting with D",
			pattern:  "D*",
			expected: []string{"/Desktop", "/Documents", "/Downloads"},
		},
		{
			name:     "match all at root",
			pattern:  "*",
			expected: []string{"/Desktop", "/Documents", "/Downloads", "/notes.txt"},
		},
		{
			name:     "match txt files at root",
			pattern:  "*.txt",
			expected: []string{"/notes.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := cache.MatchGlob("/", tt.pattern)
			sort.Strings(matches)
			sort.Strings(tt.expected)
			assert.Equal(t, tt.expected, matches)
		})
	}
}

func TestMatchGlob_EmptyDirectory(t *testing.T) {
	cache := api.NewFileCache()

	emptyID := int64(600)
	cache.Add(&api.FileEntry{ID: emptyID, Name: "empty", Type: "folder"}, "/empty")
	cache.MarkChildrenLoaded("/empty") // Mark as loaded but with no children

	matches := cache.MatchGlob("/empty", "*")
	assert.Empty(t, matches, "Empty directory should return no matches")
}

func TestMatchGlob_NoDirectChildren(t *testing.T) {
	cache := api.NewFileCache()

	// Setup nested structure
	rootID := int64(0)
	parentID := int64(700)
	childID := int64(701)

	cache.Add(&api.FileEntry{ID: rootID, Name: "/", Type: "folder"}, "/")
	cache.Add(&api.FileEntry{ID: parentID, Name: "parent", Type: "folder"}, "/parent")
	cache.Add(&api.FileEntry{ID: childID, Name: "child", Type: "folder", ParentID: &parentID}, "/parent/child")
	cache.Add(&api.FileEntry{ID: 702, Name: "deep.txt", Type: "text", ParentID: &childID}, "/parent/child/deep.txt")
	cache.MarkChildrenLoaded("/parent")

	// Glob at /parent should NOT match /parent/child/deep.txt (only direct children)
	matches := cache.MatchGlob("/parent", "*.txt")
	assert.Empty(t, matches, "Should not match grandchildren")
}

func TestMatchGlob_SpecialCharacters(t *testing.T) {
	cache := api.NewFileCache()

	specialID := int64(800)
	cache.Add(&api.FileEntry{ID: specialID, Name: "special", Type: "folder"}, "/special")
	cache.AddChildren("/special", []api.FileEntry{
		{ID: 801, Name: "file (1).txt", Type: "text", ParentID: &specialID},
		{ID: 802, Name: "file (2).txt", Type: "text", ParentID: &specialID},
		{ID: 803, Name: "report-2024.pdf", Type: "text", ParentID: &specialID},
		{ID: 804, Name: "my file.doc", Type: "text", ParentID: &specialID},
	})

	tests := []struct {
		name     string
		pattern  string
		expected []string
	}{
		{
			name:     "files with parentheses",
			pattern:  "file (*).txt",
			expected: []string{"/special/file (1).txt", "/special/file (2).txt"},
		},
		{
			name:     "files with spaces",
			pattern:  "* *.doc",
			expected: []string{"/special/my file.doc"},
		},
		{
			name:     "files with dash",
			pattern:  "*-*.pdf",
			expected: []string{"/special/report-2024.pdf"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := cache.MatchGlob("/special", tt.pattern)
			sort.Strings(matches)
			sort.Strings(tt.expected)
			assert.Equal(t, tt.expected, matches)
		})
	}
}

func TestMatchGlob_CaseSensitivity(t *testing.T) {
	cache := api.NewFileCache()

	caseID := int64(900)
	cache.Add(&api.FileEntry{ID: caseID, Name: "mixed", Type: "folder"}, "/mixed")
	cache.AddChildren("/mixed", []api.FileEntry{
		{ID: 901, Name: "README.md", Type: "text", ParentID: &caseID},
		{ID: 902, Name: "readme.txt", Type: "text", ParentID: &caseID},
		{ID: 903, Name: "ReadMe.rst", Type: "text", ParentID: &caseID},
		{ID: 904, Name: "NOTES.TXT", Type: "text", ParentID: &caseID},
	})

	tests := []struct {
		name     string
		pattern  string
		expected []string
	}{
		{
			name:     "uppercase pattern",
			pattern:  "README*",
			expected: []string{"/mixed/README.md"},
		},
		{
			name:     "lowercase pattern",
			pattern:  "readme*",
			expected: []string{"/mixed/readme.txt"},
		},
		{
			name:     "using character class for case",
			pattern:  "[Rr][Ee][Aa][Dd][Mm][Ee]*",
			expected: []string{"/mixed/README.md", "/mixed/ReadMe.rst", "/mixed/readme.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := cache.MatchGlob("/mixed", tt.pattern)
			sort.Strings(matches)
			sort.Strings(tt.expected)
			assert.Equal(t, tt.expected, matches)
		})
	}
}

// ============================================================================
// EDGE CASE TESTS
// ============================================================================

func TestMatchGlob_InvalidPattern(t *testing.T) {
	cache := api.NewFileCache()

	testID := int64(1000)
	cache.Add(&api.FileEntry{ID: testID, Name: "test", Type: "folder"}, "/test")
	cache.AddChildren("/test", []api.FileEntry{
		{ID: 1001, Name: "file.txt", Type: "text", ParentID: &testID},
	})

	// Invalid patterns should not panic, just return empty or no matches
	// doublestar.Match returns error for invalid patterns, but MatchGlob ignores it

	invalidPatterns := []string{
		"[",    // unclosed bracket
		"[a-",  // unclosed range
		"{",    // unclosed brace
		"{a,b", // unclosed brace
	}

	for _, pattern := range invalidPatterns {
		t.Run("invalid_"+pattern, func(t *testing.T) {
			// Should not panic
			matches := cache.MatchGlob("/test", pattern)
			// Result doesn't matter as long as it doesn't crash
			_ = matches
		})
	}
}

func TestMatchGlob_EmptyPattern(t *testing.T) {
	cache := api.NewFileCache()

	emptyPatternID := int64(1100)
	cache.Add(&api.FileEntry{ID: emptyPatternID, Name: "empty_pattern", Type: "folder"}, "/empty_pattern")
	cache.AddChildren("/empty_pattern", []api.FileEntry{
		{ID: 1101, Name: "file.txt", Type: "text", ParentID: &emptyPatternID},
	})

	// Empty pattern should match nothing
	matches := cache.MatchGlob("/empty_pattern", "")
	assert.Empty(t, matches, "Empty pattern should match nothing")
}

func TestMatchGlob_DirectoryNotLoaded(t *testing.T) {
	cache := api.NewFileCache()

	// Directory exists but children not loaded
	notLoadedID := int64(1200)
	cache.Add(&api.FileEntry{ID: notLoadedID, Name: "not_loaded", Type: "folder"}, "/not_loaded")
	// Note: NOT calling AddChildren or MarkChildrenLoaded

	// Should return empty as no children are cached
	matches := cache.MatchGlob("/not_loaded", "*.txt")
	assert.Empty(t, matches, "Unloaded directory should return no matches")
}

func TestMatchGlob_NonExistentDirectory(t *testing.T) {
	cache := api.NewFileCache()

	// Parent directory doesn't exist in cache at all
	matches := cache.MatchGlob("/does_not_exist", "*.txt")
	assert.Empty(t, matches, "Non-existent directory should return no matches")
}
