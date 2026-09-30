package theme

import "strings"

// Glyphs for the big logo, in the "ANSI Shadow" figlet style. Every glyph is
// six rows tall and each row of a glyph has the same width.
var glyphs = map[rune][6]string{
	'T': {
		"████████╗",
		"╚══██╔══╝",
		"   ██║   ",
		"   ██║   ",
		"   ██║   ",
		"   ╚═╝   ",
	},
	'E': {
		"███████╗",
		"██╔════╝",
		"█████╗  ",
		"██╔══╝  ",
		"███████╗",
		"╚══════╝",
	},
	'R': {
		"██████╗ ",
		"██╔══██╗",
		"██████╔╝",
		"██╔══██╗",
		"██║  ██║",
		"╚═╝  ╚═╝",
	},
	'M': {
		"███╗   ███╗",
		"████╗ ████║",
		"██╔████╔██║",
		"██║╚██╔╝██║",
		"██║ ╚═╝ ██║",
		"╚═╝     ╚═╝",
	},
	'C': {
		" ██████╗",
		"██╔════╝",
		"██║     ",
		"██║     ",
		"╚██████╗",
		" ╚═════╝",
	},
	'A': {
		" █████╗ ",
		"██╔══██╗",
		"███████║",
		"██╔══██║",
		"██║  ██║",
		"╚═╝  ╚═╝",
	},
	'D': {
		"██████╗ ",
		"██╔══██╗",
		"██║  ██║",
		"██║  ██║",
		"██████╔╝",
		"╚═════╝ ",
	},
}

// Name is the product name, used for the logo and in text.
const Name = "TERMCADE"

// Tagline is shown under the logo.
const Tagline = "a tiny arcade you play over ssh"

// BigLogo returns the six-line block logo for word. Unknown letters are
// skipped.
func BigLogo(word string) string {
	var rows [6]strings.Builder
	for _, r := range word {
		g, ok := glyphs[r]
		if !ok {
			continue
		}
		for i := range rows {
			rows[i].WriteString(g[i])
		}
	}
	lines := make([]string, len(rows))
	for i := range rows {
		lines[i] = rows[i].String()
	}
	return strings.Join(lines, "\n")
}

// SmallLogo is the compact logo used in the header bar.
func SmallLogo() string {
	return "▞▚ " + Name
}
