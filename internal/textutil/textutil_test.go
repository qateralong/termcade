package textutil

import "testing"

func TestValidateName(t *testing.T) {
	tests := []struct {
		name string
		want error
	}{
		{"qatera", nil},
		{"Neo_42", nil},
		{"a-b", nil},
		{"ab", ErrNameTooShort},
		{"abcdefghijklmnopq", ErrNameTooLong},
		{"1abc", ErrNameStart},
		{"_abc", ErrNameStart},
		{"hey there", ErrNameChars},
		{"иван", ErrNameStart},
		{"bob!", ErrNameChars},
		{"Admin", ErrNameReserved},
		{"guest123", ErrNameReserved},
	}
	for _, tt := range tests {
		if got := ValidateName(tt.name); got != tt.want {
			t.Errorf("ValidateName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestSuggestName(t *testing.T) {
	tests := map[string]string{
		"qatera":                "qatera",
		"123bob":                "bob",
		"john.doe":              "johndoe",
		"root":                  "",
		"a":                     "",
		"averyveryverylongname": "averyveryverylon",
		"иван":                  "",
	}
	for in, want := range tests {
		if got := SuggestName(in); got != want {
			t.Errorf("SuggestName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitizeLine(t *testing.T) {
	tests := []struct {
		in   string
		max  int
		want string
	}{
		{"hello", 100, "hello"},
		{"  lots   of \t space  ", 100, "lots of space"},
		{"\x1b[31mred\x1b[0m text", 100, "red text"},
		{"bell\a and\x00 nul", 100, "bell and nul"},
		{"rtl‮override", 100, "rtloverride"},
		{"привет мир", 100, "привет мир"},
		{"abcdef", 3, "abc"},
		{"ab cd", 3, "ab"},
	}
	for _, tt := range tests {
		if got := SanitizeLine(tt.in, tt.max); got != tt.want {
			t.Errorf("SanitizeLine(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
		}
	}
}
