package ui

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
)

// SyntaxTheme returns the appropriate chroma style based on terminal background
func SyntaxTheme() string {
	if lipgloss.HasDarkBackground() {
		return "dracula"
	}
	return "github"
}

// Highlight returns syntax-highlighted content based on filename extension.
// If highlighting fails or no lexer is found, returns the original content.
func Highlight(content, filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		// Try to detect from filename itself (e.g., "Makefile", "Dockerfile")
		ext = filepath.Base(filename)
	}

	// Get lexer by extension or filename
	lexer := lexers.Match(filename)
	if lexer == nil {
		lexer = lexers.Get(ext)
	}
	if lexer == nil {
		// Try to analyze content
		//nolint:misspell // Library uses British spelling
		lexer = lexers.Analyse(content)
	}
	if lexer == nil {
		// No highlighting available
		return content
	}

	// Coalesce runs of same tokens for better output
	lexer = chroma.Coalesce(lexer)

	// Get style
	style := styles.Get(SyntaxTheme())
	if style == nil {
		style = styles.Fallback
	}

	// Use terminal256 formatter for wide compatibility
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	// Tokenize and format
	iterator, err := lexer.Tokenise(nil, content)
	if err != nil {
		return content
	}

	buf := new(bytes.Buffer)
	if err := formatter.Format(buf, style, iterator); err != nil {
		return content
	}

	return buf.String()
}

// HighlightTo writes syntax-highlighted content to the given writer.
// Returns bytes written and any error.
func HighlightTo(w io.Writer, content, filename string) (int, error) {
	highlighted := Highlight(content, filename)
	return io.WriteString(w, highlighted)
}

// HighlightLines returns syntax-highlighted content with line numbers.
func HighlightLines(content, filename string) string {
	highlighted := Highlight(content, filename)
	if highlighted == content {
		// No highlighting, add line numbers manually
		return addLineNumbers(content)
	}

	// For highlighted content, we need to be careful with ANSI codes
	// Just return highlighted without line numbers for now
	// (line numbers with ANSI codes is complex)
	return highlighted
}

// addLineNumbers adds line numbers to plain text
func addLineNumbers(content string) string {
	lines := strings.Split(content, "\n")
	var buf strings.Builder
	width := len(fmt.Sprintf("%d", len(lines)))
	format := fmt.Sprintf("%%%dd │ ", width)

	for i, line := range lines {
		buf.WriteString(MutedStyle.Render(fmt.Sprintf(format, i+1)))
		buf.WriteString(line)
		if i < len(lines)-1 {
			buf.WriteString("\n")
		}
	}
	return buf.String()
}

// SupportedExtensions returns a list of file extensions that have syntax highlighting support
func SupportedExtensions() []string {
	return []string{
		".go", ".py", ".js", ".ts", ".jsx", ".tsx",
		".java", ".c", ".cpp", ".h", ".hpp", ".cs",
		".rb", ".rs", ".swift", ".kt", ".scala",
		".php", ".pl", ".lua", ".r", ".m",
		".html", ".css", ".scss", ".less",
		".json", ".yaml", ".yml", ".xml", ".toml",
		".md", ".sql", ".sh", ".bash", ".zsh",
		".dockerfile", ".makefile",
	}
}
