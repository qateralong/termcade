// Package textutil holds helpers for cleaning up untrusted user text before it
// is stored or rendered into other players' terminals.
package textutil

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// Name constraints for player nicknames.
const (
	NameMinLen = 3
	NameMaxLen = 16
)

var (
	ErrNameTooShort = errors.New("name must be at least 3 characters")
	ErrNameTooLong  = errors.New("name must be at most 16 characters")
	ErrNameChars    = errors.New("use letters, digits, - and _ only")
	ErrNameStart    = errors.New("name must start with a letter")
	ErrNameReserved = errors.New("that name is reserved")
)

var reservedNames = map[string]bool{
	"admin": true, "administrator": true, "root": true, "system": true,
	"server": true, "mod": true, "moderator": true, "termcade": true,
	"guest": true, "anonymous": true, "nobody": true, "you": true,
}

// ValidateName reports whether name is an acceptable nickname. Names are
// deliberately restricted to ASCII so they render identically everywhere and
// cannot be used to impersonate others with look-alike characters.
func ValidateName(name string) error {
	n := len(name)
	switch {
	case n < NameMinLen:
		return ErrNameTooShort
	case n > NameMaxLen:
		return ErrNameTooLong
	}
	for i, r := range name {
		isLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if i == 0 && !isLetter {
			return ErrNameStart
		}
		if !isLetter && !(r >= '0' && r <= '9') && r != '-' && r != '_' {
			return ErrNameChars
		}
	}
	if reservedNames[strings.ToLower(name)] || strings.HasPrefix(strings.ToLower(name), "guest") {
		return ErrNameReserved
	}
	return nil
}

// SuggestName turns an arbitrary string (such as the SSH username) into a
// valid nickname, or returns "" if nothing usable is left.
func SuggestName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_':
			if b.Len() == 0 && !unicode.IsLetter(r) {
				continue
			}
			b.WriteRune(r)
		}
		if b.Len() == NameMaxLen {
			break
		}
	}
	name := b.String()
	if ValidateName(name) != nil {
		return ""
	}
	return name
}

// SanitizeLine strips escape sequences and control characters, collapses
// whitespace, and truncates the result to at most maxRunes runes. It is safe
// to render the result in another user's terminal.
func SanitizeLine(s string, maxRunes int) string {
	s = ansi.Strip(s)
	var b strings.Builder
	space := false
	count := 0
	for _, r := range s {
		if r == utf8.RuneError {
			continue
		}
		if unicode.IsSpace(r) {
			space = b.Len() > 0
			continue
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			// Cf covers bidi overrides and zero-width characters.
			continue
		}
		if space {
			if count+1 >= maxRunes {
				break
			}
			b.WriteByte(' ')
			count++
			space = false
		}
		if count >= maxRunes {
			break
		}
		b.WriteRune(r)
		count++
	}
	return b.String()
}
