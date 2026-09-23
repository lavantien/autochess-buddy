package main

import (
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/seed"
	"github.com/lavantien/autochess-buddy/internal/store/sqlite"
)

func writeTemp(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "readme.md")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRefreshReadme_InsertsUnderFirstHeading(t *testing.T) {
	p := writeTemp(t, "# autochess companion\n\narchitecture prose\n")
	if err := refreshReadme(p, "docs/screenshots/dashboard.png"); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	got, _ := os.ReadFile(p)
	want := "# autochess companion\n\n" +
		"<!-- ac-shot:start -->\n![dashboard](docs/screenshots/dashboard.png)\n<!-- ac-shot:end -->\n" +
		"\narchitecture prose\n"
	if string(got) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRefreshReadme_ReplacesBlockIdempotently(t *testing.T) {
	p := writeTemp(t, "# title\n\n<!-- ac-shot:start -->\n![dashboard](old.png)\n<!-- ac-shot:end -->\n\nbody\n")
	for i, img := range []string{"new.png", "new.png"} {
		if err := refreshReadme(p, img); err != nil {
			t.Fatalf("refresh %d: %v", i, err)
		}
	}
	got, _ := os.ReadFile(p)
	if strings.Count(string(got), shotStart) != 1 || strings.Contains(string(got), "old.png") {
		t.Fatalf("block not replaced exactly once:\n%s", got)
	}
	if !strings.Contains(string(got), "![dashboard](new.png)") {
		t.Fatalf("new image missing:\n%s", got)
	}
}

func TestRefreshReadme_RejectsHeadinglessFile(t *testing.T) {
	p := writeTemp(t, "just prose, no heading\n")
	if err := refreshReadme(p, "x.png"); err == nil {
		t.Fatal("want error for file without a leading heading")
	}
}

func TestRefreshReadme_RejectsMissingFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "absent.md")
	if err := refreshReadme(p, "x.png"); err == nil {
		t.Fatal("want error for a missing readme file")
	}
}

func TestRefreshReadme_RejectsEndBeforeStart(t *testing.T) {
	p := writeTemp(t, "# title\n\n"+shotEnd+"\nbody\n"+shotStart+"\n")
	err := refreshReadme(p, "x.png")
	if err == nil {
		t.Fatal("want error when the end marker precedes the start marker")
	}
	if !strings.Contains(err.Error(), "end marker before start") {
		t.Fatalf("wrong error: %v", err)
	}
}

func TestNewShotServer_SetsHeaderTimeout(t *testing.T) {
	srv := newShotServer(http.NotFoundHandler())
	if srv.ReadHeaderTimeout <= 0 {
		t.Fatalf("ReadHeaderTimeout = %v, want a positive cap on header reads", srv.ReadHeaderTimeout)
	}
	if srv.Handler == nil {
		t.Fatal("Handler is nil, want the passed handler wired through")
	}
}

func TestBootSeededApp_ServesDashboardAndDrains(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "shot.db")
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	base, stop, err := bootSeededApp(lis, dbPath)
	if err != nil {
		t.Fatalf("boot: %v", err)
	}
	resp, err := http.Get(base + "/dashboard")
	if err != nil {
		t.Fatalf("get dashboard: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusOK || len(body) == 0 {
		t.Fatalf("dashboard status %d, %d bytes", resp.StatusCode, len(body))
	}
	if err := stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

func TestBootSeededApp_BadDbPathFails(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = lis.Close() }()
	dbPath := filepath.Join(t.TempDir(), "missing", "app.db")
	_, _, err = bootSeededApp(lis, dbPath)
	if err == nil || !strings.Contains(err.Error(), "open sqlite") {
		t.Fatalf("err = %v, want open sqlite failure", err)
	}
}

func TestBootSeededApp_SecondSeedRefused(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "app.db")
	st, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := seed.Load(st.DB); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = lis.Close() }()
	if _, _, err := bootSeededApp(lis, dbPath); err == nil || !strings.Contains(err.Error(), "seed") {
		t.Fatalf("err = %v, want seed refusal", err)
	}
}
