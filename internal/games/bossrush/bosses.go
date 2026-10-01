package bossrush

import "termcade/internal/ui/theme"

// boss describes one opponent in the rush.
type boss struct {
	name    string
	hp      int
	damage  int // HP lost per hit
	speed   float64
	color   string
	art     [4]string
	attacks []attackKind
	lines   []string // flavor text between turns
}

var bosses = []boss{
	{
		name: "Bone Jester", hp: 60, damage: 2, speed: 1.0, color: "#E8E8F0",
		art: [4]string{
			"   ▄█████▄   ",
			"  █  ◉ ◉  █  ",
			"  █ ▀▀▀▀▀ █  ",
			"   ▀█▀█▀█▀   ",
		},
		attacks: []attackKind{boneWall, boneHop, aimed},
		lines: []string{
			"The Bone Jester grins at you.",
			"The Bone Jester is having a great time.",
			"You feel like you're gonna have a bad time.",
			"The Bone Jester winks. It doesn't help.",
		},
	},
	{
		name: "Ember Wisp", hp: 80, damage: 3, speed: 1.1, color: "#FF9F43",
		art: [4]string{
			"    ▲  ▲  ▲    ",
			"   ▟███████▙   ",
			"   █  ● ●  █   ",
			"    ▀▀▀▀▀▀▀    ",
		},
		attacks: []attackKind{fireRain, spiral, boneWall},
		lines: []string{
			"The Ember Wisp crackles.",
			"It smells like smoke.",
			"The Ember Wisp burns brighter.",
		},
	},
	{
		name: "Iron Warden", hp: 100, damage: 3, speed: 1.15, color: "#9AA6C0",
		art: [4]string{
			"  ▐█████████▌  ",
			"  █  ■   ■  █  ",
			"  █▄▄▄▄▄▄▄▄▄█  ",
			"  ▀▀▀  ▀  ▀▀▀  ",
		},
		attacks: []attackKind{laser, ring, boneHop},
		lines: []string{
			"The Iron Warden stands guard.",
			"Gears grind somewhere inside it.",
			"The Iron Warden recalibrates.",
		},
	},
	{
		name: "Storm Eye", hp: 120, damage: 4, speed: 1.25, color: "#5AA9FF",
		art: [4]string{
			"   ╲    │    ╱   ",
			"   ── ( ◉ ) ──   ",
			"   ╱    │    ╲   ",
			"                 ",
		},
		attacks: []attackKind{aimed, spiral, laser, fireRain},
		lines: []string{
			"The Storm Eye watches your every move.",
			"Thunder rolls.",
			"The air tastes like static.",
		},
	},
	{
		name: "Void Monarch", hp: 150, damage: 4, speed: 1.4, color: theme.Violet,
		art: [4]string{
			"  ▲  ▲ ▲▲▲ ▲  ▲  ",
			"  █▀▀▀◉▀▀▀◉▀▀▀█  ",
			"  █   ▀▀▀▀▀   █  ",
			"  ▀█▄█▄▄▄▄▄█▄█▀  ",
		},
		attacks: []attackKind{boneWall, boneHop, fireRain, spiral, laser, ring, aimed},
		lines: []string{
			"The Void Monarch looks down on you.",
			"Reality flickers at the edges.",
			"This is it. Stay determined.",
		},
	},
}
