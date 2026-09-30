// Package deploy knows how Termcade is laid out on a Linux server and how to
// manage its systemd service.
package deploy

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Unit is the systemd unit file shipped inside the binary, so updating the
// binary can also update the service definition.
//
//go:embed termcade.service
var Unit string

// Locations on the server.
const (
	Service   = "termcade"
	BinPath   = "/usr/local/bin/termcade"
	UnitPath  = "/etc/systemd/system/termcade.service"
	ConfigDir = "/etc/termcade"
	EnvFile   = ConfigDir + "/termcade.env"
	TokenFile = ConfigDir + "/github-token"
)

// DefaultEnv returns the initial contents of EnvFile.
func DefaultEnv(addr string) string {
	return `# Termcade settings. Apply changes with: sudo systemctl restart termcade
#
# Address players connect to. Use :22 to allow a plain "ssh your-host", after
# moving the server's own OpenSSH to another port.
TERMCADE_ADDR=` + addr + `
#
# Where "termcade update" looks for releases.
#TERMCADE_REPO=qateralong/termcade
`
}

// ErrNotRoot is returned when an operation needs root privileges.
var ErrNotRoot = errors.New("this needs root, run it with sudo")

// RequireRoot fails unless running as root on Linux.
func RequireRoot() error {
	if os.Geteuid() != 0 {
		return ErrNotRoot
	}
	return nil
}

// WriteUnit installs the embedded unit file. It reports whether the file
// changed, in which case systemd must be reloaded.
func WriteUnit() (changed bool, err error) {
	old, err := os.ReadFile(UnitPath)
	if err == nil && string(old) == Unit {
		return false, nil
	}
	if err := os.WriteFile(UnitPath, []byte(Unit), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// Installed reports whether the service unit exists.
func Installed() bool {
	_, err := os.Stat(UnitPath)
	return err == nil
}

func systemctl(args ...string) (string, error) {
	out, err := exec.Command("systemctl", args...).CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		if s == "" {
			s = err.Error()
		}
		return s, fmt.Errorf("systemctl %s: %s", strings.Join(args, " "), s)
	}
	return s, nil
}

// DaemonReload makes systemd pick up unit file changes.
func DaemonReload() error {
	_, err := systemctl("daemon-reload")
	return err
}

// Enable enables the service at boot and starts (or restarts) it now.
func Enable() error {
	if _, err := systemctl("enable", Service); err != nil {
		return err
	}
	return Restart()
}

// Restart restarts the service.
func Restart() error {
	_, err := systemctl("restart", Service)
	return err
}

// State returns systemd's active state: "active", "activating", "failed"...
func State() string {
	s, _ := systemctl("is-active", Service)
	return s
}

// WaitHealthy waits for the service to come up and stay up for the given
// period. A binary that crashes on start shows up as "activating" (waiting
// to be restarted) or "failed" instead of "active".
func WaitHealthy(period time.Duration) error {
	deadline := time.Now().Add(period)
	for {
		if s := State(); s != "active" {
			// Give a fresh start a moment before judging it.
			if time.Until(deadline) > period/2 {
				time.Sleep(250 * time.Millisecond)
				continue
			}
			return fmt.Errorf("service is %q; see: journalctl -u %s -n 50", s, Service)
		}
		if time.Now().After(deadline) {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// ReadToken reads the GitHub token saved by install.
func ReadToken(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// SaveToken writes the GitHub token readable by root only.
func SaveToken(path, token string) error {
	if err := os.MkdirAll(ConfigDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(token)+"\n"), 0o600)
}
