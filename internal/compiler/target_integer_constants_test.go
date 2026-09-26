package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/project"
)

func targetConstantsProject(t *testing.T, goos, arch string) string {
	t.Helper()
	root := t.TempDir()
	manifest := fmt.Sprintf("[project]\nname=\"target-constants\"\nversion=\"0.1.0\"\ngo-module=\"target-constants.test\"\ngo-version=\"1.23\"\n[target]\ngoos=\"%s\"\ngoarch=\"%s\"\ncgo=\"disabled\"\n", goos, arch)
	for name, text := range map[string]string{"kinmokusei.toml": manifest, "go.mod": "module target-constants.test\n\ngo 1.23\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := project.LockDependencies(root, true); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestLockedTargetIntegerDiagnostics(t *testing.T) {
	// Check does not build a binary: no 32-bit runtime/toolchain is needed.
	for _, arch := range []string{"386", "amd64"} {
		t.Run(arch, func(t *testing.T) {
			root := targetConstantsProject(t, "linux", arch)
			other := "386"
			if arch == "386" {
				other = "amd64"
			}
			t.Setenv("GOARCH", other)
			path := filepath.Join(root, "entry.km")
			library := filepath.Join(root, "bounds.km")
			if err := os.WriteFile(library, []byte(`export const limit=2147483648;`), 0o644); err != nil {
				t.Fatal(err)
			}
			input := `import {limit as count} from "./bounds";function run():int[]{return make[int[]](count);}`
			result, err := CheckFilesWithOverlay([]string{path}, map[string]string{path: input})
			if err != nil {
				t.Fatal(err)
			}
			if (len(result.Diagnostics) == 0) != (arch == "amd64") {
				t.Fatalf("%s: %v", arch, result.Diagnostics)
			}
			for _, d := range result.Diagnostics {
				if d.Span.Path != path {
					t.Fatalf("non-source diagnostic: %v", d)
				}
			}
			if err := os.WriteFile(path, []byte(`const mask=^uint(0);export function Run():uint32{return uint32(mask);}`), 0o644); err != nil {
				t.Fatal(err)
			}
			_, diagnostics, err := EmitGo([]string{path}, "targetconstants")
			if err != nil {
				t.Fatalf("generated Go validation used wrong target: %v", err)
			}
			if (len(diagnostics) == 0) != (arch == "386") {
				t.Fatalf("complement %s: %v", arch, diagnostics)
			}
		})
	}
}

func TestTargetIntegerConstantsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := targetConstantsProject(t, runtime.GOOS, runtime.GOARCH)
	input := `export const Max=^uint(0);
export function Width():int{return int(Max>>1);}
export function Shift(n:uint):uint64{return 4294967296<<n;}
export function Small():int{const large=1<<80;return large>>80;}
export function Mask():uint32{return uint32(Max & 4294967295);}
export function Generic<T extends comparable>(x:T,y:T):boolean{return x==y;}`
	path := filepath.Join(root, "entry.km")
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "targetconstants")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
const Max=^uint(0)
func Width()int{return int(Max>>1)}
func Shift(n uint)uint64{return 4294967296<<n}
func Small()int{const large=1<<80;return large>>80}
func Mask()uint32{return uint32(Max & 4294967295)}
func Generic[T comparable](x,y T)bool{return x==y}`
	comparison := `package targetconstants_test
import("testing";g "target-constants.test";r "target-constants.test/reference")
func TestValues(t *testing.T){if g.Width()!=r.Width()||g.Small()!=r.Small()||g.Mask()!=r.Mask()||g.Generic(42,42)!=r.Generic(42,42){t.Fatal("target constants")};for _,n:=range []uint{0,1,31,32}{if g.Shift(n)!=r.Shift(n){t.Fatal("shift",n)}}}`
	runGeneratedGoDifferentialTest(t, root, "target-constants.test", generated, reference, comparison)
}
