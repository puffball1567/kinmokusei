package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefersAcrossExceptionBlocksMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input := `export function Run():string {
 let events="";
 const record=(value:string):void=>{events+=value;};
 const work=():void=>{
   defer record("outer;");
   try {
     defer record("try;");record("body;");throw new Exception("expected");
   } catch(_:Exception) {
     defer record("catch;");record("caught;");
   } finally {
     defer record("finally;");record("end;");
   }
   defer record("last;");record("after;");
 };
 work();return events;
}`
	path := filepath.Join(root, "defer.km")
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "defers")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
func Run()string {
 events:="";record:=func(v string){events+=v}
 func(){
   defer record("outer;")
   defer record("try;");record("body;")
   defer record("catch;");record("caught;")
   defer record("finally;");record("end;")
   defer record("last;");record("after;")
 }()
 return events
}`
	tests := `package defers_test
import("testing";g "defer-exception-scope.test";r "defer-exception-scope.test/reference")
func TestOrder(t *testing.T){if got,want:=g.Run(),r.Run();got!=want{t.Fatalf("got %q want %q",got,want)}}`
	runGeneratedGoDifferentialTest(t, root, "defer-exception-scope.test", generated, reference, tests)
}
