package session_test

import (
	"testing"

	"github.com/gYonder/drime-shell/internal/api"
	"github.com/gYonder/drime-shell/internal/session"
	"github.com/stretchr/testify/assert"
)

func TestSession_ResolvePath(t *testing.T) {
	s := &session.Session{
		CWD:         "/users/mikael",
		HomeDir:     "/",
		PreviousDir: "/users",
		Cache:       api.NewFileCache(),
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"/", "/"},
		{".", "/users/mikael"},
		{"..", "/users"},
		{"../..", "/"},
		{"docs", "/users/mikael/docs"},
		{"./docs", "/users/mikael/docs"},
		{"/absolute/path", "/absolute/path"},
		{"~", "/"},
		{"~/docs", "/docs"},
		{"-", "/users"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := s.ResolvePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
