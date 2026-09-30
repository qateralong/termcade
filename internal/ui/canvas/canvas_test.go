package canvas

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestRender(t *testing.T) {
	c := New(4, 4)
	c.Set(0, 0, "#ff0000") // top only
	c.Set(1, 1, "#00ff00") // bottom only
	c.Set(2, 0, "#0000ff") // both, same color
	c.Set(2, 1, "#0000ff")
	c.Set(3, 0, "#ff0000") // both, different
	c.Set(3, 1, "#00ff00")
	c.Text(1, 1, "hi", "#ffffff", "")

	out := c.Render(termenv.ANSI256)
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines", len(lines))
	}
	if got := ansi.Strip(lines[0]); got != "▀▄█▀" {
		t.Fatalf("row 0 = %q", got)
	}
	if got := ansi.Strip(lines[1]); got != " hi " {
		t.Fatalf("row 1 = %q", got)
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w != 4 {
			t.Fatalf("row %d is %d wide", i, w)
		}
	}
}

func TestRenderAsciiProfileHasNoColors(t *testing.T) {
	c := New(2, 2)
	c.Fill(0, 0, 2, 2, "#123456")
	if out := c.Render(termenv.Ascii); strings.Contains(out, "38;") {
		t.Fatalf("ascii profile emitted colors: %q", out)
	}
}
