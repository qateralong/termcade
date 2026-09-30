package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGitHub serves one release with a binary and checksums.
func fakeGitHub(t *testing.T, binary []byte, checksum string) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	mux := http.NewServeMux()
	auth := func(w http.ResponseWriter, r *http.Request) bool {
		if r.Header.Get("Authorization") != "Bearer secret" {
			http.Error(w, "nope", http.StatusUnauthorized)
			return false
		}
		return true
	}
	release := func(w http.ResponseWriter, r *http.Request) {
		if !auth(w, r) {
			return
		}
		json.NewEncoder(w).Encode(Release{
			Tag: "v0.2.0",
			Assets: []Asset{
				{ID: 1, Name: AssetName("linux", "amd64"), URL: srv.URL + "/assets/1"},
				{ID: 2, Name: ChecksumsName, URL: srv.URL + "/assets/2"},
			},
		})
	}
	mux.HandleFunc("/repos/o/r/releases/latest", release)
	mux.HandleFunc("/repos/o/r/releases/tags/v0.2.0", release)
	mux.HandleFunc("/assets/1", func(w http.ResponseWriter, r *http.Request) {
		// GitHub redirects asset downloads to its storage host.
		if auth(w, r) {
			http.Redirect(w, r, "/storage/1", http.StatusFound)
		}
	})
	mux.HandleFunc("/storage/1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/octet-stream" {
			t.Errorf("asset requested with Accept %q", r.Header.Get("Accept"))
		}
		w.Write(binary)
	})
	mux.HandleFunc("/assets/2", func(w http.ResponseWriter, r *http.Request) {
		if auth(w, r) {
			fmt.Fprintf(w, "%s  %s\n%s  termcade-linux-arm64\n", checksum, AssetName("linux", "amd64"), strings.Repeat("0", 64))
		}
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func sum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func TestFetchBinary(t *testing.T) {
	ctx := context.Background()
	bin := []byte("#!/new binary")
	srv := fakeGitHub(t, bin, sum(bin))
	c := &Client{Repo: "o/r", Token: "secret", BaseURL: srv.URL}

	rel, err := c.Latest(ctx)
	if err != nil || rel.Tag != "v0.2.0" {
		t.Fatalf("Latest = %+v, %v", rel, err)
	}
	if _, err := c.ByTag(ctx, "0.2.0"); err != nil {
		t.Fatalf("ByTag without v prefix: %v", err)
	}
	got, err := c.FetchBinary(ctx, rel, "linux", "amd64")
	if err != nil || string(got) != string(bin) {
		t.Fatalf("FetchBinary = %q, %v", got, err)
	}
	if _, err := c.FetchBinary(ctx, rel, "linux", "riscv64"); !errors.Is(err, ErrNoAsset) {
		t.Fatalf("expected ErrNoAsset, got %v", err)
	}
	if _, err := c.ByTag(ctx, "v9.9.9"); !errors.Is(err, ErrNoRelease) {
		t.Fatalf("expected ErrNoRelease, got %v", err)
	}

	bad := &Client{Repo: "o/r", Token: "wrong", BaseURL: srv.URL}
	if _, err := bad.Latest(ctx); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestFetchBinaryRejectsBadChecksum(t *testing.T) {
	bin := []byte("tampered")
	srv := fakeGitHub(t, bin, sum([]byte("original")))
	c := &Client{Repo: "o/r", Token: "secret", BaseURL: srv.URL}
	rel, err := c.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.FetchBinary(context.Background(), rel, "linux", "amd64"); !errors.Is(err, ErrChecksum) {
		t.Fatalf("expected ErrChecksum, got %v", err)
	}
}

func TestInstallAndRollback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "termcade")
	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	backup, err := Install(path, []byte("new"))
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != "new" {
		t.Fatalf("installed = %q", b)
	}
	if b, _ := os.ReadFile(backup); string(b) != "old" {
		t.Fatalf("backup = %q", b)
	}

	if err := Rollback(path, backup); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != "old" {
		t.Fatalf("after rollback = %q", b)
	}

	// No temp files are left behind.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("leftover files: %v", entries)
	}
}

func TestInstallFresh(t *testing.T) {
	path := filepath.Join(t.TempDir(), "termcade")
	backup, err := Install(path, []byte("bin"))
	if err != nil || backup != "" {
		t.Fatalf("Install = %q, %v", backup, err)
	}
}
