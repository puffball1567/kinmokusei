package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/diagnostic"
	"github.com/puffball1567/kinmokusei/internal/product"
	"github.com/puffball1567/kinmokusei/internal/project"
)

func TestManifestLockedGoDependencyCompilesAndRunsOffline(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "library")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `[project]
name = "application"
version = "0.1.0"
go-module = "example.com/application"
go-version = "1.23"
`
	if err := os.WriteFile(filepath.Join(root, product.ProjectFileName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "go.mod"), []byte("module example.com/library\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "library.go"), []byte("package library\ntype ID int\nfunc Value() ID { return 42 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "main.km")
	if err := os.WriteFile(source, []byte(`import go library from "example.com/library"; function value(): library.ID { return library.Value(); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := project.AddDependency(root, "example.com/library", "v0.0.0", "./library", true); err != nil {
		t.Fatal(err)
	}
	generatedDirectory, diagnostics, err := WriteGeneratedModule([]string{source}, "manifestapp")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("directory=%q diagnostics=%v err=%v", generatedDirectory, diagnostics, err)
	}
	goMod, err := os.ReadFile(filepath.Join(generatedDirectory, "go.mod"))
	if err != nil || !strings.Contains(string(goMod), "require (\n\texample.com/library v0.0.0") || !strings.Contains(string(goMod), "replace example.com/library => ../../library") {
		t.Fatalf("generated go.mod err=%v:\n%s", err, goMod)
	}
	generated, err := os.ReadFile(filepath.Join(generatedDirectory, "generated.go"))
	if err != nil {
		t.Fatal(err)
	}
	referenceSource := `package reference
import library "example.com/library"
func Value() library.ID { return library.Value() }
`
	testSource := `package manifestapp
import (
  "testing"
  reference "example.com/application/reference"
)
func TestValue(t *testing.T) {
  if got, want := value(), reference.Value(); got != want { t.Errorf("value = %d, Go = %d", got, want) }
}
`
	runGeneratedGoDifferentialTestInExistingModule(t, generatedDirectory, "example.com/application", generated, referenceSource, testSource,
		[]string{"test", "-mod=readonly", "./..."}, []string{"GOPROXY=off"})
	lockedManifest, err := os.ReadFile(filepath.Join(root, product.ProjectFileName))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, product.ProjectFileName), append(lockedManifest, []byte("\n# stale\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckFiles([]string{source}); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("stale manifest err = %v", err)
	}
}

func TestManifestProjectRequiresLockBeforeCompilation(t *testing.T) {
	root := t.TempDir()
	manifest := `[project]
name = "missing-lock"
version = "0.1.0"
go-module = "example.com/missing-lock"
go-version = "1.23"
`
	if err := os.WriteFile(filepath.Join(root, product.ProjectFileName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "main.km")
	if err := os.WriteFile(source, []byte(`function value(): int { return 42; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckFiles([]string{source}); err == nil || !strings.Contains(err.Error(), "deps lock") {
		t.Fatalf("missing lock err = %v", err)
	}
}

func TestManifestBuildTagsSelectGoPackageAPI(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "library")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `[project]
name = "tagged-application"
version = "0.1.0"
go-module = "example.com/tagged-application"
go-version = "1.23"
`
	manifestPath := filepath.Join(root, product.ProjectFileName)
	for path, contents := range map[string]string{
		manifestPath:                          manifest,
		filepath.Join(library, "go.mod"):      "module example.com/tagged-library\n\ngo 1.23\n",
		filepath.Join(library, "base.go"):     "package library\n",
		filepath.Join(library, "tagged.go"):   "//go:build kinmokusei_special\n\npackage library\nfunc Tagged() int { return 42 }\n",
		filepath.Join(library, "untagged.go"): "//go:build !kinmokusei_special\n\npackage library\nfunc Untagged() int { return 0 }\n",
	} {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	source := filepath.Join(root, "main.km")
	if err := os.WriteFile(source, []byte(`import go library from "example.com/tagged-library"; function value(): int { return library.Tagged(); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := project.AddDependency(root, "example.com/tagged-library", "v0.0.0", "./library", true); err != nil {
		t.Fatal(err)
	}
	withoutTag, err := CheckFiles([]string{source})
	if err != nil || len(withoutTag.Diagnostics) == 0 {
		t.Fatalf("without tag diagnostics=%v err=%v", withoutTag.Diagnostics, err)
	}
	canonical, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	canonical = append(canonical, []byte("\n[target]\ncgo = \"disabled\"\ntags = \"kinmokusei_special\"\n")...)
	if err = os.WriteFile(manifestPath, canonical, 0o644); err != nil {
		t.Fatal(err)
	}
	lock, err := project.LockDependencies(root, true)
	if err != nil {
		t.Fatal(err)
	}
	generatedDirectory, diagnostics, err := WriteGeneratedModule([]string{source}, "taggedapp")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("directory=%q diagnostics=%v err=%v", generatedDirectory, diagnostics, err)
	}
	generated, err := os.ReadFile(filepath.Join(generatedDirectory, "generated.go"))
	if err != nil {
		t.Fatal(err)
	}
	referenceSource := `package reference
import library "example.com/tagged-library"
func Value() int { return library.Tagged() }
`
	testSource := `package taggedapp
import (
  "testing"
  reference "example.com/tagged-application/reference"
)
func TestTagged(t *testing.T) {
  if got, want := value(), reference.Value(); got != want { t.Errorf("value = %d, Go = %d", got, want) }
}
`
	arguments := append([]string{"test"}, lock.Target.GoBuildFlags()...)
	arguments = append(arguments, "-mod=readonly", "./...")
	runGeneratedGoDifferentialTestInExistingModule(t, generatedDirectory, "example.com/tagged-application", generated, referenceSource, testSource,
		arguments, lock.Target.Environment([]string{"GOPROXY=off"}))
}

func TestManifestCGOTargetMatrix(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "cfixture")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `[project]
name = "cgo-application"
version = "0.1.0"
go-module = "example.com/cgo-application"
go-version = "1.23"

[target]
cgo = "disabled"

[go.dependencies]
"example.com/cfixture" = "v0.0.0"

[go.replacements]
"example.com/cfixture" = "cfixture"
`
	cgoSource := `package cfixture
/*
static int kinmokusei_answer(void) { return 42; }
*/
import "C"
func Answer() int { return int(C.kinmokusei_answer()) }
`
	for path, contents := range map[string]string{
		filepath.Join(root, product.ProjectFileName): manifest,
		filepath.Join(library, "go.mod"):             "module example.com/cfixture\n\ngo 1.23\n",
		filepath.Join(library, "cfixture.go"):        cgoSource,
	} {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	source := filepath.Join(root, "main.km")
	if err := os.WriteFile(source, []byte(`import go cfixture from "example.com/cfixture"; function value(): int { return cfixture.Answer(); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := project.LockDependencies(root, true); err != nil {
		t.Fatal(err)
	}
	disabled, err := CheckFiles([]string{source})
	if err != nil || !strings.Contains(diagnosticsText(disabled.Diagnostics), "requires cgo") || !strings.Contains(diagnosticsText(disabled.Diagnostics), "CGO_ENABLED=0") {
		t.Fatalf("cgo disabled diagnostics=%v err=%v", disabled.Diagnostics, err)
	}
	ccOutput, err := exec.Command("go", "env", "CC").Output()
	if err != nil {
		t.Skipf("cannot query C compiler: %v", err)
	}
	cc := strings.TrimSpace(string(ccOutput))
	if _, err = exec.LookPath(cc); err != nil {
		t.Skipf("C compiler %q is unavailable", cc)
	}
	manifest = strings.Replace(manifest, `cgo = "disabled"`, `cgo = "enabled"`, 1)
	if err = os.WriteFile(filepath.Join(root, product.ProjectFileName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	lock, err := project.LockDependencies(root, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CC", "kinmokusei-c-compiler-does-not-exist")
	missingCompiler, err := CheckFiles([]string{source})
	if err != nil || !strings.Contains(diagnosticsText(missingCompiler.Diagnostics), "requires a working C toolchain") || !strings.Contains(diagnosticsText(missingCompiler.Diagnostics), "CGO_ENABLED=1") {
		t.Fatalf("missing C compiler diagnostics=%v err=%v", missingCompiler.Diagnostics, err)
	}
	t.Setenv("CC", cc)
	generatedDirectory, diagnostics, err := WriteGeneratedModule([]string{source}, "cgoapp")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("directory=%q diagnostics=%v err=%v", generatedDirectory, diagnostics, err)
	}
	generated, err := os.ReadFile(filepath.Join(generatedDirectory, "generated.go"))
	if err != nil {
		t.Fatal(err)
	}
	referenceSource := `package reference
import cfixture "example.com/cfixture"
func Value() int { return cfixture.Answer() }
`
	testSource := `package cgoapp
import (
  "testing"
  reference "example.com/cgo-application/reference"
)
func TestCGO(t *testing.T) {
  if got, want := value(), reference.Value(); got != want { t.Errorf("value = %d, Go = %d", got, want) }
}
`
	arguments := append([]string{"test"}, lock.Target.GoBuildFlags()...)
	arguments = append(arguments, "-mod=readonly", "./...")
	runGeneratedGoDifferentialTestInExistingModule(t, generatedDirectory, "example.com/cgo-application", generated, referenceSource, testSource,
		arguments, lock.Target.Environment([]string{"GOPROXY=off", "CC=" + cc}))
}

func TestManifestUnsafeGoInteropPolicyCompileAndRun(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "unsafeapi")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `[project]
name = "unsafe-application"
version = "0.1.0"
go-module = "example.com/unsafe-application"
go-version = "1.23"

[go.dependencies]
"example.com/unsafeapi" = "v0.0.0"

[go.replacements]
"example.com/unsafeapi" = "unsafeapi"
`
	apiSource := `package unsafeapi
import "unsafe"
func Pointer() unsafe.Pointer { return nil }
func Apply(callback func([]unsafe.Pointer) map[string]unsafe.Pointer) {}
type Box struct { Value unsafe.Pointer }
func (Box) Method() unsafe.Pointer { return nil }
`
	manifestPath := filepath.Join(root, product.ProjectFileName)
	for path, contents := range map[string]string{
		manifestPath:                               manifest,
		filepath.Join(library, "go.mod"):           "module example.com/unsafeapi\n\ngo 1.23\n",
		filepath.Join(library, "unsafeapi.go"):     apiSource,
		filepath.Join(library, "safe_internal.go"): "package unsafeapi\nimport \"unsafe\"\nvar _ = unsafe.Sizeof(0)\nfunc Safe() int { return 42 }\n",
	} {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	source := filepath.Join(root, "main.km")
	deniedSource := `import go api from "example.com/unsafeapi";
import go unsafe from "unsafe";
function pointerIsNil(): boolean { return api.Pointer() == nil; }
function fieldIsNil(box: api.Box): boolean { return box.Value == nil; }
function methodIsNil(box: api.Box): boolean { return box.Method() == nil; }
function nestedSignature(): void { api.Apply((values: unsafe.Pointer[]): Map<string, unsafe.Pointer> => { return makeMap[string, unsafe.Pointer](); }); }
function safe(): int { return api.Safe(); }`
	if err := os.WriteFile(source, []byte(deniedSource), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := project.LockDependencies(root, true); err != nil {
		t.Fatal(err)
	}
	denied, err := CheckFiles([]string{source})
	joined := diagnosticsText(denied.Diagnostics)
	if err != nil || !strings.Contains(joined, "uses unsafe.Pointer") || strings.Contains(joined, "api.Safe") {
		t.Fatalf("deny diagnostics=%v err=%v", denied.Diagnostics, err)
	}
	manifest += "\n[go.interop]\nunsafe = \"allow\"\n"
	if err = os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err = project.LockDependencies(root, true); err != nil {
		t.Fatal(err)
	}
	directSource := filepath.Join(root, "direct.km")
	if err = os.WriteFile(directSource, []byte(`import go unsafe from "unsafe"; function identity(value: unsafe.Pointer): unsafe.Pointer { return value; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	generatedDirectory, diagnostics, err := WriteGeneratedModule([]string{source, directSource}, "unsafeapp")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("directory=%q diagnostics=%v err=%v", generatedDirectory, diagnostics, err)
	}
	generated, err := os.ReadFile(filepath.Join(generatedDirectory, "generated.go"))
	if err != nil {
		t.Fatal(err)
	}
	referenceSource := `package reference
import (
  api "example.com/unsafeapi"
  "unsafe"
)
func PointerIsNil() bool { return api.Pointer() == nil }
func FieldIsNil(box api.Box) bool { return box.Value == nil }
func MethodIsNil(box api.Box) bool { return box.Method() == nil }
func NestedSignature() { api.Apply(func(values []unsafe.Pointer) map[string]unsafe.Pointer { return make(map[string]unsafe.Pointer) }) }
func Safe() int { return api.Safe() }
func Identity(value unsafe.Pointer) unsafe.Pointer { return value }
`
	testSource := `package unsafeapp
import (
  "testing"
  api "example.com/unsafeapi"
  reference "example.com/unsafe-application/reference"
)
func TestUnsafePolicy(t *testing.T) {
  if got, want := pointerIsNil(), reference.PointerIsNil(); got != want { t.Errorf("pointerIsNil = %v, Go = %v", got, want) }
  if got, want := fieldIsNil(api.Box{}), reference.FieldIsNil(api.Box{}); got != want { t.Errorf("fieldIsNil = %v, Go = %v", got, want) }
  if got, want := methodIsNil(api.Box{}), reference.MethodIsNil(api.Box{}); got != want { t.Errorf("methodIsNil = %v, Go = %v", got, want) }
  if got, want := identity(nil), reference.Identity(nil); got != want { t.Errorf("identity = %v, Go = %v", got, want) }
  if got, want := safe(), reference.Safe(); got != want { t.Errorf("safe = %v, Go = %v", got, want) }
  nestedSignature()
  reference.NestedSignature()
}
`
	runGeneratedGoDifferentialTestInExistingModule(t, generatedDirectory, "example.com/unsafe-application", generated, referenceSource, testSource,
		[]string{"test", "-mod=readonly", "./..."}, []string{"GOPROXY=off"})
}

func TestManifestUnsafeBuiltinsCompileRunAndPanicMatrix(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "layoutapi")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `[project]
name = "unsafe-builtins"
version = "0.1.0"
go-module = "example.com/unsafe-builtins"
go-version = "1.23"

[go.dependencies]
"example.com/layoutapi" = "v0.0.0"

[go.replacements]
"example.com/layoutapi" = "layoutapi"

[go.interop]
unsafe = "allow"
`
	for path, contents := range map[string]string{
		filepath.Join(root, product.ProjectFileName): manifest,
		filepath.Join(library, "go.mod"):             "module example.com/layoutapi\n\ngo 1.23\n",
		filepath.Join(library, "layout.go"):          "package layoutapi\ntype Layout struct { A byte; B int64 }\ntype BytePointer *byte\ntype Bytes []byte\ntype Inner struct { C int }\ntype Embedded struct { *Inner }\ntype EmbeddedValue struct { Inner }\n",
	} {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	source := filepath.Join(root, "main.km")
	kinmokuseiSource := `import go layout from "example.com/layoutapi";
import go danger from "unsafe";
function size(value: int64): int { return int(danger.Sizeof(value)); }
function alignment(value: int64): int { return int(danger.Alignof(value)); }
function fieldOffset(value: layout.Layout): int { return int(danger.Offsetof(value.B)); }
function pointerFieldOffset(value: *layout.Layout): int { return int(danger.Offsetof(value.B)); }
function promotedFieldOffset(value: layout.EmbeddedValue): int { return int(danger.Offsetof(value.C)); }
function add(pointer: danger.Pointer, offset: int): danger.Pointer { return danger.Add(pointer, offset); }
function bytes(pointer: *byte, length: int): byte[] { return danger.Slice(pointer, length); }
function namedBytes(pointer: layout.BytePointer, length: int): byte[] { return danger.Slice(pointer, length); }
function first(values: byte[]): *byte { return danger.SliceData(values); }
function namedFirst(values: layout.Bytes): *byte { return danger.SliceData(values); }
function text(pointer: *byte, length: int): string { return danger.String(pointer, length); }
function namedText(pointer: layout.BytePointer, length: int): string { return danger.String(pointer, length); }
function textData(value: string): *byte { return danger.StringData(value); }
`
	if err := os.WriteFile(source, []byte(kinmokuseiSource), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := project.LockDependencies(root, true); err != nil {
		t.Fatal(err)
	}
	invalidSource := filepath.Join(root, "invalid.km")
	if err := os.WriteFile(invalidSource, []byte(`import go layout from "example.com/layoutapi"; import go danger from "unsafe"; function invalid(value: layout.Embedded): int { return int(danger.Offsetof(value.C)); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	invalidResult, err := CheckFiles([]string{invalidSource})
	if err != nil || !strings.Contains(diagnosticsText(invalidResult.Diagnostics), "field cannot be embedded through a pointer") {
		t.Fatalf("embedded pointer diagnostics=%v err=%v", invalidResult.Diagnostics, err)
	}
	generatedDirectory, diagnostics, err := WriteGeneratedModule([]string{source}, "unsafebuiltins")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("directory=%q diagnostics=%v err=%v", generatedDirectory, diagnostics, err)
	}
	generated, err := os.ReadFile(filepath.Join(generatedDirectory, "generated.go"))
	if err != nil {
		t.Fatal(err)
	}
	referenceSource := `package reference
import (
  layout "example.com/layoutapi"
  "unsafe"
)
func Size(value int64) int { return int(unsafe.Sizeof(value)) }
func Alignment(value int64) int { return int(unsafe.Alignof(value)) }
func FieldOffset(value layout.Layout) int { return int(unsafe.Offsetof(value.B)) }
func PointerFieldOffset(value *layout.Layout) int { return int(unsafe.Offsetof(value.B)) }
func PromotedFieldOffset(value layout.EmbeddedValue) int { return int(unsafe.Offsetof(value.C)) }
func Add(pointer unsafe.Pointer, offset int) unsafe.Pointer { return unsafe.Add(pointer, offset) }
func Bytes(pointer *byte, length int) []byte { return unsafe.Slice(pointer, length) }
func NamedBytes(pointer layout.BytePointer, length int) []byte { return unsafe.Slice(pointer, length) }
func First(values []byte) *byte { return unsafe.SliceData(values) }
func NamedFirst(values layout.Bytes) *byte { return unsafe.SliceData(values) }
func Text(pointer *byte, length int) string { return unsafe.String(pointer, length) }
func NamedText(pointer layout.BytePointer, length int) string { return unsafe.String(pointer, length) }
func TextData(value string) *byte { return unsafe.StringData(value) }
`
	testSource := `package unsafebuiltins
import (
  stdbytes "bytes"
  "os"
  "testing"
  "unsafe"
  layout "example.com/layoutapi"
  reference "example.com/unsafe-builtins/reference"
)
func didPanic(operation func()) (panicked bool) { defer func() { panicked = recover() != nil }(); operation(); return false }
func TestUnsafeBuiltinResults(t *testing.T) {
  for _, value := range []int64{-1, 0, 1, 1<<40} {
    if got, want := size(value), reference.Size(value); got != want { t.Errorf("size(%d)=%d Go=%d", value, got, want) }
    if got, want := alignment(value), reference.Alignment(value); got != want { t.Errorf("alignment(%d)=%d Go=%d", value, got, want) }
  }
  if got, want := fieldOffset(layout.Layout{}), reference.FieldOffset(layout.Layout{}); got != want { t.Errorf("offset=%d Go=%d", got, want) }
  item := layout.Layout{}
  if got, want := pointerFieldOffset(&item), reference.PointerFieldOffset(&item); got != want { t.Errorf("pointer offset=%d Go=%d", got, want) }
  if got, want := promotedFieldOffset(layout.EmbeddedValue{}), reference.PromotedFieldOffset(layout.EmbeddedValue{}); got != want { t.Errorf("promoted offset=%d Go=%d", got, want) }
  generatedData, goData := []byte{'A', 'B', 'C'}, []byte{'A', 'B', 'C'}
  for _, item := range []struct{ index, offset int }{{0, 1}, {1, -1}} {
    got := uintptr(add(unsafe.Pointer(&generatedData[item.index]), item.offset))-uintptr(unsafe.Pointer(&generatedData[0]))
    want := uintptr(reference.Add(unsafe.Pointer(&goData[item.index]), item.offset))-uintptr(unsafe.Pointer(&goData[0]))
    if got != want { t.Errorf("add(%d,%d) offset=%d Go=%d", item.index, item.offset, got, want) }
  }
  generatedView := bytes(&generatedData[0], len(generatedData))
  goView := reference.Bytes(&goData[0], len(goData))
  generatedView[1], goView[1] = 'Z', 'Z'
  if !stdbytes.Equal(generatedData, goData) { t.Errorf("slice alias=%q Go=%q", generatedData, goData) }
  if got, want := first(generatedView) == &generatedData[0], reference.First(goView) == &goData[0]; got != want || !got { t.Errorf("slice data=%v Go=%v", got, want) }
  if got, want := first(nil) == nil, reference.First(nil) == nil; got != want { t.Errorf("nil slice data=%v Go=%v", got, want) }
  if got, want := bytes(nil, 0) == nil, reference.Bytes(nil, 0) == nil; got != want { t.Errorf("nil empty slice=%v Go=%v", got, want) }
  if got, want := text(&generatedData[0], len(generatedData)), reference.Text(&goData[0], len(goData)); got != want { t.Errorf("string=%q Go=%q", got, want) }
  if got, want := namedBytes(layout.BytePointer(&generatedData[0]), len(generatedData)), reference.NamedBytes(layout.BytePointer(&goData[0]), len(goData)); !stdbytes.Equal(got, want) { t.Errorf("named slice=%q Go=%q", got, want) }
  if got, want := namedFirst(layout.Bytes(generatedData)) == &generatedData[0], reference.NamedFirst(layout.Bytes(goData)) == &goData[0]; got != want || !got { t.Errorf("named slice data=%v Go=%v", got, want) }
  if got, want := namedText(layout.BytePointer(&generatedData[0]), len(generatedData)), reference.NamedText(layout.BytePointer(&goData[0]), len(goData)); got != want { t.Errorf("named string=%q Go=%q", got, want) }
  if got, want := add(nil, 0) == nil, reference.Add(nil, 0) == nil; got != want { t.Errorf("nil add=%v Go=%v", got, want) }
  if got, want := text(nil, 0), reference.Text(nil, 0); got != want { t.Errorf("nil empty string=%q Go=%q", got, want) }
  for _, value := range []string{"ok", "温泉"} {
    got, want := textData(value), reference.TextData(value)
    if (got == nil) != (want == nil) || got == nil || *got != *want { t.Errorf("string data(%q)=%v Go=%v", value, got, want) }
  }
}
func TestUnsafeBuiltinPanics(t *testing.T) {
  if os.Getenv("KINMOKUSEI_DIFFERENTIAL_RACE") == "1" { t.Skip("race checkptr turns these documented panics into unrecoverable process failures") }
  if got, want := didPanic(func() { _ = bytes(nil, 1) }), didPanic(func() { _ = reference.Bytes(nil, 1) }); got != want || !got { t.Errorf("slice panic=%v Go=%v", got, want) }
  if got, want := didPanic(func() { _ = text(nil, 1) }), didPanic(func() { _ = reference.Text(nil, 1) }); got != want || !got { t.Errorf("string panic=%v Go=%v", got, want) }
}
`
	runGeneratedGoDifferentialTestInExistingModule(t, generatedDirectory, "example.com/unsafe-builtins", generated, referenceSource, testSource,
		[]string{"test", "-mod=readonly", "./..."}, []string{"GOPROXY=off"})
}

func diagnosticsText(diagnostics []diagnostic.Diagnostic) string {
	messages := make([]string, len(diagnostics))
	for index, item := range diagnostics {
		messages[index] = item.Message
	}
	return strings.Join(messages, "\n")
}
