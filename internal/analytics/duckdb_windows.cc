//go:build windows && amd64

// The duckdb-go prebuilt static libs are compiled by an older mingw-w64 whose
// libstdc++ kept basic_streambuf<char>::seekpos as an out-of-line function, so
// duckdb objects reference its mangled symbol directly. GCC 15 defines seekpos
// inline in the header and never emits that symbol, and on this toolchain
// pos_type is fpos<int> (mbstate_t is int) so a compiler-emitted copy would
// carry a different mangled name than the referenced fpos<_Mbstatet> one. The
// vtable slot this fills (a stream-seek on a non-seekable sink) is never
// invoked, so the definition only has to return pos_type(off_type(-1)):
// streamoff -1 in the first 8 bytes, zeroed state in the rest. The Win x64 ABI
// returns a 16-byte struct through a hidden sret pointer in RCX, this in RDX,
// pos_type by pointer (structs over 8 bytes pass by reference) and openmode by
// value.
extern "C" void* _ZNSt15basic_streambufIcSt11char_traitsIcEE7seekposESt4fposI9_MbstatetESt13_Ios_Openmode(
    void* ret, void* self, const void* pos, int mode) {
    long long* slots = static_cast<long long*>(ret);
    slots[0] = -1;
    slots[1] = 0;
    return ret;
}

// The duckdb objects also reference the plain __stdio_common_* UCRT kernels,
// which libucrt.a defines but this toolchain's default link spec (msvcrt) never
// names. -lucrt is ordered before the duckdb archives on the link line because
// cgo emits package LDFLAGS reverse-topologically, and GNU ld pulls archive
// members only for undefined symbols known at scan time, never rescanning. So
// the members defining these two functions would never be pulled: the forwarder
// definitions below carry the references instead. They transparently call the
// real kernels through their __imp_ IAT pointers, declared here so the
// references originate in this object, which is scanned before -lucrt. The
// signatures are the toolchain's own (stdio_s.h:36 and stdio.h:1064): on win64
// size_t is unsigned long long, _locale_t is void*, va_list is char*.
#include <cstddef>
extern "C" {
void* __imp___stdio_common_vsnprintf_s;
void* __imp___stdio_common_vswprintf;

int __stdio_common_vsnprintf_s(unsigned long long options, char* str, size_t len,
                               size_t maxCount, const char* format, void* locale, char* argList) {
    using Kernel = int (*)(unsigned long long, char*, size_t, size_t, const char*, void*, char*);
    return reinterpret_cast<Kernel>(__imp___stdio_common_vsnprintf_s)(
        options, str, len, maxCount, format, locale, argList);
}

int __stdio_common_vswprintf(unsigned long long options, wchar_t* str, size_t len,
                             const wchar_t* format, void* locale, char* argList) {
    using Kernel = int (*)(unsigned long long, wchar_t*, size_t, const wchar_t*, void*, char*);
    return reinterpret_cast<Kernel>(__imp___stdio_common_vswprintf)(
        options, str, len, format, locale, argList);
}
}
