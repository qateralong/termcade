package deploy

import (
	"strings"
	"testing"
)

func TestEmbeddedUnit(t *testing.T) {
	for _, want := range []string{
		"ExecStart=" + BinPath + " serve",
		"EnvironmentFile=-" + EnvFile,
		"StateDirectory=" + Service,
	} {
		if !strings.Contains(Unit, want) {
			t.Errorf("unit is missing %q", want)
		}
	}
}

func TestDefaultEnv(t *testing.T) {
	if !strings.Contains(DefaultEnv(":2222"), "\nTERMCADE_ADDR=:2222\n") {
		t.Fatal(DefaultEnv(":2222"))
	}
}
