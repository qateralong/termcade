package version

import "testing"

func TestParse(t *testing.T) {
	good := map[string]Semver{
		"0.0.1":    {0, 0, 1},
		"v1.2.3":   {1, 2, 3},
		" v10.0.0": {10, 0, 0},
	}
	for in, want := range good {
		got, err := Parse(in)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "dev", "1.2", "1.2.3.4", "1.x.3", "v1.02.3", "-1.0.0", "1.2.3-rc1"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) should fail", in)
		}
	}
}

func TestNewer(t *testing.T) {
	tests := []struct {
		candidate, current string
		want               bool
	}{
		{"v0.0.2", "0.0.1", true},
		{"v0.1.0", "0.0.9", true},
		{"v1.0.0", "0.9.9", true},
		{"v0.0.10", "0.0.9", true},
		{"v0.0.1", "0.0.1", false},
		{"v0.0.1", "0.0.2", false},
		{"v0.0.1", "dev", true},
		{"garbage", "0.0.1", false},
	}
	for _, tt := range tests {
		if got := Newer(tt.candidate, tt.current); got != tt.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", tt.candidate, tt.current, got, tt.want)
		}
	}
}

func TestString(t *testing.T) {
	old := Version
	defer func() { Version = old }()

	Version = "dev"
	if String() != "dev" {
		t.Fatal(String())
	}
	Version = "0.0.2"
	if String() != "v0.0.2" {
		t.Fatal(String())
	}
}
