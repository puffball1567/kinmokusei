package sema

import (
	"strings"
	"testing"
)

const optionalPrelude = `class User {
 public name:string="Alice";
 public next:User|null=null;
 public items:int[]=[7];
 public function read(n:int):int{return n;}
 public function touch():void{}
 public function self():User{return this;}
}
`

func TestOptionalChainingTypes(t *testing.T) {
	t.Parallel()
	for name, code := range map[string]string{
		"scalar default":    `function f(u:User|null):string{return u?.name??"anonymous";}`,
		"scalar split":      `function f(u:User|null):string{const[v,ok]=u?.name;if(!ok){return "anonymous";}return v;}`,
		"scalar assertion":  `function f(u:User|null):string{return u?.name!;}`,
		"method":            `function f(u:User|null):int{return u?.read(7)??0;}`,
		"void method":       `function f(u:User|null):void{u?.touch();}`,
		"nested":            `function f(u:User|null):string{return u?.next?.name??"anonymous";}`,
		"reference":         `function f(u:User|null):User|null{return u?.self();}`,
		"grouped":           `function f(u:User|null):string{return (u?.self())!.name;}`,
		"ordinary tail":     `function f(u:User|null):int{return u?.items[0]??0;}`,
		"optional index":    `function f(a:int[]|null):int{return a?.[0]??0;}`,
		"optional function": `alias Callback=(n:int)=>int;function f(a:Callback|null):int{return a?.(7)??0;}`,
		"generic payload":   `alias Callback<T>=()=>T;function f<T>(a:Callback<T>|null,d:T):T{return a?.()??d;}`,
		"map reference":     `function f(m:Map<string,User>):string{return m["missing"]?.name??"anonymous";}`,
		"raw Go":            `import go flag from "flag";function f():bstring{return flag.Lookup("missing")?.Name??b"none";}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if ds := checkSource(t, optionalPrelude+code); len(ds) != 0 {
				t.Fatal(ds)
			}
		})
	}
}

func TestOptionalChainingRejectsUnsafeUses(t *testing.T) {
	t.Parallel()
	for name, code := range map[string]string{
		"scalar receiver":  `function f(u:int):void{u?.name;}`,
		"nullable tail":    `function f(u:User|null):string{return u?.next.name??"none";}`,
		"group ends chain": `function f(u:User|null):string{return (u?.self()).name;}`,
		"address":          `function f(u:User|null):void{const p=&u?.name;}`,
		"effect narrowing": `function f(u:User|null):string{u?.touch();return u.name;}`,
		"scalar storage":   `function f(u:User|null):void{const name=u?.name;}`,
		"Result":           `alias Callback=()=>Result<int>;function f(a:Callback|null):void{a?.();}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if ds := checkSource(t, optionalPrelude+code); len(ds) == 0 {
				t.Fatal("unsafe optional chain accepted")
			}
		})
	}
	if ds := strings.Join(checkSource(t, optionalPrelude+`function f(u:User|null):void{const value=u?.name;}`), "\n"); !strings.Contains(ds, "multiple") {
		t.Fatal(ds)
	}
}
