// Package theme defines Termcade's colors and shared styles. Styles are built
// from a per-session lipgloss renderer so every SSH client gets output that
// matches its own terminal's color support.
package theme

import (
	"github.com/charmbracelet/lipgloss"
)

// Neon arcade palette.
const (
	Pink   = "#FF4FD8"
	Cyan   = "#3DE8FF"
	Violet = "#9D7BFF"
	Lime   = "#B8FF5A"
	Amber  = "#FFC940"
	Coral  = "#FF7A6B"
	Sky    = "#5AA9FF"
	Mint   = "#4DFFC3"
)

// PlayerColor is a color a player can pick for their name.
type PlayerColor struct {
	Key string
	Hex string
}

// PlayerColors lists the colors players can choose from, in display order.
var PlayerColors = []PlayerColor{
	{"pink", Pink},
	{"cyan", Cyan},
	{"violet", Violet},
	{"lime", Lime},
	{"amber", Amber},
	{"coral", Coral},
	{"sky", Sky},
	{"mint", Mint},
}

// DefaultPlayerColor is used when a stored color is unknown.
const DefaultPlayerColor = "pink"

// PlayerHex returns the hex value of a player color key.
func PlayerHex(key string) string {
	for _, c := range PlayerColors {
		if c.Key == key {
			return c.Hex
		}
	}
	return Pink
}

// ValidPlayerColor reports whether key is one of PlayerColors.
func ValidPlayerColor(key string) bool {
	for _, c := range PlayerColors {
		if c.Key == key {
			return true
		}
	}
	return false
}

// LogoGradient is the gradient used for the logo and other highlights.
var LogoGradient = []string{Pink, Violet, Cyan, Violet}

// Theme bundles the renderer with the styles built from it.
type Theme struct {
	R *lipgloss.Renderer

	Text, Muted, Faint, Border, Highlight, Accent lipgloss.TerminalColor

	Base      lipgloss.Style // plain text
	Dim       lipgloss.Style // secondary text
	Faded     lipgloss.Style // tertiary text: hints, timestamps
	Bold      lipgloss.Style
	Title     lipgloss.Style // section titles, e.g. panel headings
	Key       lipgloss.Style // key caps in help lines
	Error     lipgloss.Style
	Success   lipgloss.Style
	Selected  lipgloss.Style // selected list row
	Panel     lipgloss.Style // unfocused panel frame
	PanelOn   lipgloss.Style // focused panel frame
	Modal     lipgloss.Style
	Badge     lipgloss.Style
	StatusBar lipgloss.Style
}

// New builds a theme for renderer r.
func New(r *lipgloss.Renderer) *Theme {
	t := &Theme{
		R:         r,
		Text:      lipgloss.AdaptiveColor{Light: "#2A2438", Dark: "#E8E3F7"},
		Muted:     lipgloss.AdaptiveColor{Light: "#6E6685", Dark: "#9A93B5"},
		Faint:     lipgloss.AdaptiveColor{Light: "#9E97B3", Dark: "#5E5775"},
		Border:    lipgloss.AdaptiveColor{Light: "#CFC8E0", Dark: "#3B3452"},
		Highlight: lipgloss.AdaptiveColor{Light: "#EFE8FC", Dark: "#261E3A"},
		Accent:    lipgloss.Color(Pink),
	}
	s := r.NewStyle
	t.Base = s().Foreground(t.Text)
	t.Dim = s().Foreground(t.Muted)
	t.Faded = s().Foreground(t.Faint)
	t.Bold = s().Foreground(t.Text).Bold(true)
	t.Title = s().Foreground(lipgloss.Color(Pink)).Bold(true)
	t.Key = s().Foreground(lipgloss.Color(Cyan)).Bold(true)
	t.Error = s().Foreground(lipgloss.Color(Coral))
	t.Success = s().Foreground(lipgloss.Color(Lime))
	t.Selected = s().Background(t.Highlight).Foreground(t.Text).Bold(true)
	t.Panel = s().Border(lipgloss.RoundedBorder()).BorderForeground(t.Border)
	t.PanelOn = s().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(Violet))
	t.Modal = s().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(Pink)).Padding(1, 3)
	t.Badge = s().Padding(0, 1).Bold(true)
	t.StatusBar = s().Foreground(t.Muted)
	return t
}

// Fg returns a style with the given hex foreground.
func (t *Theme) Fg(hex string) lipgloss.Style {
	return t.R.NewStyle().Foreground(lipgloss.Color(hex))
}

// PlayerName renders a player name in their color.
func (t *Theme) PlayerName(name, colorKey string) string {
	return t.Fg(PlayerHex(colorKey)).Bold(true).Render(name)
}
