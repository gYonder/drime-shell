package ui

import "fmt"

// FormatSize returns a human-readable size string
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// RenderLink formats a URL with the link style
func RenderLink(url string) string {
	return LinkStyle.Render(url)
}
