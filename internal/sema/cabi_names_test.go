package sema

import (
	"fmt"
	"strings"
	"testing"
)

func TestCABIExportRejectsBoundaryNameCollisions(t *testing.T) {
	for _, name := range []string{
		"double", "volatile", "alignas", "thread_local", "class", "namespace", "operator", "co_await", "std",
		"int32_t", "uint_fast64_t", "int_least8_t", "intmax_t", "uintptr_t", "size_t", "ptrdiff_t",
		"INT32_MAX", "INT64_C", "UINTMAX_C", "SIZE_MAX", "NULL",
		"GoString", "GoInterface", "GoSlice", "GoInt32", "GoUintptr",
		"malloc", "free", "abs", "exit", "printf", "memcpy",
		"recover", "nil", "int32", "bool", "any", "true", "error", "C", "unsafe", "syscall", "crosscall2", "x_cgo_init",
		"KINMOKUSEI_ABI_OK", "KINMOKUSEI_GENERATED_ABI_H", "KINMOKUSEI_FUTURE", "user__reserved",
	} {
		t.Run(name, func(t *testing.T) {
			source := fmt.Sprintf(`function impl(v: int32): int32 { return v; } export c(%q) {impl};`, name)
			if diagnostics := checkSource(t, source); len(diagnostics) == 0 || !strings.Contains(strings.Join(diagnostics, "\n"), `C ABI symbol "`+name+`"`) {
				t.Fatalf("missing boundary diagnostic for %q: %v", name, diagnostics)
			}
		})
	}
}

func TestCABIExportExceptionRuntimeName(t *testing.T) {
	for _, name := range []string{"Exception", "NewException"} {
		source := fmt.Sprintf(`function impl(): int32 { throw new Exception("failure"); } export c(%q) {impl};`, name)
		if diagnostics := checkSource(t, source); !strings.Contains(strings.Join(diagnostics, "\n"), "reserved by the exception runtime") {
			t.Fatalf("name=%s diagnostics=%v", name, diagnostics)
		}
	}
}

func TestCABIExportNeighboringNamesRemainValid(t *testing.T) {
	for _, name := range []string{"k_malloc", "GoMyThing", "INT32_MAX_value", "Exception", "NewException", "recover_value", "outResult", "arg0"} {
		source := fmt.Sprintf(`function impl(v: int32): int32 { return v; } export c(%q) {impl};`, name)
		if diagnostics := checkSource(t, source); len(diagnostics) != 0 {
			t.Fatalf("name=%s diagnostics=%v", name, diagnostics)
		}
	}
}

func TestCABIImplicitImportDoesNotCollideWithPackageDeclarations(t *testing.T) {
	for _, declaration := range []string{
		`function C(): void {}`,
		`class C {}`,
		`function syscall(): void {}`,
		`function _cgopackage(): void {}`,
	} {
		source := declaration + ` function impl(): void {} export c("k_impl") {impl};`
		if diagnostics := checkSource(t, source); !strings.Contains(strings.Join(diagnostics, "\n"), "reserved by the cgo gateway") {
			t.Fatalf("source=%s diagnostics=%v", source, diagnostics)
		}
	}
}
