package domain

import "math"

// WilsonLB is the wilson score interval 95 percent lower bound, the floor a win rate is
// trusted at given its sample size. deviation 2: computed in pure Go over SQL-assembled
// counts, the analytics batch aggregates k and n in the database and applies this here,
// keeping the interval arithmetic out of both sqlite and duckdb.
func WilsonLB(k, n int) float64 {
	if n == 0 {
		return 0
	}
	if k == 0 {
		// The lower endpoint is exactly 0 by the closed form, floating point leaves dust above it.
		return 0
	}
	const z = 1.96
	p := float64(k) / float64(n)
	z2 := z * z
	denom := 1 + z2/float64(n)
	center := p + z2/(2*float64(n))
	radius := z * math.Sqrt(p*(1-p)/float64(n)+z2/(4*float64(n)*float64(n)))
	return math.Max(0, (center-radius)/denom)
}
