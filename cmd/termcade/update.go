package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"termcade/deploy"
	"termcade/internal/update"
	"termcade/internal/version"
)

func updateCmd(args []string) error {
	fset := flag.NewFlagSet("update", flag.ExitOnError)
	var (
		check     = fset.Bool("check", false, "only report whether a newer version exists")
		to        = fset.String("to", "", "install this version instead of the newest, e.g. 0.1.0 (allows downgrades)")
		force     = fset.Bool("force", false, "reinstall even if already up to date")
		noRestart = fset.Bool("no-restart", false, "don't restart the service afterwards")
		repo      = fset.String("repo", env("TERMCADE_REPO", update.DefaultRepo), "GitHub repository (env TERMCADE_REPO)")
		tokenFile = fset.String("token-file", deploy.TokenFile, "file holding a GitHub token (env GITHUB_TOKEN wins)")
	)
	fset.Parse(args)

	if !*check && runtime.GOOS == "linux" {
		if err := deploy.RequireRoot(); err != nil {
			return fmt.Errorf("%w: sudo termcade update", err)
		}
	}
	token, err := loadToken(*tokenFile)
	if err != nil {
		return err
	}

	header("update")
	client := &update.Client{Repo: *repo, Token: token}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var rel *update.Release
	if *to != "" {
		rel, err = client.ByTag(ctx, *to)
	} else {
		rel, err = client.Latest(ctx)
	}
	if errors.Is(err, update.ErrNoRelease) {
		if *to != "" {
			return fmt.Errorf("there is no release %s in %s", *to, *repo)
		}
		return fmt.Errorf("%s has no releases yet (or the token can't see it)", *repo)
	}
	if err != nil {
		return err
	}

	row("current", highlight(version.String()))
	row("latest", highlight(rel.Tag)+th.Faded.Render("  published "+rel.PublishedAt.Local().Format("2 Jan 2006 15:04")))
	fmt.Println()

	newer := version.Newer(rel.Tag, version.Version)
	if !newer && *to == "" && !*force {
		ok("Already up to date.")
		fmt.Println()
		return nil
	}
	if *check {
		note("Version %s is available. Install it with %s", rel.Tag, code("sudo termcade update"))
		fmt.Println()
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return err
	}

	data, err := client.FetchBinary(ctx, rel, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	ok("Downloaded %s %s, checksum verified", update.AssetName(runtime.GOOS, runtime.GOARCH),
		th.Faded.Render(fmt.Sprintf("(%.1f MB)", float64(len(data))/(1<<20))))

	backup, err := update.Install(exe, data)
	if errors.Is(err, fs.ErrPermission) {
		return fmt.Errorf("%w; try: sudo termcade update", err)
	}
	if err != nil {
		return err
	}
	ok("Installed %s", exe)

	if runtime.GOOS != "linux" || !deploy.Installed() {
		note("No termcade service found; restart the server yourself to use %s.", rel.Tag)
		fmt.Println()
		return nil
	}

	// The new binary carries its own unit file; let it update the service
	// definition before the restart.
	if err := refreshService(exe); err != nil {
		rollback(exe, backup)
		return err
	}
	if *noRestart {
		note("Restart skipped. Run %s to switch to %s.", code("sudo systemctl restart termcade"), rel.Tag)
		fmt.Println()
		return nil
	}

	if err := restartAndCheck(); err != nil {
		rollback(exe, backup)
		if rerr := restartAndCheck(); rerr != nil {
			return fmt.Errorf("%s failed to start (%v), and so did the previous version after rolling back: %v", rel.Tag, err, rerr)
		}
		return fmt.Errorf("%s failed to start, so the previous version was restored and is running again: %v", rel.Tag, err)
	}
	ok("Restarted, the arcade is now running %s", highlight(rel.Tag))
	if backup != "" {
		hint("  previous version kept at %s", backup)
	}
	if rel.URL != "" {
		hint("  release notes: %s", rel.URL)
	}
	fmt.Println()
	return nil
}

func loadToken(path string) (string, error) {
	if t := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); t != "" {
		return t, nil
	}
	t, err := deploy.ReadToken(path)
	switch {
	case err == nil:
		return t, nil
	case errors.Is(err, fs.ErrNotExist):
		return "", nil // fine for public repositories
	case errors.Is(err, fs.ErrPermission):
		return "", fmt.Errorf("can't read the GitHub token in %s; run with sudo", path)
	default:
		return "", err
	}
}

// refreshService runs "<exe> install --refresh" so the freshly installed
// binary writes its own unit file.
func refreshService(exe string) error {
	out, err := exec.Command(exe, "install", "--refresh").CombinedOutput()
	if err != nil {
		return fmt.Errorf("new version could not update the service: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func restartAndCheck() error {
	if err := deploy.Restart(); err != nil {
		return err
	}
	return deploy.WaitHealthy(6 * time.Second)
}

func rollback(exe, backup string) {
	if backup == "" {
		return
	}
	if err := update.Rollback(exe, backup); err != nil {
		note("Rolling back failed: %v", err)
		return
	}
	if err := refreshService(exe); err != nil {
		note("Restored the previous binary, but not its service file: %v", err)
		return
	}
	note("Rolled back to %s", version.String())
}
