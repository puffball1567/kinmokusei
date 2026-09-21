package sema

import (
	"strings"
	"testing"
)

func TestBuiltinResultStatementBoundaries(t *testing.T) {
	for _, expression := range []string{
		"len([1])", "cap([1])", "append([1],2)", "min(1,2)", "max(1,2)",
		"complex(1,2)", "real(1i)", "imag(1i)", "makeSlice<int>(1)", "makeMap<string,int>()", "goChannel<int>()",
		"min(pair())", "complex(parts())", "append(items())",
	} {
		for _, prefix := range []string{"", "defer ", "go "} {
			t.Run(prefix+expression, func(t *testing.T) {
				source := `function pair():(int,int){return 1,2;} function parts():(float,float){return 1,2;} function items():(int[],int){return [1],2;} function use():void{` + prefix + expression + `;}`
				got := strings.Join(checkSource(t, source), "\n")
				want := "result must be used"
				if prefix != "" {
					want = "cannot discard the result"
				}
				if !strings.Contains(got, want) {
					t.Fatalf("want %q, got %s", want, got)
				}
			})
		}
	}
}

func TestBuiltinStatementAllowedOperations(t *testing.T) {
	for _, statement := range []string{
		`copy([1],[2]);`, `defer copy([1],[2]);`, `go copy([1],[2]);`,
		`clear([1]);`, `defer clear([1]);`, `go clear([1]);`,
		`const m=makeMap<string,int>();delete(m,"key");`,
		`const ch=goChannel<int>();defer closeGoChannel(ch);`,
		`_=min(1,2);_=complex(1,2);_=append([1],2);`,
		`const min=(a:int,b:int):int=>a;min(1,2);defer min(1,2);go min(1,2);`,
	} {
		t.Run(statement, func(t *testing.T) {
			if got := checkSource(t, `function use():void{`+statement+`}`); len(got) != 0 {
				t.Fatal(got)
			}
		})
	}
}
