package theme

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// Blend returns the color at position t (wrapping into [0, 1)) along a
// cyclic gradient through stops.
func Blend(stops []string, t float64) string {
	if len(stops) == 0 {
		return Pink
	}
	if len(stops) == 1 {
		return stops[0]
	}
	t -= math.Floor(t)
	pos := t * float64(len(stops))
	i := int(pos) % len(stops)
	j := (i + 1) % len(stops)
	a, errA := colorful.Hex(stops[i])
	b, errB := colorful.Hex(stops[j])
	if errA != nil || errB != nil {
		return stops[i]
	}
	return a.BlendLuv(b, pos-math.Floor(pos)).Clamped().Hex()
}

// Gradient colors each line of text with a horizontal cyclic gradient.
// span is the width, in cells, that one full cycle of the gradient covers,
// and phase shifts the gradient (animate it to make the colors flow).
// Spaces are left unstyled to keep the output compact.
func (t *Theme) Gradient(text string, stops []string, span int, phase float64, bold bool) string {
	if span <= 0 {
		span = 1
	}
	lines := strings.Split(text, "\n")
	var out strings.Builder
	for li, line := range lines {
		if li > 0 {
			out.WriteByte('\n')
		}
		// Group consecutive runes of the same (quantized) color so we emit
		// far fewer escape sequences.
		var run strings.Builder
		runColor := ""
		flush := func() {
			if run.Len() == 0 {
				return
			}
			st := t.R.NewStyle().Foreground(lipgloss.Color(runColor)).Bold(bold)
			out.WriteString(st.Render(run.String()))
			run.Reset()
		}
		col := 0
		for _, r := range line {
			if r == ' ' {
				flush()
				out.WriteRune(r)
				col++
				continue
			}
			c := Blend(stops, float64(col)/float64(span)+phase)
			if c != runColor {
				flush()
				runColor = c
			}
			run.WriteRune(r)
			col++
		}
		flush()
	}
	return out.String()
}
