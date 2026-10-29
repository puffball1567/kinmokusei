package cabi

import (
	"fmt"
	"go/token"
	"go/types"
	"strings"
)

// ValidateSymbol checks the shared Go/C/C++ namespace of an outgoing gateway.
// A C++ consumer must be able to include the generated extern-C header, too.
// Application/native-library symbol collisions still require an application
// prefix; compiler validation cannot know every library the caller links.
func ValidateSymbol(name string) error {
	if name == "" || !asciiLetter(name[0]) {
		return fmt.Errorf("C ABI symbol %q must start with an ASCII letter and contain only ASCII letters, digits, or '_'", name)
	}
	for index := 1; index < len(name); index++ {
		if !asciiLetter(name[index]) && (name[index] < '0' || name[index] > '9') && name[index] != '_' {
			return fmt.Errorf("C ABI symbol %q must start with an ASCII letter and contain only ASCII letters, digits, or '_'", name)
		}
	}
	switch {
	case token.Lookup(name).IsKeyword():
		return fmt.Errorf("C ABI symbol %q is a Go keyword and cannot be generated", name)
	case name == "main" || name == "init":
		return fmt.Errorf("C ABI symbol %q is reserved by the generated Go package", name)
	case types.Universe.Lookup(name) != nil:
		return fmt.Errorf("C ABI symbol %q is a Go predeclared identifier and cannot be generated", name)
	case name == "C" || name == "unsafe" || name == "syscall":
		return fmt.Errorf("C ABI symbol %q is reserved by the cgo gateway", name)
	case strings.HasPrefix(name, "KINMOKUSEI_"):
		return fmt.Errorf("C ABI symbol %q uses the reserved KINMOKUSEI_ metadata namespace", name)
	case strings.Contains(name, "__"):
		return fmt.Errorf("C ABI symbol %q contains a double underscore reserved by C++", name)
	case strings.HasPrefix(name, "x_cgo_") || name == "crosscall2":
		return fmt.Errorf("C ABI symbol %q is reserved by the cgo runtime", name)
	}
	if category := reservedSymbols[name]; category != "" {
		return fmt.Errorf("C ABI symbol %q is reserved by %s", name, category)
	}
	return nil
}

func asciiLetter(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

var reservedSymbols = func() map[string]string {
	names := make(map[string]string)
	add := func(category, words string) {
		for _, name := range strings.Fields(words) {
			names[name] = category
		}
	}
	add("C/C++ keywords", `
alignas alignof and and_eq asm auto bitand bitor bool break
case catch char char8_t char16_t char32_t class compl concept const consteval constexpr constinit const_cast
continue co_await co_return co_yield decltype default delete do double dynamic_cast else enum explicit export
extern false float for friend goto if inline int long mutable namespace new noexcept not not_eq nullptr
operator or or_eq private protected public register reinterpret_cast requires restrict return short
signed sizeof static static_assert static_cast struct switch template this thread_local throw
true try typedef typeid typename typeof typeof_unqual union unsigned using virtual void volatile wchar_t
while xor xor_eq`)
	add("cgo declarations", `
GoInt8 GoUint8 GoInt16 GoUint16 GoInt32 GoUint32 GoInt64 GoUint64 GoInt GoUint GoUintptr
GoFloat32 GoFloat64 GoComplex64 GoComplex128 GoString GoMap GoChan GoInterface GoSlice`)
	add("C++ standard namespace", "std")
	add("C standard headers", `
size_t ptrdiff_t wchar_t max_align_t intmax_t uintmax_t intptr_t uintptr_t NULL offsetof
INTMAX_MIN INTMAX_MAX UINTMAX_MAX INTMAX_C UINTMAX_C INTPTR_MIN INTPTR_MAX UINTPTR_MAX
PTRDIFF_MIN PTRDIFF_MAX SIZE_MAX SIG_ATOMIC_MIN SIG_ATOMIC_MAX WCHAR_MIN WCHAR_MAX WINT_MIN WINT_MAX
INTMAX_WIDTH UINTMAX_WIDTH INTPTR_WIDTH UINTPTR_WIDTH PTRDIFF_WIDTH SIZE_WIDTH SIG_ATOMIC_WIDTH WCHAR_WIDTH WINT_WIDTH`)
	for _, width := range []string{"8", "16", "32", "64"} {
		for _, family := range []string{"int", "uint", "int_least", "uint_least", "int_fast", "uint_fast"} {
			names[family+width+"_t"] = "C standard integer headers"
		}
		for _, family := range []string{"INT", "UINT", "INT_LEAST", "UINT_LEAST", "INT_FAST", "UINT_FAST"} {
			for _, suffix := range []string{"_MAX", "_WIDTH"} {
				names[family+width+suffix] = "C standard integer headers"
			}
			if strings.HasPrefix(family, "INT") {
				names[family+width+"_MIN"] = "C standard integer headers"
			}
		}
		names["INT"+width+"_C"] = "C standard integer headers"
		names["UINT"+width+"_C"] = "C standard integer headers"
	}
	// cgo's export implementation includes stdlib.h. Reserve its public
	// names and common libc entry points, rather than defining replacements
	// with gateway status/out signatures that can interpose on libc calls.
	add("C standard library declarations", `
abort abs aligned_alloc at_quick_exit atexit atof atoi atol atoll bsearch calloc div exit free getenv
labs ldiv llabs lldiv malloc mblen mbstowcs mbtowc qsort quick_exit rand realloc srand strtod strtof
strtol strtold strtoll strtoul strtoull system wcstombs wctomb div_t ldiv_t lldiv_t
EXIT_FAILURE EXIT_SUCCESS MB_CUR_MAX RAND_MAX
clearerr fclose feof ferror fflush fgetc fgetpos fgets fopen fprintf fputc fputs fread freopen fscanf
fseek fsetpos ftell fwrite getc getchar perror printf putc putchar puts remove rename rewind scanf
setbuf setvbuf snprintf sprintf sscanf tmpfile tmpnam ungetc vfprintf vfscanf vprintf vscanf vsnprintf
vsprintf vsscanf FILE fpos_t stdin stdout stderr EOF FILENAME_MAX FOPEN_MAX L_tmpnam SEEK_CUR SEEK_END SEEK_SET
BUFSIZ TMP_MAX memchr memcmp memcpy memmove memset strcat strchr strcmp strcoll strcpy strcspn strerror
strlen strncat strncmp strncpy strpbrk strrchr strspn strstr strtok strxfrm errno`)
	return names
}()
