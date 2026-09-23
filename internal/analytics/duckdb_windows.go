//go:build windows && amd64

// The cgo import compiles duckdb_windows.cc and pulls -lucrt into the link: the
// duckdb prebuilts import UCRT symbols (_timezone, _tzname and the plain
// __stdio_common_* kernels), but the mingw toolchain defaults to the msvcrt
// link spec, so libucrt.a must be named explicitly.
package analytics

/*
#cgo windows LDFLAGS: -lucrt
*/
import "C"
