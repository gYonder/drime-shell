package shell

import (
	"strings"
)

// ExpandAlias checks if the first word of the command is an alias and expands it.
// Returns the expanded command line and whether expansion occurred.
func ExpandAlias(line string, aliases map[string]string) (string, bool) {
	if len(aliases) == 0 {
		return line, false
	}

	line = strings.TrimSpace(line)
	if line == "" {
		return line, false
	}

	// Find the first word (command name)
	parts := strings.SplitN(line, " ", 2)
	cmdName := parts[0]

	// Check if it's an alias
	expansion, ok := aliases[cmdName]
	if !ok {
		return line, false
	}

	// Build expanded command
	if len(parts) > 1 {
		// Append original arguments
		return expansion + " " + parts[1], true
	}
	return expansion, true
}
