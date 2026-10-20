package sema

import (
	"strings"
	"testing"
)

func TestNullableClassInterfaceUpcastContracts(t *testing.T) {
	t.Parallel()
	const prelude = `interface I{function read():int;}
class C implements I{public function read():int{return 42;}}
function find():C|null{return null;}
function pair():(C|null,int){return find(),1;}
function take(v:I|null,n:int):boolean{return v===null;}
`
	for _, test := range []struct{ name, source, want string }{
		{"guarded local", `function f():int{const v:I|null=find();if(v===null){return 0;}return v.read();}`, ""},
		{"nullable guard required", `function f():int{const v:I|null=find();return v.read();}`, "must be checked against null"},
		{"assignment guard required", `function f():int{let v:I|null=new C();v=find();return v.read();}`, "must be checked against null"},
		{"non-null target", `function f():I{return find();}`, "cannot use"},
		{"mismatched interface", `interface Other{function other():int;}function f():Other|null{return find();}`, "cannot use"},
		{"explicit results", `function f():(I|null,int){return find(),1;}`, ""},
		{"destructured forwarding", `function f():(I|null,int){const [v,n]=pair();return v,n;}`, ""},
		{"sole-call conversion", `function f():boolean{return take(pair());}`, "destructure"},
		{"result forwarding conversion", `function f():(I|null,int){return pair();}`, "cannot use result 1"},
		{"exact forwarding", `function values():(I|null,int){return find(),1;}function f():(I|null,int){return values();}`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, prelude+test.source)
			if test.want == "" {
				if len(diagnostics) != 0 {
					t.Fatalf("unexpected diagnostics: %v", diagnostics)
				}
			} else if !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics %v; want %q", diagnostics, test.want)
			}
		})
	}
}
