package bomber

import (
	"strconv"
	"time"

	"termcade/internal/games/multi"
	"termcade/internal/ui/canvas"
	"termcade/internal/ui/theme"
)

// View implements multi.World.
func (w *World) View(seat int, th *theme.Theme) string {
	c := canvas.New(Cols*tile, Rows*tile)
	for y := 0; y < Rows; y++ {
		for x := 0; x < Cols; x++ {
			ox, oy := x*tile, y*tile
			switch w.grid[y][x] {
			case wall:
				c.Fill(ox, oy, tile, tile, "#5D6275")
				c.Fill(ox, oy, tile, 1, "#8B91A8")
				c.Fill(ox, oy+tile-1, tile, 1, "#3E4152")
			case crate:
				c.Fill(ox, oy, tile, tile, "#A0703F")
				c.Fill(ox, oy+1, tile, 1, "#7A522B")
				c.Fill(ox+1, oy, 1, tile, "#7A522B")
			default:
				col := "#1D3326"
				if (x+y)%2 == 0 {
					col = "#213B2C"
				}
				c.Fill(ox, oy, tile, tile, col)
			}
		}
	}
	for p, pw := range w.powers {
		ox, oy := p.x*tile, p.y*tile
		bg, fg := "#2F5FE0", "#FFFFFF"
		switch pw {
		case fire:
			bg, fg = "#E0602F", "#FFE066"
		case speed:
			bg, fg = "#2FB0C0", "#FFFFFF"
		}
		c.Fill(ox, oy, tile, tile, bg)
		c.Fill(ox+1, oy+1, 2, 2, fg)
	}
	for _, b := range w.bombs {
		ox, oy := b.pos.x*tile, b.pos.y*tile
		body := "#1A1A22"
		left := b.at - w.clock
		if left < 600*time.Millisecond && (w.clock/(100*time.Millisecond))%2 == 0 {
			body = "#C0303F"
		}
		c.Fill(ox+1, oy, 2, 1, body)
		c.Fill(ox, oy+1, tile, 2, body)
		c.Fill(ox+1, oy+3, 2, 1, body)
		c.Set(ox+1, oy+1, "#8A8AA0")
		if (w.clock/(150*time.Millisecond))%2 == 0 {
			c.Set(ox+3, oy, "#FFB020")
		}
	}
	for _, f := range w.flames {
		ox, oy := f.pos.x*tile, f.pos.y*tile
		c.Fill(ox, oy, tile, tile, "#FF7A3F")
		c.Fill(ox+1, oy+1, 2, 2, "#FFE066")
	}
	order := make([]int, 0, len(w.players))
	for i := range w.players {
		if i != seat {
			order = append(order, i)
		}
	}
	order = append(order, seat)
	for _, i := range order {
		p := w.players[i]
		if !p.alive {
			continue
		}
		ox, oy := p.pos.x*tile, p.pos.y*tile
		head := theme.Mix(p.info.Color, "#FFFFFF", 0.6)
		if i == seat {
			head = "#FFFFFF"
		}
		c.Fill(ox+1, oy, 2, 1, head)
		c.Fill(ox, oy+1, tile, 2, p.info.Color)
		c.Set(ox, oy+3, theme.Mix(p.info.Color, "#000000", 0.4))
		c.Set(ox+3, oy+3, theme.Mix(p.info.Color, "#000000", 0.4))
	}
	if !w.players[seat].alive {
		c.TextCenter(0, " blown up, watching ", "#FFFFFF", theme.Coral)
	}

	rows := make([]multi.SidebarRow, 0, len(w.players))
	for _, st := range w.Standings() {
		p := w.players[st.Seat]
		v := strconv.Itoa(p.kills) + "✦"
		rows = append(rows, multi.SidebarRow{Name: p.info.Name, Color: p.info.Color, Value: v, You: st.Seat == seat, Out: !p.alive})
	}
	secs := int((roundLength - w.clock + time.Second - 1) / time.Second)
	return multi.Arena(th, c, "TIME LEFT", multi.Clock(secs), rows, "* is you", "✦ = kills", "blue: +bomb", "red: +fire", "cyan: +speed")
}
