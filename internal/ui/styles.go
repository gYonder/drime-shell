package ui

import "github.com/charmbracelet/lipgloss"

// Catppuccin Mocha (dark theme)
var mocha = struct {
	Rosewater, Flamingo, Pink, Mauve, Red, Maroon, Peach, Yellow, Green, Teal, Sky, Sapphire, Blue, Lavender  lipgloss.Color
	Text, Subtext1, Subtext0, Overlay2, Overlay1, Overlay0, Surface2, Surface1, Surface0, Base, Mantle, Crust lipgloss.Color
}{
	Rosewater: "#f5e0dc", Flamingo: "#f2cdcd", Pink: "#f5c2e7", Mauve: "#cba6f7",
	Red: "#f38ba8", Maroon: "#eba0ac", Peach: "#fab387", Yellow: "#f9e2af",
	Green: "#a6e3a1", Teal: "#94e2d5", Sky: "#89dceb", Sapphire: "#74c7ec",
	Blue: "#89b4fa", Lavender: "#b4befe",
	Text: "#cdd6f4", Subtext1: "#bac2de", Subtext0: "#a6adc8",
	Overlay2: "#9399b2", Overlay1: "#7f849c", Overlay0: "#6c7086",
	Surface2: "#585b70", Surface1: "#45475a", Surface0: "#313244",
	Base: "#1e1e2e", Mantle: "#181825", Crust: "#11111b",
}

// Catppuccin Latte (light theme)
var latte = struct {
	Rosewater, Flamingo, Pink, Mauve, Red, Maroon, Peach, Yellow, Green, Teal, Sky, Sapphire, Blue, Lavender  lipgloss.Color
	Text, Subtext1, Subtext0, Overlay2, Overlay1, Overlay0, Surface2, Surface1, Surface0, Base, Mantle, Crust lipgloss.Color
}{
	Rosewater: "#dc8a78", Flamingo: "#dd7878", Pink: "#ea76cb", Mauve: "#8839ef",
	Red: "#d20f39", Maroon: "#e64553", Peach: "#fe640b", Yellow: "#df8e1d",
	Green: "#40a02b", Teal: "#179299", Sky: "#04a5e5", Sapphire: "#209fb5",
	Blue: "#1e66f5", Lavender: "#7287fd",
	Text: "#4c4f69", Subtext1: "#5c5f77", Subtext0: "#6c6f85",
	Overlay2: "#7c7f93", Overlay1: "#8c8fa1", Overlay0: "#9ca0b0",
	Surface2: "#acb0be", Surface1: "#bcc0cc", Surface0: "#ccd0da",
	Base: "#eff1f5", Mantle: "#e6e9ef", Crust: "#dce0e8",
}

// ThemePalette holds the current color scheme
type ThemePalette struct {
	Red, Green, Yellow, Blue, Magenta, Cyan, Peach, Mauve lipgloss.Color
	Text, Subtext, Overlay, Surface, Base                 lipgloss.Color
}

var currentTheme ThemePalette

func init() {
	if DetectTheme() == ThemeDark {
		SetDarkTheme()
	} else {
		SetLightTheme()
	}
}

// SetDarkTheme switches to Catppuccin Mocha
func SetDarkTheme() {
	currentTheme = ThemePalette{
		Red: mocha.Red, Green: mocha.Green, Yellow: mocha.Yellow,
		Blue: mocha.Blue, Magenta: mocha.Pink, Cyan: mocha.Teal, Peach: mocha.Peach, Mauve: mocha.Mauve,
		Text: mocha.Text, Subtext: mocha.Subtext1, Overlay: mocha.Overlay1, Surface: mocha.Surface1,
		Base: mocha.Base,
	}
	refreshStyles()
}

// SetLightTheme switches to Catppuccin Latte
func SetLightTheme() {
	currentTheme = ThemePalette{
		Red: latte.Red, Green: latte.Green, Yellow: latte.Yellow,
		Blue: latte.Blue, Magenta: latte.Pink, Cyan: latte.Teal, Peach: latte.Peach, Mauve: latte.Mauve,
		Text: latte.Text, Subtext: latte.Subtext1, Overlay: latte.Overlay1, Surface: latte.Surface1,
		Base: latte.Base,
	}
	refreshStyles()
}

// Semantic styles for the shell
var (
	DirStyle        lipgloss.Style
	FileStyle       lipgloss.Style
	ExecStyle       lipgloss.Style
	ImageStyle      lipgloss.Style
	ArchiveStyle    lipgloss.Style
	VideoStyle      lipgloss.Style
	AudioStyle      lipgloss.Style
	DocStyle        lipgloss.Style
	PermStyle       lipgloss.Style
	SizeStyle       lipgloss.Style
	OwnerStyle      lipgloss.Style
	DateStyle       lipgloss.Style
	MutedStyle      lipgloss.Style
	ErrorStyle      lipgloss.Style
	WarningStyle    lipgloss.Style
	SuccessStyle    lipgloss.Style
	PromptUserStyle lipgloss.Style
	PromptPathStyle lipgloss.Style
	CommandStyle    lipgloss.Style
	HeaderStyle     lipgloss.Style
	StarStyle       lipgloss.Style // For starred files indicator
	TrashStyle      lipgloss.Style // For trash indicator
	WorkspaceStyle  lipgloss.Style // For workspace name in prompt
	LinkStyle       lipgloss.Style // For URLs
)

func refreshStyles() {
	// Directory names (blue, bold)
	DirStyle = lipgloss.NewStyle().Foreground(currentTheme.Blue).Bold(true)

	// Regular files (default text)
	FileStyle = lipgloss.NewStyle().Foreground(currentTheme.Text)

	// Executable files (green)
	ExecStyle = lipgloss.NewStyle().Foreground(currentTheme.Green)

	// Images (magenta/pink)
	ImageStyle = lipgloss.NewStyle().Foreground(currentTheme.Magenta)

	// Archives (peach/orange)
	ArchiveStyle = lipgloss.NewStyle().Foreground(currentTheme.Peach)

	// Videos (magenta, bold)
	VideoStyle = lipgloss.NewStyle().Foreground(currentTheme.Magenta).Bold(true)

	// Audio (cyan/teal)
	AudioStyle = lipgloss.NewStyle().Foreground(currentTheme.Cyan)

	// Documents/PDF (yellow)
	DocStyle = lipgloss.NewStyle().Foreground(currentTheme.Yellow)

	// Permission string (subtext/muted)
	PermStyle = lipgloss.NewStyle().Foreground(currentTheme.Subtext)

	// Size column (green)
	SizeStyle = lipgloss.NewStyle().Foreground(currentTheme.Green)

	// Owner column (yellow)
	OwnerStyle = lipgloss.NewStyle().Foreground(currentTheme.Yellow)

	// Date column (subtext/muted)
	DateStyle = lipgloss.NewStyle().Foreground(currentTheme.Subtext)

	// Muted/secondary text (overlay)
	MutedStyle = lipgloss.NewStyle().Foreground(currentTheme.Overlay)

	// Error text (red, bold)
	ErrorStyle = lipgloss.NewStyle().Foreground(currentTheme.Red).Bold(true)

	// Warning text (peach)
	WarningStyle = lipgloss.NewStyle().Foreground(currentTheme.Peach)

	// Success text (green)
	SuccessStyle = lipgloss.NewStyle().Foreground(currentTheme.Green)

	// Prompt username (cyan)
	PromptUserStyle = lipgloss.NewStyle().Foreground(currentTheme.Cyan)

	// Prompt path (blue, bold)
	PromptPathStyle = lipgloss.NewStyle().Foreground(currentTheme.Blue).Bold(true)

	// Command names (green, bold)
	CommandStyle = lipgloss.NewStyle().Foreground(currentTheme.Green).Bold(true)

	// Header text (magenta, bold)
	HeaderStyle = lipgloss.NewStyle().Foreground(currentTheme.Magenta).Bold(true)

	// Starred files indicator (yellow)
	StarStyle = lipgloss.NewStyle().Foreground(currentTheme.Yellow)

	// Trash indicator (red)
	TrashStyle = lipgloss.NewStyle().Foreground(currentTheme.Red)

	// Workspace name in prompt (magenta)
	WorkspaceStyle = lipgloss.NewStyle().Foreground(currentTheme.Magenta)

	// Links
	LinkStyle = lipgloss.NewStyle().Foreground(currentTheme.Blue).Underline(true)
}

// StyleForType returns the appropriate style for a file type
func StyleForType(fileType string) lipgloss.Style {
	switch fileType {
	case "folder":
		return DirStyle
	case "image":
		return ImageStyle
	case "video":
		return VideoStyle
	case "audio":
		return AudioStyle
	case "pdf", "document", "text":
		return DocStyle
	case "archive":
		return ArchiveStyle
	default:
		return FileStyle
	}
}

// StyleName applies the appropriate style to a filename based on type
func StyleName(name string, fileType string) string {
	style := StyleForType(fileType)
	return style.Render(name)
}
