package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"termcade/internal/ui/theme"
	"termcade/internal/version"
)

// Console output helpers for the admin commands.

var th = theme.New(lipgloss.DefaultRenderer())

func header(title string) {
	fmt.Println()
	fmt.Println("  " + th.Gradient(theme.SmallLogo(), theme.LogoGradient, 12, 0, true) +
		th.Faded.Render(" "+version.String()+"  ·  ") + th.Bold.Render(title))
	fmt.Println()
}

func row(label, value string) {
	fmt.Println("  " + th.Dim.Render(fmt.Sprintf("%-9s", label)) + value)
}

func ok(format string, args ...any) {
	fmt.Println("  " + th.Success.Render("✓") + " " + fmt.Sprintf(format, args...))
}

func note(format string, args ...any) {
	fmt.Println("  " + th.Fg(theme.Amber).Render("◆") + " " + fmt.Sprintf(format, args...))
}

func hint(format string, args ...any) {
	fmt.Println("  " + th.Faded.Render(fmt.Sprintf(format, args...)))
}

func fail(err error) {
	msg := err.Error()
	fmt.Fprintln(os.Stderr, "\n  "+th.Error.Render("✗ ")+strings.ToUpper(msg[:1])+msg[1:]+"\n")
	os.Exit(1)
}

func highlight(s string) string { return th.Fg(theme.Cyan).Bold(true).Render(s) }

func code(s string) string { return th.Key.Render(s) }

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
