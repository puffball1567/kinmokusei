package sema

import (
	"strings"
	"testing"
)

func TestSourceMethodContractInvariance(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"nullable result implementation", `class Leaf{} interface Reader{function read():Leaf;} class Bad implements Reader{public function read():Leaf|null{return null;}}`, "incompatible signature"},
		{"nullable input implementation", `class Leaf{} interface Sink{function put(value:Leaf|null):void;} class Bad implements Sink{public function put(value:Leaf):void{}}`, "incompatible signature"},
		{"nullable result override", `class Leaf{} class Base{public virtual function read():Leaf{return new Leaf();}} class Bad extends Base{public override function read():Leaf|null{return null;}}`, "incompatible signature"},
		{"nullable input override", `class Leaf{} class Base{public virtual function put(value:Leaf|null):void{}} class Bad extends Base{public override function put(value:Leaf):void{}}`, "incompatible signature"},
		{"nullable inherited conflict", `class Leaf{} interface A{function read():Leaf;} interface B{function read():Leaf|null;} interface Bad extends A,B{}`, "incompatible signatures"},
		{"nullable redeclaration", `class Leaf{} interface A{function read():Leaf;} interface Bad extends A{function read():Leaf|null;}`, "incompatible signatures"},
		{"generic nullable diamond", `class Leaf{} interface Reader<T>{function read():T;} interface A extends Reader<Leaf>{} interface B extends Reader<Leaf|null>{} interface Bad extends A,B{}`, "incompatible signatures"},
		{"nullable slice implementation", `class Leaf{} alias Maybe=Leaf|null; interface Reader{function read():Maybe[];} class Bad implements Reader{public function read():Leaf[]{return [];}}`, "incompatible signature"},
		{"nullable callback input", `class Leaf{} interface Reader{function read(callback:(value:Leaf|null)=>void):void;} class Bad implements Reader{public function read(callback:(value:Leaf)=>void):void{}}`, "incompatible signature"},
		{"nullable object field", `class Leaf{} interface Reader{function read():{value:Leaf};} class Bad implements Reader{public function read():{value:Leaf|null}{return {value:null};}}`, "incompatible signature"},
		{"nullable Result payload", `class Leaf{} interface Reader{function read():Result<Leaf>;} class Bad implements Reader{public function read():Result<Leaf|null>{return ok(null);}}`, "incompatible signature"},
		{"nullable map value", `class Leaf{} interface Sink{function put(value:Map<string,Leaf|null>):void;} class Bad implements Sink{public function put(value:Map<string,Leaf>):void{}}`, "incompatible signature"},
		{"nullable generic argument", `class Leaf{} class Box<T>{constructor(public value:T){}} interface Reader{function read():Box<Leaf>;} class Bad implements Reader{public function read():Box<Leaf|null>{return new Box<Leaf|null>(null);}}`, "incompatible signature"},
		{"nullable callback result", `class Leaf{} interface Reader{function read(callback:()=>Leaf):void;} class Bad implements Reader{public function read(callback:()=>Leaf|null):void{}}`, "incompatible signature"},
		{"generic method cannot implement", `interface Reader{function read():int;} class Bad implements Reader{public function read<T>():int{return 1;}}`, "incompatible signature"},
		{"matching nullable contract", `class Leaf{} interface Reader<T>{function read():T;} interface A extends Reader<Leaf|null>{} interface B extends Reader<Leaf|null>{} interface C extends A,B{} class Good implements C{public function read():Leaf|null{return null;}}`, ""},
		{"matching nested nullable contract", `class Leaf{} alias Maybe=Leaf|null; interface Reader{function read(callback:(value:Maybe)=>void):Maybe[];} class Good implements Reader{public function read(callback:(value:Maybe)=>void):Maybe[]{callback(null);return [null];}}`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if test.want == "" && len(diagnostics) != 0 || test.want != "" && !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics=%v want=%q", diagnostics, test.want)
			}
		})
	}
}
