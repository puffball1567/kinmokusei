package sema

import "testing"

func TestNamedGoImportsHonorUnsafePolicy(t *testing.T) {
	t.Parallel()
	input := `import go { Pointer, Sizeof, Alignof } from "unsafe";
function f(value:Pointer):uint64{return uint64(Sizeof(value)+Alignof(value));}`
	if diagnostics := checkSource(t, input); len(diagnostics) == 0 {
		t.Fatal("unsafe import accepted without policy")
	}
	if diagnostics := checkSourceWithPolicy(t, input, GoInteropPolicy{AllowUnsafe: true}); len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
}

func TestNamedGoImportScopeAndConstantRules(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`import go { IsPrint } from "unicode"; function f(IsPrint:int):int{return IsPrint;}`,
		`import go { MaxUint8 } from "math"; class C{public value:int;constructor(){for(const i of MaxUint8){this.value=i;}}}`,
		`import go { GOOS } from "runtime"; class C{public value:int;constructor(){for(const i of GOOS){this.value=int(i);}}}`,
		`import go { IsPrint } from "unicode"; const callback=IsPrint; function f():boolean{return callback(32);}`,
	} {
		if diagnostics := checkSource(t, input); len(diagnostics) != 0 {
			t.Fatalf("%s: %v", input, diagnostics)
		}
	}
}
