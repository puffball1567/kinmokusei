package sema

import (
	"strings"
	"testing"
)

func TestAnonymousInterfaceSourceType(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"method", `function read(value: interface { read(offset:int):string; }):string{return value.read(0);}`, ""},
		{"multiple methods", `function use(value: interface { read():string; close():void; }):string{value.close();return value.read();}`, ""},
		{"generic method parameter", `function use(value: interface { read(value:int):string; }):string{return value.read(1);}`, ""},
		{"terminator optional", `function bad(value: interface { read():string }):string{return "";}`, ""},
		{"generic class", `class Reader<T>{public function read(value:T):T{return value;}} function use():void{const reader:interface{read(value:int):int;}=new Reader<int>();}`, ""},
		{"nullable result", `class Box{} class Reader{public function read():Box | null{return null;}} function use():void{const reader:interface{read():Box;}=new Reader();}`, "cannot use"},
		{"nullable argument", `class Box{} class Reader{public function read(value:Box):void{}} function use():void{const reader:interface{read(value:Box | null):void;}=new Reader();}`, "cannot use"},
		{"Go embedding", `import go io from "io";function use(v:interface{io.Reader;}):(int,error){const p:byte[]=[0];return v.Read(p);}`, ""},
		{"Go embedding as generic bound", `import go io from "io";function use<T extends interface{io.Reader;io.ReadCloser;}>(v:T):(int,error){const p:byte[]=[0];return v.Read(p);}`, ""},
		{"nested embedding", `alias Text=interface{read():string;};function use(v:interface{Text; interface{close():void;};}):string{v.close();return v.read();}`, ""},
		{"overlapping methods", `import go fmt from "fmt";function use(v:interface{fmt.Stringer;String():bstring;}):bstring{return v.String();}`, ""},
		{"incompatible source method", `import go fmt from "fmt";function use(v:interface{fmt.Stringer;String():string;}):void{}`, "conflicting Go method name"},
		{"competing source spelling", `import go io from "io";function use(v:interface{io.Reader;read(p:byte[]):(int,error);}):void{}`, "conflicting Go method name"},
		{"incompatible nested method", `function use(v:interface{interface{run():int;};interface{run():string;};}):void{}`, "conflicting Go method name"},
		{"concrete embedding", `function use(v:interface{int;}):void{}`, "embedding requires"},
		{"type set embedding", `constraint Number=~int;function use(v:interface{Number;}):void{}`, "can only be used after 'extends'"},
		{"nullable embedding", `import go io from "io";function use(v:interface{io.Reader|null;}):void{}`, "embedding requires"},
		{"unexported method", `import go reflect from "reflect";function use(v:interface{reflect.Type;}):void{}`, "unexported Go method"},
		{"recursive alias embedding", `alias A=interface{B;};alias B=interface{A;};function use(v:A):void{}`, "type declaration cycle"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.source), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
