package sema

import (
	"strings"
	"testing"
)

func TestSingleInheritanceSemanticMatrix(t *testing.T) {
	valid := []string{
		`class Base { public virtual function read(): int { return 1; } } class Child extends Base { public override function read(): int { return super.read() + 1; } } function use(): int { return new Child().read(); }`,
		`class Base { constructor() {} public function read(): int { return 1; } } class Child extends Base { constructor() { super(); } public function own(): int { return this.read(); } }`,
		`interface Reader { function read(): int; } class Base implements Reader { public virtual function read(): int { return 1; } } class Child extends Base { public override function read(): int { return 2; } } function use(): int { const value: Reader = new Child(); return value.read(); }`,
		`class Base { public value: int; constructor(value: int) { this.value = value; } } class Child extends Base { constructor(value: int) { super(value); } public function read(): int { return this.value; } }`,
		`class Base {} class Child extends Base {} function use(value: Base): boolean { const [child, ok] = value as? Child; return ok; }`,
		`class Base {} class Child extends Base {} class Grandchild extends Child {} function use(value: Base | null): Child | null { const [child, ok] = value as? Child; if (ok) { return child; } return null; }`,
		`class Base {} class Child extends Base {} function use(value: Base): Child { return value as! Child; }`,
		`class Base { constructor(protected value: int) {} protected virtual function read(): int { return this.value; } protected static function label(): string { return "base"; } } class Child extends Base { constructor() { super(1); } protected override function read(): int { return super.read() + this.value; } public function inspect(other: Base): string { const value = other.value + this.read(); return Base.label(); } }`,
		`class Base { public virtual function read(): int { return 1; } } final class Child extends Base { public final override function read(): int { return 2; } } function use(): int { const value: Base = new Child(); return value.read(); }`,
	}
	for index, source := range valid {
		if diagnostics := checkSource(t, source); len(diagnostics) != 0 {
			t.Errorf("valid case %d diagnostics = %v", index, diagnostics)
		}
	}

	invalid := []struct {
		name     string
		source   string
		contains string
	}{
		{"unknown base", `class Child extends Missing {}`, `unknown base class "Missing"`},
		{"cycle", `class First extends Second {} class Second extends First {}`, "inheritance cycle"},
		{"missing override", `class Base { public virtual function read(): int { return 1; } } class Child extends Base { public function read(): int { return 2; } }`, "add override"},
		{"nonvirtual override", `class Base { public function read(): int { return 1; } } class Child extends Base { public override function read(): int { return 2; } }`, "is not virtual"},
		{"orphan override", `class Value { public override function read(): int { return 1; } }`, "no inherited method"},
		{"signature", `class Base { public virtual function read(value: int): int { return value; } } class Child extends Base { public override function read(value: string): int { return 2; } }`, "incompatible signature"},
		{"private virtual", `class Base { private virtual function read(): int { return 1; } }`, "virtual methods must be public"},
		{"static virtual", `class Base { public static virtual function read(): int { return 1; } }`, "static methods cannot be virtual"},
		{"missing super", `class Base { constructor(value: int) {} } class Child extends Base { constructor() {} }`, "must call super"},
		{"no constructor", `class Base { constructor(value: int) {} } class Child extends Base {}`, "needs a constructor"},
		{"late super", `class Base { constructor() {} } class Child extends Base { constructor() { const value = 1; super(); } }`, "must be the first statement"},
		{"super outside derived", `class Value { public function read(): int { return super.read(); } }`, "requires a derived class"},
		{"private inherited field", `class Base { private value: int; constructor(value: int) { this.value = value; } } class Child extends Base { constructor() { super(1); } public function read(): int { return this.value; } }`, `field "value" is private`},
		{"field method collision", `class Base { public function read(): int { return 1; } } class Child extends Base { public read: int; constructor() { this.read = 1; } }`, `conflicts with method`},
		{"unrelated downcast", `class First {} class Second {} function use(value: First): Second { return value as! Second; }`, `not in the same inheritance chain`},
		{"upcast assertion", `class Base {} class Child extends Base {} function use(value: Child): boolean { const [base, ok] = value as? Base; return ok; }`, `is an upcast`},
		{"same class assertion", `class Value {} function use(value: Value): Value { return value as! Value; }`, `no downcast is needed`},
		{"nonclass downcast target", `class Value {} function use(value: Value): int { return value as! int; }`, `class downcast requires class source and target types`},
		{"protected field outside hierarchy", `class Base { protected value: int; constructor() { this.value = 1; } } function use(value: Base): int { return value.value; }`, `field "value" is protected`},
		{"protected method outside hierarchy", `class Base { protected function read(): int { return 1; } } function use(value: Base): int { return value.read(); }`, `method "read" is protected`},
		{"protected static outside hierarchy", `class Base { protected static function read(): int { return 1; } } function use(): int { return Base.read(); }`, `method "read" is protected`},
		{"inherited private static in child", `class Base { private static function read(): int { return 1; } } class Child extends Base { public static function use(): int { return Child.read(); } }`, `method "read" is private`},
		{"protected override narrowed", `class Base { protected virtual function read(): int { return 1; } } class Child extends Base { private override function read(): int { return 2; } }`, `must preserve inherited visibility`},
		{"public override narrowed", `class Base { public virtual function read(): int { return 1; } } class Child extends Base { protected override function read(): int { return 2; } }`, `must preserve inherited visibility`},
		{"extend final class", `final class Base {} class Child extends Base {}`, `cannot extend final class Base`},
		{"override final method", `class Base { public virtual function read(): int { return 1; } } class Middle extends Base { public final override function read(): int { return 2; } } class Child extends Middle { public override function read(): int { return 3; } }`, `is final and cannot be overridden`},
		{"final nonvirtual method", `class Value { public final function read(): int { return 1; } }`, `final methods must override an inherited virtual method`},
		{"final root virtual method", `class Value { public final virtual function read(): int { return 1; } }`, `final methods must override an inherited virtual method`},
		{"final static method", `class Value { public static final function read(): int { return 1; } }`, `final methods must override an inherited virtual method`},
		{"public upcast name collision", `function UpcastChildToBase(value: int): int { return value; } class Base {} class Child extends Base {}`, `generated Go name "UpcastChildToBase" collides`},
		{"virtual slot field collision", `class Base { private __ontamaBaseRead: int; constructor() { this.__ontamaBaseRead = 1; } public virtual function read(): int { return 1; } }`, `generated Go struct member name "__ontamaBaseRead" collides`},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if !containsDiagnostic(diagnostics, test.contains) {
				t.Fatalf("diagnostics = %v, want substring %q", diagnostics, test.contains)
			}
		})
	}
}

func containsDiagnostic(diagnostics []string, substring string) bool {
	for _, message := range diagnostics {
		if strings.Contains(message, substring) {
			return true
		}
	}
	return false
}
