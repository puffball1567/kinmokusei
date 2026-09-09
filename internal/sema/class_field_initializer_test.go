package sema

import (
	"strings"
	"testing"
)

func TestClassFieldInitializerSemanticMatrix(t *testing.T) {
	for _, test := range []struct{ name, input, want string }{
		{"implicit constructor", `class Leaf {} class Holder { public leaf: Leaf = new Leaf(); }`, ""},
		{"constructor branch", `class Leaf {} class Holder { public leaf: Leaf = new Leaf(); constructor(flag: boolean) { if (flag) { this.leaf = new Leaf(); } } }`, ""},
		{"generic collections", `class Box<T> { public values: T[] = []; public value: T | null = null; }`, ""},
		{"generic conversion", `constraint Number = ~int | ~int8; class Box<T extends Number> { public value: T = T(1); }`, ""},
		{"module scope", `const value = 3; class Box { public number: int = value; constructor(value: string) {} }`, ""},
		{"static method", `class Box { public number: int = Box.initial(); private static function initial(): int { return 2; } }`, ""},
		{"callback", `class Box { public callback: (value: int) => int = (value: int): int => { return value + 1; }; }`, ""},
		{"class upcast", `class Base {} class Child extends Base {} class Box { public value: Base = new Child(); }`, ""},
		{"required other field", `class Leaf {} class Holder { public first: Leaf = new Leaf(); public second: Leaf; }`, "must be initialized"},
		{"mistyped", `class Box { public value: int = "wrong"; }`, "cannot use string as int"},
		{"null", `class Leaf {} class Box { public leaf: Leaf = null; }`, "cannot use null as Leaf"},
		{"constructor parameter", `class Box { public number: int = value; constructor(value: int) {} }`, `undefined name "value"`},
		{"this field", `class Box { public number: int = this.other; public other: int = 3; }`, "class field initializers cannot reference this or super"},
		{"this capture", `class Box { public callback: () => int = (): int => this.read(); public function read(): int { return 1; } }`, "class field initializers cannot reference this or super"},
		{"super method", `class Base { public function read(): int { return 1; } } class Box extends Base { public value: int = super.read(); }`, "class field initializers cannot reference this or super"},
		{"super constructor callback", `class Base {} class Box extends Base { public call: () => void = (): void => { super(); }; }`, "super(...) may only be called from a derived-class constructor"},
		{"generated helper collision", `class Box { public value: int = 1; } function __kinmokuseiFieldsBox(): void {}`, "generated Go name"},
		{"generated helper parameter shadow", `class Box { public value: int = 1; constructor(__kinmokuseiFieldsBox: int) {} }`, "constructor parameter name conflicts with the generated class field initializer"},
		{"generated helper type parameter shadow", `class Box<__kinmokuseiFieldsBox> { public value: int = 1; }`, "type parameter name conflicts with the generated class field initializer"},
		{"base constructor still required", `class Base { constructor(value: int) {} } class Box extends Base { public value: int = 1; }`, "needs a constructor"},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.input)
			if test.want == "" && len(diagnostics) != 0 || test.want != "" && !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}
