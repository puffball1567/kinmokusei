package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConversionDiscardsMatchGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "entry.km")
	input := `import go clock from "time";
let calls=0;
function next():int{calls++;return calls;}
constraint Number=~int;
function discard<T extends Number>():void{_=T(next());}
function Run():int{
 calls=0;_=int32(next());discard<int>();_=clock.Duration(next());
 for(_=int(next());calls<5;_=int(next())){}
 next();return calls;
}`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module conversion-discard.test\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "conversions")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import "time"
var calls int
func next()int{calls++;return calls}
func discard[T ~int](){_=T(next())}
func Run()int{calls=0;_=int32(next());discard[int]();_=time.Duration(next());for _=int(next());calls<5;_=int(next()){};next();return calls}
`
	comparison := `package conversions_test
import("testing";g "conversion-discard.test";r "conversion-discard.test/reference")
func TestRun(t *testing.T){if got,want:=g.Run(),r.Run();got!=want{t.Fatalf("conversion evaluation: %d != %d",got,want)}}
`
	runGeneratedGoDifferentialTest(t, root, "conversion-discard.test", generated, reference, comparison)
}
