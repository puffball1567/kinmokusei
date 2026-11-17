package sema

import (
	"strings"
	"testing"
)

const resultPresencePrelude = `class C { public n: int = 7; }
const YES = true;
const NO = false;
function load(reject: boolean): Result<C> {
  if (reject) { return fail(new Exception("failed")); }
  return ok(new C());
}
`

func TestResultPresenceGuards(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"success":          `if(err===nil){return value.n;}return 0;`,
		"failure exit":     `if(err!==nil){return 0;}return value.n;`,
		"reverse":          `if(nil==err){return value.n;}return 0;`,
		"negation":         `if(!(err!=nil)){return value.n;}return 0;`,
		"nested boolean":   `if((err===nil)!==NO){return value.n;}return 0;`,
		"boolean switch":   `switch(err===nil){case YES{return value.n;}default{return 0;}}`,
		"condition switch": `switch(true){case err!=nil{return 0;}default{return value.n;}}`,
		"error switch":     `switch(err){case nil{return value.n;}default{return 0;}}`,
		"default first":    `switch(err){default{return 0;}case nil{return value.n;}}`,
		"prior join":       `if(reject){}if(err===nil){return value.n;}return 0;`,
		"constant capture": `const read=():int=>{if(err!=nil){return 0;}return value.n;};return read();`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			input := resultPresencePrelude + `function f(reject:boolean):int{const [value,err]=load(reject);` + body + `}`
			if diagnostics := checkSource(t, input); len(diagnostics) != 0 {
				t.Fatalf("rejected: %v", diagnostics)
			}
		})
	}
}

func TestRejectsUnprovenResultPresence(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"error merely read":      `_ = err;return value.n;`,
		"copied error":           `const copy=err;if(copy==nil){return value.n;}return 0;`,
		"copied boolean":         `const ok=err==nil;if(ok){return value.n;}return 0;`,
		"reset error":            `err=nil;if(err==nil){return value.n;}return 0;`,
		"shadowed error":         `{const err:error=nil;if(err==nil){return value.n;}}_=err;return 0;`,
		"failure branch":         `if(err!=nil){return value.n;}return 0;`,
		"merged branches":        `if(err==nil){}return value.n;`,
		"failure switch":         `switch(err){case nil{return 0;}default{return value.n;}}`,
		"mixed boolean cases":    `switch(err==nil){case true,false{return value.n;}}return 0;`,
		"failure fallthrough":    `switch(err==nil){case false{fallthrough;}case true{return value.n;}}return 0;`,
		"address of error":       `const pointer=&err;if(err==nil){return value.n;}return 0;`,
		"mutable capture":        `if(err!=nil){return 0;}const read=()=>value.n;return read();`,
		"new result old error":   `let other:error=nil;[value,other]=load(reject);_=other;if(err==nil){return value.n;}return 0;`,
		"changed result in loop": `if(err!=nil){return 0;}while(reject){[value,err]=load(reject);}return value.n;`,
		"changed pair in label":  `const reset=():error=>{[value,err]=load(reject);return new Exception("failed");};switch(err){case reset(){return 0;}case nil{return value.n;}default{return 0;}}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			input := resultPresencePrelude + `function f(reject:boolean):int{let [value,err]=load(reject);` + body + `}`
			diagnostics := strings.Join(checkSource(t, input), "\n")
			if !strings.Contains(diagnostics, "check its corresponding error binding against nil") {
				t.Fatalf("unsafe Result payload accepted: %s", diagnostics)
			}
		})
	}
}

func TestResultPresenceProducerAndStorageKinds(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"existing bindings": resultPresencePrelude + `function f(reject:boolean):int{let value:C=new C();let err:error=nil;[value,err]=load(reject);if(err!=nil){return 0;}return value.n;}`,
		"checked snapshot":  resultPresencePrelude + `function f(reject:boolean):int{let [value,err]=load(reject);if(err!=nil){return 0;}err=nil;return value.n;}`,
		"safe replacement":  resultPresencePrelude + `function f(reject:boolean):int{let [value,err]=load(reject);value=new C();_=err;return value.n;}`,
		"nullable error":    resultPresencePrelude + `function f(reject:boolean):int{let value:C=new C();let err:error|null=null;[value,err]=load(reject);if(err===null){return value.n;}return 0;}`,
		"safe scalar":       `function load():Result<int>{return ok(7);}function f():int{const[value,_]=load();return value;}`,
		"nullable payload":  resultPresencePrelude + `function optional():Result<C|null>{return ok(null);}function f():int{const[value,_]=optional();if(value===null){return 0;}return value.n;}`,
		"callback":          `alias F=()=>int;function load():Result<F>{return ok(()=>7);}function f():int{const[value,err]=load();if(err!=nil){return 0;}return value();}`,
		"generic":           `function f<T>(source:()=>Result<T>,fallback:T):T{const[value,err]=source();if(err!=nil){return fallback;}return value;}`,
		"record":            resultPresencePrelude + `struct Row{public value:C;}function source():Result<Row>{return ok(Row{value:new C()});}function f():int{const[row,err]=source();if(err!=nil){return 0;}return row.value.n;}`,
		"result then map":   resultPresencePrelude + `function f(reject:boolean,m:Map<int,C>):int{let[value,err]=load(reject);_=err;let present=false;[value,present]=m[0];if(present){return value.n;}return 0;}`,
		"map then result":   resultPresencePrelude + `function f(reject:boolean,m:Map<int,C>):int{let[value,present]=m[0];let err:error=nil;[value,err]=load(reject);if(err==nil){return value.n;}return 0;}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if diagnostics := checkSource(t, input); len(diagnostics) != 0 {
				t.Fatalf("rejected: %v", diagnostics)
			}
		})
	}
	for name, input := range map[string]string{
		"discarded error": resultPresencePrelude + `function f():int{const[value,_]=load(false);return value.n;}`,
		"global storage":  resultPresencePrelude + `let value:C=new C();function f(reject:boolean):int{let err:error=nil;[value,err]=load(reject);_=err;return value.n;}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if diagnostics := checkSource(t, input); len(diagnostics) == 0 {
				t.Fatal("unsafe Result storage accepted")
			}
		})
	}
}
