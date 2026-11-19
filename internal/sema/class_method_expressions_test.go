package sema

import (
	"strings"
	"testing"
)

func TestClassMethodExpressions(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, source, want string }{
		{"dependent receiver", `class Box<T>{public function read(v:T):T{return v;}}function f<T>(box:Box<T>,v:T):T{return Box[T].read(box,v);}`, ""},
		{"lexical shadow", `class Box<T>{public function read(v:T):T{return v;}}function f(Box:Box<int>):int{return Box.read(7);}`, ""},
		{"shadowed implicit result type", `class Item{}class Box{public function read():Item{return new Item();}}function f(box:Box):void{const Item=1;_=Box.read(box);}`, "shadows generated method expression type"},
		{"private within owner", `class Box{private function read():int{return 7;}public function use():int{return Box.read(this);}}`, ""},
		{"private outside owner", `class Box{private function read():int{return 7;}}const read=Box.read;`, "is private"},
		{"protected within descendant", `class Base{protected function read():int{return 7;}}class Child extends Base{public function use():int{return Child.read(this);}}`, ""},
		{"protected outside hierarchy", `class Base{protected function read():int{return 7;}}class Child extends Base{}const read=Child.read;`, "is protected"},
		{"nullable receiver", `class Box{public function read():int{return 7;}}function f(v:Box|null):int{return Box.read(v);}`, "cannot use"},
		{"nullable result", `class Box<T>{public function read(v:T):T{return v;}}class Item{}function f(box:Box<Item|null>,v:Item|null):Item{return Box<Item|null>.read(box,v);}`, "cannot use"},
		{"raw string result", `class Box<T>{public function read(v:T):T{return v;}}function f(box:Box<bstring>,v:bstring):string{return Box<bstring>.read(box,v);}`, "cannot use"},
		{"pointer spelling", `class Box{public function read():int{return 7;}}const read=(*Box).read;`, "cannot be used as a Go pointer target"},
		{"instance field", `class Box{public value:int=7;}const value=Box.value;`, "has no non-generic method"},
		{"instance property", `class Box{public get value():int{return 7;}}const value=Box.value;`, "requires an instance"},
		{"method type parameters", `class Box{public function read<T>(v:T):T{return v;}}const read=Box.read;`, "has no non-generic method"},
		{"missing owner arguments", `class Box<T>{public function read(v:T):T{return v;}}const read=Box.read;`, "type arguments"},
		{"owner constraint", `class Box<T extends comparable>{public function read(v:T):T{return v;}}const read=Box<int[]>.read;`, "does not satisfy"},
		{"concrete receiver identity", `class Base{public function read():int{return 7;}}class Child extends Base{}function f(v:Base):int{return Child.read(v);}`, "cannot use"},
		{"global cycle through method body", `class Box{public function read():int{return this.other();}public function other():int{return n;}}const n:int=Box.read(new Box());`, "initialization cycle"},
		{"virtual wrapper cycle", `class Base{public virtual function read():int{return n;}}class Child extends Base{public override function read():int{return 7;}}const n:int=Child.read(new Child());`, "initialization cycle"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			diagnostics := strings.Join(checkSource(t, tt.source), "\n")
			if tt.want == "" && diagnostics != "" || tt.want != "" && !strings.Contains(diagnostics, tt.want) {
				t.Fatalf("diagnostics=%q want=%q", diagnostics, tt.want)
			}
		})
	}
}
