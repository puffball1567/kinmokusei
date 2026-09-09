package sema

import (
	"fmt"
	"strings"
	"testing"
)

func TestInterfaceInheritanceSemanticMatrix(t *testing.T) {
	tests := []struct{ name, source, want string }{
		{"forward diamond", `interface Combined<T> extends Left<T>, Right<T> {} interface Left<T> extends Root<T> {} interface Right<T> extends Root<T> {} interface Root<T> { function read(): T; } class Value implements Combined<int> { public function read(): int { return 1; } } function use(value: Combined<int>): Root<int> { return value; } function create(): Root<int> { return new Value(); }`, ""},
		{"reordered nested parameters", `interface Child<T, U> extends Parent<U[], T> {} interface Parent<T, U> { function apply(value: T): U; } function use(value: Child<int, string>): int { return value.apply(["x"]); }`, ""},
		{"same signature redeclaration", `interface Root { function read(): int; } interface Child extends Root { function read(): int; }`, ""},
		{"transparent base alias", `interface Root<T> { function read(): T; } alias Readable<T> = Root<T>; interface Child<T> extends Readable<T> {} function use(value: Child<int>): int { return value.read(); }`, ""},
		{"explicit arguments select implemented contract", `interface Marker<T> {} class Both implements Marker<int>, Marker<string> {} function use<T>(value: Marker<T>): int { return 1; } function create(): int { return use<string>(new Both()); }`, ""},
		{"partial arguments select ancestor", `interface Marker<T, U> {} interface Child extends Marker<int, string>, Marker<string, int> {} function use<T, U>(value: Marker<T, U>): int { return 1; } function call(value: Child): int { return use<string>(value); }`, ""},
		{"ambiguous contract inference", `interface Marker<T> {} interface Child extends Marker<int>, Marker<string> {} function use<T>(value: Marker<T>): int { return 1; } function call(value: Child): int { return use(value); }`, "ambiguous Marker interface ancestors"},
		{"generic class inherits implementation", `interface Root<T> { function read(): T; } interface Child<T> extends Root<T> {} class Box<T> implements Child<T> { constructor(public value: T) {} public function read(): T { return this.value; } } class Derived<T> extends Box<T> { constructor(value: T) { super(value); } } function use(value: Derived<int>): Root<int> { return value; }`, ""},
		{"inferred ancestor and class", `interface Root<T> { function read(): T; } interface Child<T> extends Root<T> {} class Box implements Child<int> { public function read(): int { return 3; } } function read<T>(value: Root<T>): T { return value.read(); } function use(value: Child<string>): string { return read(value); } function create(): int { return read(new Box()); }`, ""},
		{"missing inherited method", `interface Root { function read(): int; } interface Child extends Root {} class Bad implements Child {}`, "missing method read"},
		{"private implementation", `interface Root { function read(): int; } interface Child extends Root {} class Bad implements Child { private function read(): int { return 1; } }`, "method read must be public"},
		{"static implementation", `interface Root { function read(): int; } interface Child extends Root {} class Bad implements Child { public static function read(): int { return 1; } }`, "method read cannot be static"},
		{"inherited mismatch", `interface Root<T> { function read(): T; } interface Child extends Root<string> {} class Bad implements Child { public function read(): int { return 1; } }`, "incompatible signature"},
		{"base conflict", `interface A { function read(): int; } interface B { function read(): string; } interface C extends A, B {}`, "inherits incompatible signatures for method read"},
		{"own conflict", `interface A { function read(): int; } interface B extends A { function read(): string; }`, "inherits incompatible signatures for method read"},
		{"generic diamond conflict", `interface A<T> { function read(): T; } interface B extends A<int> {} interface C extends A<string> {} interface D extends B, C {}`, "inherits incompatible signatures for method read"},
		{"duplicate", `interface A {} interface B extends A, A {}`, "duplicate extended interface A"},
		{"self cycle", `interface A<T> extends A<T[]> {}`, "interface inheritance cycle"},
		{"indirect cycle", `interface A extends B {} interface B extends C {} interface C extends A {}`, "interface inheritance cycle"},
		{"not an interface", `class A {} interface B extends A {}`, "interface extends expects a source interface"},
		{"Go interface base", `import go io from "io"; interface B extends io.Reader {}`, ""},
		{"nullable base", `interface A {} interface B extends A | null {}`, "interface extends expects a source interface"},
		{"constraint base", `constraint Number = ~int; interface B extends Number {}`, "can only be used after 'extends' in a type parameter"},
		{"wrong ancestor instantiation", `interface A<T> {} interface B<T> extends A<T> {} function use(value: B<int>): A<string> { return value; }`, "cannot use B<int> as A<string>"},
		{"class contract parameter identity", `interface Marker<T> {} class Box<T> implements Marker<T> {} function use<T, U>(value: Box<U>): Marker<T> { return value; }`, "cannot use Box<U> as Marker<T>"},
		{"no implicit structural conformance", `interface A { function read(): int; } interface B { function read(): int; } function use(value: B): A { return value; }`, "cannot use B as A"},
		{"parent cannot become child", `interface A {} interface B extends A {} function use(value: A): B { return value; }`, "cannot use A as B"},
		{"parent cannot capture child parameter", `interface Child<T> extends Parent {} interface Parent extends Root<T> {} interface Root<U> {}`, `unknown type "T"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if test.want == "" && len(diagnostics) != 0 || test.want != "" && !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}

func TestInterfaceInheritanceDeepDiamond(t *testing.T) {
	var input strings.Builder
	input.WriteString("interface Root<T> { function read(): T; }\n")
	left, right := "Root<T>", "Root<T>"
	for index := 0; index < 30; index++ {
		if index == 0 {
			fmt.Fprintln(&input, "interface Left0<T> extends Root<T> {} interface Right0<T> extends Root<T> {}")
		} else {
			fmt.Fprintf(&input, "interface Left%d<T> extends %s, %s {} interface Right%d<T> extends %s, %s {}\n", index, left, right, index, left, right)
		}
		left, right = fmt.Sprintf("Left%d<T>", index), fmt.Sprintf("Right%d<T>", index)
	}
	input.WriteString("function use<T>(value: Left29<T>): Root<T> { return value; }")
	if diagnostics := checkSource(t, input.String()); len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}
