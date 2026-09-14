package sema

import (
	"strings"
	"testing"
)

func TestConstraintMethodComposition(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"method only", `import go fmt from "fmt"; constraint Printable=fmt.Stringer; function show<T extends Printable>(value:T):string{return value.String();}`, ""},
		{"compose methods", `import go io from "io"; constraint Stream=io.Reader&io.Closer; function close<T extends Stream>(value:T):error{return value.Close();}`, ""},
		{"repeat diamond", `import go io from "io"; constraint A=io.Reader&io.Closer; constraint B=io.Reader&A; constraint C=B&A; function close<T extends C>(value:T):error{return value.Close();}`, ""},
		{"underlying and methods", `import go fmt from "fmt"; constraint A=~int&fmt.Stringer; function show<T extends A>(x:T):string{return (x+x).String();}`, ""},
		{"methods first", `import go fmt from "fmt"; constraint A=fmt.Stringer&~int; function show<T extends A>(x:T):string{return (x+x).String();}`, ""},
		{"nested narrowed type set", `import go fmt from "fmt"; constraint Base=~int|~string; constraint A=Base&fmt.Stringer; constraint B=A&~int; function twice<T extends B>(x:T):T{return x*2;}`, ""},
		{"missing method", `import go fmt from "fmt"; constraint A=~int&fmt.Stringer; function keep<T extends A>(x:T):T{return x;} function bad(x:int):int{return keep(x);}`, "does not satisfy"},
		{"nullable receiver", `constraint A=error; function show<T extends A>(x:T):string{return x.Error();} function bad(x:error|null):string{return show(x);}`, "does not satisfy"},
		{"missing type set", `import go fmt from "fmt"; import go time from "time"; constraint A=~int&fmt.Stringer; function keep<T extends A>(x:T):T{return x;} function bad(x:time.Duration):time.Duration{return keep(x);}`, "does not satisfy"},
		{"method reuse must retain requirements", `import go fmt from "fmt"; constraint A=fmt.Stringer; constraint B=A; function keep<T extends B>(x:T):T{return x;} function bad(x:int):int{return keep(x);}`, "does not satisfy"},
		{"union cannot erase methods", `import go fmt from "fmt"; constraint A=~int&fmt.Stringer; constraint B=A|string;`, "union operands must be type sets without method or comparable requirements"},
		{"method union", `import go io from "io"; constraint A=io.Reader|io.Closer;`, "use '&' to compose interfaces"},
		{"tilde interface", `import go fmt from "fmt"; constraint A=~fmt.Stringer;`, "must name concrete types"},
		{"imported type set", `import go cmp from "cmp"; constraint A=cmp.Ordered&~int;`, ""},
		{"source interface", `interface Reader{function read():int;} constraint A=Reader;`, "must be a concrete type, not an interface"},
		{"cycle", `import go fmt from "fmt"; constraint A=B&fmt.Stringer; constraint B=A;`, "constraint declaration cycle"},
		{"pure method not numeric", `import go fmt from "fmt"; constraint A=fmt.Stringer; function bad<T extends A>(x:T):T{return x+x;}`, "numeric"},
		{"constraint not value", `import go io from "io"; constraint A=io.Reader&io.Closer; function bad(x:A):void{}`, "can only be used after 'extends'"},
		{"empty finite intersection with methods", `import go fmt from "fmt"; constraint A=int&fmt.Stringer&string;`, "no common types"},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if test.want == "" && len(diagnostics) != 0 || test.want != "" && !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics=%v want=%q", diagnostics, test.want)
			}
		})
	}
}
