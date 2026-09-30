package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"termcade/deploy"
	"termcade/internal/update"
	"termcade/internal/version"
)

func installCmd(args []string) error {
	fset := flag.NewFlagSet("install", flag.ExitOnError)
	var (
		addr    = fset.String("addr", ":2222", "address players connect to (only used when creating the config)")
		token   = fset.String("token", os.Getenv("GITHUB_TOKEN"), "GitHub token for updates from a private repository (env GITHUB_TOKEN)")
		refresh = fset.Bool("refresh", false, "only update the service file (used by termcade update)")
	)
	fset.Parse(args)

	if runtime.GOOS != "linux" {
		return errors.New("install sets up a systemd service and only works on Linux")
	}
	if err := deploy.RequireRoot(); err != nil {
		return fmt.Errorf("%w: sudo termcade install", err)
	}

	if *refresh {
		changed, err := deploy.WriteUnit()
		if err != nil {
			return err
		}
		if changed {
			return deploy.DaemonReload()
		}
		return nil
	}

	header("install")

	// 1. Put the binary in place.
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return err
	}
	if exe != deploy.BinPath {
		data, err := os.ReadFile(exe)
		if err != nil {
			return err
		}
		if _, err := update.Install(deploy.BinPath, data); err != nil {
			return err
		}
		os.Remove(deploy.BinPath + ".old") // nothing to roll back to on a fresh install
	}
	ok("Installed %s %s", deploy.BinPath, th.Faded.Render("("+version.String()+")"))

	// 2. Configuration.
	if err := os.MkdirAll(deploy.ConfigDir, 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(deploy.EnvFile); errors.Is(err, fs.ErrNotExist) {
		if err := os.WriteFile(deploy.EnvFile, []byte(deploy.DefaultEnv(*addr)), 0o644); err != nil {
			return err
		}
		ok("Created %s", deploy.EnvFile)
	} else {
		ok("Kept existing %s", deploy.EnvFile)
	}
	if *token != "" {
		if err := deploy.SaveToken(deploy.TokenFile, *token); err != nil {
			return err
		}
		ok("Saved the GitHub token to %s %s", deploy.TokenFile, th.Faded.Render("(root only)"))
	} else if _, err := os.Stat(deploy.TokenFile); err != nil {
		note("No GitHub token saved. Updates from a private repository need one:")
		hint("  sudo GITHUB_TOKEN=<token> termcade install")
	}

	// 3. The service.
	if _, err := deploy.WriteUnit(); err != nil {
		return err
	}
	if err := deploy.DaemonReload(); err != nil {
		return err
	}
	if err := deploy.Enable(); err != nil {
		return err
	}
	if err := deploy.WaitHealthy(4 * time.Second); err != nil {
		return err
	}
	ok("Service %s is running and starts on boot", highlight(deploy.Service))

	fmt.Println()
	port := listenPort(deploy.EnvFile)
	if port == "22" {
		hint("  Players connect with:  %s", code("ssh <this server>"))
	} else {
		hint("  Players connect with:  %s", code("ssh -p "+port+" <this server>"))
	}
	hint("  Update to the newest version:  %s", code("sudo termcade update"))
	hint("  Logs:  %s", code("journalctl -u termcade -f"))
	fmt.Println()
	return nil
}

// listenPort reads TERMCADE_ADDR from the env file and returns its port.
func listenPort(envFile string) string {
	addr := ":2222"
	if f, err := os.Open(envFile); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if v, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "TERMCADE_ADDR="); ok {
				addr = strings.Trim(v, `"'`)
			}
		}
	}
	if _, port, err := net.SplitHostPort(addr); err == nil {
		return port
	}
	return addr
}
