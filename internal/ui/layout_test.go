package ui

import (
	"context"
	"io"
	"testing"

	"github.com/a-h/templ"
)

func TestBaseLayout_Golden(t *testing.T) {
	content := templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := io.WriteString(w, "<p>probe</p>")
		return err
	})
	got := renderComp(t, BaseLayout("codex", "heroes", content))
	want := `<!doctype html><html lang="en"><head>` +
		`<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">` +
		`<title>heroes</title>` +
		`<link rel="stylesheet" href="/static/app.css">` +
		`<script src="/static/htmx.min.js" defer></script></head><body>` +
		`<header class="topbar"><a class="brand" href="/dashboard">autochess companion</a>` +
		`<nav aria-label="primary">` +
		`<a class="navlink" href="/dashboard">dashboard</a> ` +
		`<a class="navlink" href="/matches">matches</a> ` +
		`<a class="navlink navlink-active" href="/heroes">codex</a></nav></header>` +
		`<main class="content">` +
		`<nav class="tabs" aria-label="codex">` +
		`<a class="tab tab-active" href="/heroes">heroes</a>` +
		`<a class="tab" href="/races">synergies</a>` +
		`<a class="tab" href="/items">items</a>` +
		`<a class="tab" href="/relics">relics</a>` +
		`<a class="tab" href="/patches">patches</a>` +
		`<a class="tab" href="/pros">pros</a></nav>` +
		`<p>probe</p></main></body></html>`
	if got != want {
		t.Fatalf("base layout golden mismatch\n got: %q\nwant: %q", got, want)
	}
}
