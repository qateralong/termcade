// Package update fetches Termcade releases from GitHub and swaps the running
// executable for a new one.
//
// Releases are published by .github/workflows/release.yml. Each release has
// one binary per platform, named by AssetName, plus a checksums.txt file in
// the format produced by sha256sum.
package update

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultRepo is the GitHub repository releases are fetched from.
const DefaultRepo = "qateralong/termcade"

// ChecksumsName is the name of the checksum file attached to every release.
const ChecksumsName = "checksums.txt"

// maxBinarySize guards against downloading something absurd.
const maxBinarySize = 200 << 20

var (
	ErrNoRelease    = errors.New("no release found")
	ErrUnauthorized = errors.New("GitHub rejected the token (is it valid, and can it read the repository?)")
	ErrNoAsset      = errors.New("the release has no binary for this platform")
	ErrChecksum     = errors.New("checksum mismatch: the download is corrupt or was tampered with")
)

// AssetName is the release asset name of the binary for a platform.
func AssetName(goos, goarch string) string {
	return "termcade-" + goos + "-" + goarch
}

// Release is a published GitHub release.
type Release struct {
	Tag         string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	URL         string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	Assets      []Asset   `json:"assets"`
}

// Asset is a file attached to a release.
type Asset struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
	URL  string `json:"url"` // API URL; works for private repositories
}

// Asset returns the asset with the given name.
func (r *Release) Asset(name string) (Asset, bool) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

// Client talks to the GitHub REST API.
type Client struct {
	Repo    string // "owner/name"
	Token   string // needed for private repositories
	BaseURL string // defaults to https://api.github.com
	HTTP    *http.Client
}

func (c *Client) base() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return "https://api.github.com"
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 5 * time.Minute}
}

func (c *Client) get(ctx context.Context, url, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "termcade-updater")
	if c.Token != "" {
		// Go drops this header when GitHub redirects the download to its
		// storage host, which is exactly what we want.
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return resp, nil
	case http.StatusUnauthorized, http.StatusForbidden:
		resp.Body.Close()
		return nil, ErrUnauthorized
	case http.StatusNotFound:
		resp.Body.Close()
		// Private repositories also answer 404 when the token can't see them.
		return nil, ErrNoRelease
	default:
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		resp.Body.Close()
		return nil, fmt.Errorf("GitHub API: %s: %s", resp.Status, bytes.TrimSpace(msg))
	}
}

func (c *Client) release(ctx context.Context, path string) (*Release, error) {
	resp, err := c.get(ctx, c.base()+"/repos/"+c.Repo+"/releases/"+path, "application/vnd.github+json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var r Release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}
	return &r, nil
}

// Latest returns the newest published (non-draft, non-prerelease) release.
func (c *Client) Latest(ctx context.Context) (*Release, error) {
	return c.release(ctx, "latest")
}

// ByTag returns the release for a tag such as "v0.1.0".
func (c *Client) ByTag(ctx context.Context, tag string) (*Release, error) {
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}
	return c.release(ctx, "tags/"+tag)
}

// download fetches an asset's contents.
func (c *Client) download(ctx context.Context, a Asset) ([]byte, error) {
	resp, err := c.get(ctx, a.URL, "application/octet-stream")
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", a.Name, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBinarySize+1))
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", a.Name, err)
	}
	if len(data) > maxBinarySize {
		return nil, fmt.Errorf("download %s: file is too large", a.Name)
	}
	return data, nil
}

// FetchBinary downloads the binary for goos/goarch from a release and checks
// it against the release's checksums.
func (c *Client) FetchBinary(ctx context.Context, r *Release, goos, goarch string) ([]byte, error) {
	name := AssetName(goos, goarch)
	bin, ok := r.Asset(name)
	if !ok {
		return nil, fmt.Errorf("%w (%s)", ErrNoAsset, name)
	}
	sums, ok := r.Asset(ChecksumsName)
	if !ok {
		return nil, fmt.Errorf("release %s has no %s", r.Tag, ChecksumsName)
	}
	sumData, err := c.download(ctx, sums)
	if err != nil {
		return nil, err
	}
	want, err := findChecksum(sumData, name)
	if err != nil {
		return nil, err
	}
	data, err := c.download(ctx, bin)
	if err != nil {
		return nil, err
	}
	got := sha256.Sum256(data)
	if hex.EncodeToString(got[:]) != want {
		return nil, ErrChecksum
	}
	return data, nil
}

// findChecksum looks name up in sha256sum-style output.
func findChecksum(sums []byte, name string) (string, error) {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			sum := strings.ToLower(fields[0])
			if len(sum) != sha256.Size*2 {
				break
			}
			return sum, nil
		}
	}
	return "", fmt.Errorf("no checksum for %s in %s", name, ChecksumsName)
}

// Install replaces the executable at path with data. The previous version
// is kept next to it with a ".old" suffix so it can be restored with
// Rollback. The swap uses renames within one directory, so the path always
// points at a complete binary.
func Install(path string, data []byte) (backup string, err error) {
	dir := filepath.Dir(path)
	mode := os.FileMode(0o755)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}

	tmp, err := os.CreateTemp(dir, ".termcade-new-*")
	if err != nil {
		return "", fmt.Errorf("write new binary: %w", err)
	}
	defer os.Remove(tmp.Name()) // no-op once renamed
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", fmt.Errorf("write new binary: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return "", err
	}

	backup = path + ".old"
	if _, err := os.Stat(path); err == nil {
		os.Remove(backup)
		if err := os.Rename(path, backup); err != nil {
			return "", fmt.Errorf("back up current binary: %w", err)
		}
	} else {
		backup = ""
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		if backup != "" {
			os.Rename(backup, path)
		}
		return "", fmt.Errorf("install new binary: %w", err)
	}
	return backup, nil
}

// Rollback restores the backup made by Install.
func Rollback(path, backup string) error {
	if backup == "" {
		return errors.New("no backup to restore")
	}
	return os.Rename(backup, path)
}
