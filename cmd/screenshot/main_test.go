package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
