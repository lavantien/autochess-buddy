// badge reads a go cover profile, writes a shields.io endpoint json beside
// it, and gates total statement coverage against -min. Lines whose path
// matches a -skip substring (generated code, e.g. _templ.go) stay out of
// the metric. Run it (make badge); ci publishes coverage.json to the orphan
// badges branch.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
)

func main() {
	profile := flag.String("profile", "coverage.out", "go cover profile")
	out := flag.String("out", "coverage.json", "shields endpoint json output")
	min := flag.Float64("min", 90, "minimum total coverage percent")
	skip := flag.String("skip", "", "comma-separated path substrings kept out of the metric")
	flag.Parse()
	tenths, err := run(*profile, *out, *min, splitSkip(*skip))
	if err != nil {
		log.Fatal("badge: ", err)
	}
	fmt.Printf("coverage %.1f%% (min %.1f%%)\n", float64(tenths)/10, *min)
}

func splitSkip(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// run writes the badge json before the gate check so a failed gate still
// publishes the truthful coverage color. Coverage is kept in integer tenths
// so message, color, and gate always agree.
func run(profile, out string, min float64, skip []string) (int, error) {
	if min <= 0 {
		return 0, fmt.Errorf("min must be > 0, got %v", min)
	}
	f, err := os.Open(profile)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()
	covered, total, err := parseProfile(f, skip)
	if err != nil {
		return 0, err
	}
	tenths := pctTenths(covered, total)
	b := badge{
		SchemaVersion: 1,
		Label:         "coverage",
		Message:       fmt.Sprintf("%.1f%%", float64(tenths)/10),
		Color:         colorFor(tenths),
	}
	body, err := json.Marshal(b)
	if err != nil {
		return tenths, err
	}
	if err := os.WriteFile(out, body, 0o644); err != nil {
		return tenths, err
	}
	if tenths < int(math.Round(min*10)) {
		return tenths, fmt.Errorf("coverage %.1f%% below minimum %.1f%%", float64(tenths)/10, min)
	}
	return tenths, nil
}

type badge struct {
	SchemaVersion int    `json:"schemaVersion"`
	Label         string `json:"label"`
	Message       string `json:"message"`
	Color         string `json:"color"`
}

func parseProfile(r io.Reader, skip []string) (covered, total int, err error) {
	sc := bufio.NewScanner(r)
	for n := 0; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return 0, 0, fmt.Errorf("profile line %d: want 3 fields, got %d", n+1, len(fields))
		}
		if skipPath(fields[0], skip) {
			continue
		}
		stmts, err := strconv.Atoi(fields[1])
		if err != nil {
			return 0, 0, fmt.Errorf("profile line %d: statements: %w", n+1, err)
		}
		count, err := strconv.Atoi(fields[2])
		if err != nil {
			return 0, 0, fmt.Errorf("profile line %d: count: %w", n+1, err)
		}
		total += stmts
		if count > 0 {
			covered += stmts
		}
	}
	return covered, total, sc.Err()
}

func skipPath(path string, skip []string) bool {
	for _, s := range skip {
		if strings.Contains(path, s) {
			return true
		}
	}
	return false
}

func pctTenths(covered, total int) int {
	if total == 0 {
		return 0
	}
	return int(math.Round(float64(covered) * 1000 / float64(total)))
}

func colorFor(tenths int) string {
	switch {
	case tenths >= 900:
		return "blue"
	case tenths >= 800:
		return "green"
	case tenths >= 700:
		return "yellow"
	case tenths >= 600:
		return "orange"
	default:
		return "red"
	}
}
