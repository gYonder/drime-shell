package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/mattn/go-runewidth"
)

// Table is a simple ANSI-aware table printer
type Table struct {
	writer  io.Writer
	headers []string
	rows    [][]string
	padding int
}

// NewTable creates a new table writing to w
func NewTable(w io.Writer) *Table {
	return &Table{
		writer:  w,
		padding: 2,
	}
}

// SetHeaders sets the table headers
func (t *Table) SetHeaders(headers ...string) {
	t.headers = headers
}

// AddRow adds a row to the table
func (t *Table) AddRow(cols ...string) {
	t.rows = append(t.rows, cols)
}

// Render prints the table
func (t *Table) Render() {
	if len(t.headers) == 0 && len(t.rows) == 0 {
		return
	}

	// Calculate column widths
	numCols := len(t.headers)
	for _, row := range t.rows {
		if len(row) > numCols {
			numCols = len(row)
		}
	}

	colWidths := make([]int, numCols)

	// Check headers
	for i, h := range t.headers {
		w := VisibleLen(h)
		if w > colWidths[i] {
			colWidths[i] = w
		}
	}

	// Check rows
	for _, row := range t.rows {
		for i, col := range row {
			w := VisibleLen(col)
			if w > colWidths[i] {
				colWidths[i] = w
			}
		}
	}

	// Print headers
	if len(t.headers) > 0 {
		t.printRow(t.headers, colWidths)
	}

	// Print rows
	for _, row := range t.rows {
		t.printRow(row, colWidths)
	}
}

func (t *Table) printRow(row []string, widths []int) {
	for i, col := range row {
		// Calculate padding
		vLen := VisibleLen(col)
		pad := widths[i] - vLen

		fmt.Fprint(t.writer, col)

		// Add padding if not last column
		if i < len(widths)-1 {
			fmt.Fprint(t.writer, strings.Repeat(" ", pad+t.padding))
		}
	}
	fmt.Fprintln(t.writer)
}

// StripANSI removes ANSI escape codes from a string
func StripANSI(s string) string {
	var result strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}

// VisibleLen returns the visible length of a string (excluding ANSI codes)
func VisibleLen(s string) int {
	return runewidth.StringWidth(StripANSI(s))
}
