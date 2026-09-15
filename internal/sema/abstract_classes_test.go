package sema

import (
	"strings"
	"testing"
)

func TestAbstractClasses(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"empty", `abstract class Base{}class Leaf extends Base{}function use():Base{return new Leaf();}`, ""},
		{"contract", `interface Reader{function read():int;}abstract class Base implements Reader{public abstract function read():int;}class Leaf extends Base{public override function read():int{return 2;}}function use(v:Base):Reader{return v;}`, ""},
		{"generic", `abstract class Base<T>{protected abstract function read():T;public function get():T{return this.read();}}class Leaf extends Base<int>{protected override function read():int{return 2;}}`, ""},
		{"intermediate", `abstract class Base{public abstract function read():int;}abstract class Middle extends Base{}class Leaf extends Middle{public final override function read():int{return 2;}}`, ""},
		{"reabstract", `class Base{public virtual function read():int{return 1;}}abstract class Middle extends Base{public abstract override function read():int;}class Leaf extends Middle{public override function read():int{return 2;}}`, ""},
		{"factory name available", `abstract class Base{}function NewBase():int{return 1;}`, ""},
		{"new", `abstract class Base{}function use():Base{return new Base();}`, "cannot instantiate abstract class"},
		{"generic new", `abstract class Base<T>{}function use():Base<int>{return new Base<int>();}`, "cannot instantiate abstract class"},
		{"concrete declaration", `class Base{public abstract function read():int;}`, "abstract methods require an abstract class"},
		{"missing override body", `abstract class Base{public abstract function read():int;}class Leaf extends Base{}`, "must implement abstract method read"},
		{"missing in unused intermediate", `abstract class Base{public abstract function read():int;}class Middle extends Base{}class Leaf extends Middle{public override function read():int{return 2;}}`, "concrete class Middle"},
		{"missing interface signature", `interface Reader{function read():int;}abstract class Base implements Reader{}`, "missing method read"},
		{"explicit override", `abstract class Base{public abstract function read():int;}class Leaf extends Base{public function read():int{return 2;}}`, "add override"},
		{"signature", `abstract class Base{public abstract function read():int;}class Leaf extends Base{public override function read():string{return "x";}}`, "incompatible signature"},
		{"nullable", `class Item{}abstract class Base{public abstract function read():Item;}class Leaf extends Base{public override function read():Item|null{return null;}}`, "incompatible signature"},
		{"private", `abstract class Base{private abstract function read():int;}`, "abstract methods must be public or protected"},
		{"static", `abstract class Base{public static abstract function read():int;}`, "abstract methods cannot be static or final"},
		{"final class", `abstract final class Base{}`, "both abstract and final"},
		{"final method", `abstract class Base{public final abstract function read():int;}`, "abstract methods cannot be static or final"},
		{"generic method", `abstract class Base{public abstract function read<T>(v:T):T;}`, "Go method sets"},
		{"super", `abstract class Base{public abstract function read():int;}class Leaf extends Base{public override function read():int{return super.read();}}`, "super cannot access abstract method"},
		{"super method value", `abstract class Base{public abstract function read():int;}class Leaf extends Base{public override function read():int{const f=super.read;return f();}}`, "super cannot access abstract method"},
		{"direct constructor call", `abstract class Base{constructor(){this.read();}public abstract function read():int;}`, "abstract method on this during construction"},
		{"orphan override", `abstract class Base{public abstract override function read():int;}`, "no inherited method"},
		{"duplicate parameter", `abstract class Base{public abstract function read(value:int,value:int):int;}`, "duplicate"},
		{"Go interface", `abstract class Base implements error{public abstract function error():string;}class Leaf extends Base{public override function error():string{return "failure";}}function use(v:Base):error{return v;}`, ""},
		{"visibility", `abstract class Base{protected abstract function read():int;}class Leaf extends Base{public override function read():int{return 2;}}`, "preserve inherited visibility"},
		{"abstract final override", `class Base{public virtual function read():int{return 1;}}abstract class Leaf extends Base{public final abstract override function read():int;}`, "abstract methods cannot be static or final"},
		{"reabstract final method", `class Base{public virtual function read():int{return 1;}}class Middle extends Base{public final override function read():int{return 2;}}abstract class Leaf extends Middle{public abstract override function read():int;}`, "final and cannot be overridden"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.source), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
