package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleProfile = `mode: atomic

a.go:1.2,3.4 5 1
a.go:5.1,6.2 3 0
b.go:10.1,12.3 2 7
`

func TestParseProfileSumsCoveredStatements(t *testing.T) {
	covered, total, err := parseProfile(strings.NewReader(sampleProfile), nil)
	if err != nil {
		t.Fatalf("parseProfile: %v", err)
	}
	if covered != 7 || total != 10 {
		t.Fatalf("covered=%d total=%d, want 7/10", covered, total)
	}
}

func TestParseProfileRejectsMalformedLines(t *testing.T) {
	for _, tc := range []struct{ name, profile string }{
		{"two fields", "mode: set\na.go:1.2 5\n"},
		{"non numeric statements", "mode: set\na.go:1.2,3.4 x 1\n"},
		{"non numeric count", "mode: set\na.go:1.2,3.4 5 y\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := parseProfile(strings.NewReader(tc.profile), nil); err == nil {
				t.Fatal("want error, got nil")
			}
		})
	}
}

func TestParseProfileSkipsMatchedPaths(t *testing.T) {
	profile := "mode: atomic\nx_templ.go:1.1,2.2 5 0\na.go:1.1,2.2 3 1\n"
	for _, tc := range []struct {
		name    string
		skip    []string
		covered int
		total   int
	}{
		{"no skip", nil, 3, 8},
		{"skip templ", []string{"_templ.go"}, 3, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			covered, total, err := parseProfile(strings.NewReader(profile), tc.skip)
			if err != nil {
				t.Fatalf("parseProfile: %v", err)
			}
			if covered != tc.covered || total != tc.total {
				t.Fatalf("covered=%d total=%d, want %d/%d", covered, total, tc.covered, tc.total)
			}
		})
	}
}

func TestParseProfileDedupsDuplicateBlocks(t *testing.T) {
	// -coverpkg profiles repeat each block once per test binary: counts
	// must sum, statements count once.
	profile := "mode: atomic\n" +
		"a.go:1.1,2.2 5 1\n" +
		"a.go:1.1,2.2 5 0\n" +
		"b.go:1.1,2.2 3 0\n" +
		"b.go:1.1,2.2 3 0\n" +
		"c.go:1.1,2.2 2 4\n" +
		"c.go:1.1,2.2 2 5\n"
	covered, total, err := parseProfile(strings.NewReader(profile), nil)
	if err != nil {
		t.Fatalf("parseProfile: %v", err)
	}
	if covered != 7 || total != 10 {
		t.Fatalf("covered=%d total=%d, want 7/10", covered, total)
	}
}

func TestParseProfileRejectsMismatchedDuplicateStmts(t *testing.T) {
	profile := "mode: atomic\na.go:1.1,2.2 5 1\na.go:1.1,2.2 6 0\n"
	_, _, err := parseProfile(strings.NewReader(profile), nil)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("want duplicate-block error, got %v", err)
	}
}

func TestParseProfileSkipAppliesToAllDuplicates(t *testing.T) {
	profile := "mode: atomic\n" +
		"x_templ.go:1.1,2.2 10 0\n" +
		"x_templ.go:1.1,2.2 10 1\n" +
		"a.go:1.1,2.2 3 1\n"
	covered, total, err := parseProfile(strings.NewReader(profile), []string{"_templ.go"})
	if err != nil {
		t.Fatalf("parseProfile: %v", err)
	}
	if covered != 3 || total != 3 {
		t.Fatalf("covered=%d total=%d, want 3/3", covered, total)
	}
}

func TestPctTenths(t *testing.T) {
	for _, tc := range []struct{ covered, total, want int }{
		{0, 0, 0},
		{1, 1, 1000},
		{1, 3, 333},
		{2, 3, 667},
		{899, 1000, 899},
		{900, 1000, 900},
	} {
		if got := pctTenths(tc.covered, tc.total); got != tc.want {
			t.Fatalf("pctTenths(%d, %d) = %d, want %d", tc.covered, tc.total, got, tc.want)
		}
	}
}

func TestColorForTiers(t *testing.T) {
	for _, tc := range []struct {
		tenths int
		want   string
	}{
		{0, "red"}, {599, "red"},
		{600, "orange"}, {699, "orange"},
		{700, "yellow"}, {799, "yellow"},
		{800, "green"}, {899, "green"},
		{900, "blue"}, {1000, "blue"},
	} {
		if got := colorFor(tc.tenths); got != tc.want {
			t.Fatalf("colorFor(%d) = %q, want %q", tc.tenths, got, tc.want)
		}
	}
}

func writeProfile(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "coverage.out")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunWritesBadgeEvenWhenGateFails(t *testing.T) {
	out := filepath.Join(t.TempDir(), "coverage.json")
	tenths, err := run(writeProfile(t, "mode: atomic\na.go:1.1,2.2 10 0\n"), out, 90, nil)
	if err == nil {
		t.Fatal("want gate error below minimum, got nil")
	}
	if tenths != 0 {
		t.Fatalf("tenths = %d, want 0", tenths)
	}
	body, rerr := os.ReadFile(out)
	if rerr != nil {
		t.Fatalf("badge json missing after gate failure: %v", rerr)
	}
	const want = `{"schemaVersion":1,"label":"coverage","message":"0.0%","color":"red"}`
	if string(body) != want {
		t.Fatalf("badge json = %s, want %s", body, want)
	}
}

func TestRunPassesFullCoverage(t *testing.T) {
	out := filepath.Join(t.TempDir(), "coverage.json")
	tenths, err := run(writeProfile(t, "mode: atomic\na.go:1.1,2.2 10 10\n"), out, 90, nil)
	if err != nil {
		t.Fatalf("want pass at full coverage: %v", err)
	}
	if tenths != 1000 {
		t.Fatalf("tenths = %d, want 1000", tenths)
	}
	body, _ := os.ReadFile(out)
	const want = `{"schemaVersion":1,"label":"coverage","message":"100.0%","color":"blue"}`
	if string(body) != want {
		t.Fatalf("badge json = %s, want %s", body, want)
	}
}

func TestRunGateBoundary(t *testing.T) {
	// 899 tenths (89.9%) passes min 89.9 and fails min 90.
	profile := "mode: atomic\na.go:1.1,2.2 899 1\na.go:3.1,4.2 101 0\n"
	out := filepath.Join(t.TempDir(), "coverage.json")
	if _, err := run(writeProfile(t, profile), out, 89.9, nil); err != nil {
		t.Fatalf("899 tenths vs min 89.9: %v", err)
	}
	if _, err := run(writeProfile(t, profile), out, 90, nil); err == nil {
		t.Fatal("899 tenths vs min 90: want error, got nil")
	}
}

func TestRunEmptyProfileFailsGate(t *testing.T) {
	out := filepath.Join(t.TempDir(), "coverage.json")
	if _, err := run(writeProfile(t, "mode: atomic\n"), out, 90, nil); err == nil {
		t.Fatal("empty profile: want error, got nil")
	}
	body, _ := os.ReadFile(out)
	const want = `{"schemaVersion":1,"label":"coverage","message":"0.0%","color":"red"}`
	if string(body) != want {
		t.Fatalf("badge json = %s, want %s", body, want)
	}
}

func TestRunRejectsBadInputs(t *testing.T) {
	if _, err := run(writeProfile(t, sampleProfile), filepath.Join(t.TempDir(), "c.json"), 0, nil); err == nil {
		t.Fatal("min 0: want error, got nil")
	}
	if _, err := run(filepath.Join(t.TempDir(), "missing.out"), filepath.Join(t.TempDir(), "c.json"), 90, nil); err == nil {
		t.Fatal("missing profile: want error, got nil")
	}
}

func TestRunRejectsDegenerateMin(t *testing.T) {
	out := filepath.Join(t.TempDir(), "coverage.json")
	zero := "mode: atomic\na.go:1.1,2.2 10 0\n"
	for _, min := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 1e300, -1, 0} {
		if _, err := run(writeProfile(t, zero), out, min, nil); err == nil {
			t.Fatalf("min %v: want error, got nil", min)
		}
	}
	// A positive min below one tenth still demands at least 0.1% coverage.
	if _, err := run(writeProfile(t, zero), out, 0.04, nil); err == nil {
		t.Fatal("min 0.04 at 0% coverage: want gate error, got nil")
	}
}

func TestRunRejectsEmptySkipPattern(t *testing.T) {
	out := filepath.Join(t.TempDir(), "coverage.json")
	profile := "mode: atomic\na.go:1.1,2.2 899 1\na.go:3.1,4.2 101 0\n"
	_, err := run(writeProfile(t, profile), out, 90, []string{"a.go", ""})
	if err == nil || !strings.Contains(err.Error(), "skip") {
		t.Fatalf("want skip validation error, got %v", err)
	}
	if _, err := run(writeProfile(t, profile), out, 80, []string{"_missing.go"}); err != nil {
		t.Fatalf("clean skip list: %v", err)
	}
}

func TestRunSkipFilterExcludesGeneratedFiles(t *testing.T) {
	// Generated _templ.go blocks sink the metric; skipping them turns a
	// failing 50% tree into a passing 100% hand-written tree.
	profile := "mode: atomic\nx_templ.go:1.1,2.2 10 0\na.go:1.1,2.2 10 10\n"
	out := filepath.Join(t.TempDir(), "coverage.json")
	if _, err := run(writeProfile(t, profile), out, 90, nil); err == nil {
		t.Fatal("without skip: want gate error at 50%, got nil")
	}
	tenths, err := run(writeProfile(t, profile), out, 90, []string{"_templ.go"})
	if err != nil {
		t.Fatalf("with skip: %v", err)
	}
	if tenths != 1000 {
		t.Fatalf("tenths = %d, want 1000", tenths)
	}
	body, _ := os.ReadFile(out)
	const want = `{"schemaVersion":1,"label":"coverage","message":"100.0%","color":"blue"}`
	if string(body) != want {
		t.Fatalf("badge json = %s, want %s", body, want)
	}
}
