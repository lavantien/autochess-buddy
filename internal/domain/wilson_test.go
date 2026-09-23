package domain

import (
	"math"
	"testing"

	"pgregory.net/rapid"
)

func TestWilsonLB_ZeroN(t *testing.T) {
	for _, c := range [][2]int{{0, 0}, {3, 0}, {10, 0}} {
		if got := WilsonLB(c[0], c[1]); got != 0 {
			t.Errorf("WilsonLB(%d, %d) = %g, want 0", c[0], c[1], got)
		}
	}
}

func TestWilsonLB_Extremes(t *testing.T) {
	if got := WilsonLB(0, 10); got != 0 {
		t.Errorf("WilsonLB(0, 10) = %g, want 0", got)
	}
	perfect := WilsonLB(10, 10)
	if perfect <= 0 || perfect >= 1 {
		t.Errorf("WilsonLB(10, 10) = %g, want in (0, 1)", perfect)
	}
	// golden: hand-computed wilson lower bound for 7 of 10, z = 1.96
	got := WilsonLB(7, 10)
	if math.Abs(got-0.3968) > 0.0005 {
		t.Errorf("WilsonLB(7, 10) = %g, want 0.3968 within 0.0005", got)
	}
}

func TestWilsonLB_NeverExceedsObservedRate(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 10000).Draw(t, "sample size")
		k := rapid.IntRange(0, n).Draw(t, "successes")
		lb := WilsonLB(k, n)
		if lb < 0 {
			t.Fatalf("WilsonLB(%d, %d) = %g, want >= 0", k, n, lb)
		}
		if lb > float64(k)/float64(n) {
			t.Fatalf("WilsonLB(%d, %d) = %g, want <= observed rate %g", k, n, lb, float64(k)/float64(n))
		}
	})
}

func TestWilsonLB_MonotoneInNAtFixedRate(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n1 := rapid.IntRange(1, 500).Draw(t, "small n")
		m := rapid.IntRange(2, 8).Draw(t, "multiple")
		k1 := rapid.IntRange(0, n1).Draw(t, "successes")
		k2, n2 := k1*m, n1*m
		if WilsonLB(k2, n2) < WilsonLB(k1, n1) {
			t.Fatalf("bound fell as n grew at fixed rate: WilsonLB(%d, %d) < WilsonLB(%d, %d)", k2, n2, k1, n1)
		}
	})
}
