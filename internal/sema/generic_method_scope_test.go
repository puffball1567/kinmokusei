package sema

import (
	"strings"
	"testing"
)

const genericMethodScopeBase = `
class Base<T> {
  constructor(public value: T) {}
  public function echo<U>(value: U): U { return value; }
  public function keep<U>(marker: U): T { return this.value; }
  public function replace<V>(value: T, marker: V): T { this.value = value; return this.value; }
}
`

func TestGenericMethodTypeParameterScopes(t *testing.T) {
	for _, source := range []string{
		`class Child<U> extends Base<U> { constructor(value: U) { super(value); } } function use(child: Child<string>): int { return child.echo<int>(1); }`,
		`class Child<U> extends Base<U> { constructor(value: U) { super(value); } } function use(child: Child<string>): string { return child.keep<int>(1); }`,
		`class Middle<U> extends Base<U> { constructor(value: U) { super(value); } } class Child<V> extends Middle<V> { constructor(value: V) { super(value); } public function relay<W>(value: W): W { return super.echo(value); } } function use(child: Child<string>): int { return child.relay(1); }`,
		`function use<U>(box: Base<U>): U { return box.keep<int>(1); }`,
		`function use<U>(box: Base<U>, value: U): U { return box.replace(value, 1); }`,
	} {
		t.Run(source, func(t *testing.T) {
			if diagnostics := checkSource(t, genericMethodScopeBase+source); len(diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", diagnostics)
			}
		})
	}
}

func TestGenericMethodCannotInferEnclosingTypeParameters(t *testing.T) {
	for _, source := range []string{
		`function bad<U>(box: Base<U>): int { return box.keep<int>(1); }`,
		`function bad<U>(box: Base<U>): string { return box.replace("bad", 1); }`,
	} {
		t.Run(source, func(t *testing.T) {
			diagnostics := checkSource(t, genericMethodScopeBase+source)
			if !strings.Contains(strings.Join(diagnostics, "\n"), "cannot use") {
				t.Fatalf("diagnostics = %v, want a type mismatch", diagnostics)
			}
		})
	}
}

func TestNativeCompositeTypeArgumentInference(t *testing.T) {
	for _, source := range []string{
		`class Box<T> { constructor(public value: T) {} } function get<T>(box: Box<T>): T { return box.value; } function use(box: Box<string>): string { return get(box); }`,
		`class Box<T> { constructor(public value: T) {} } class Child<A, B> extends Box<B> { constructor(value: B) { super(value); } } function get<T>(box: Box<T>): T { return box.value; } function use(box: Child<int, string>): string { return get(box); }`,
		`struct Box<T> { public value: T; } function get<T>(boxes: Map<string, Box<T>[]>): T { return boxes["key"][0].value; } function use(boxes: Map<string, Box<int>[]>): int { return get(boxes); }`,
		`interface Reader<T> { function read(): T; } function get<T>(reader: Reader<T>): T { return reader.read(); } function use(reader: Reader<string>): string { return get(reader); }`,
		`struct Box<T> { public value: T; } class Tools { public function get<U>(box: *Box<U>): U { return box.value; } } function use(box: *Box<string>): string { return new Tools().get(box); }`,
		`constraint Integer = ~int | ~int8; struct Box<T extends Integer> { public value: T; } function get<T extends Integer>(box: Box<T>): T { return box.value; } function use(box: Box<int8>): int8 { return get(box); }`,
	} {
		t.Run(source, func(t *testing.T) {
			if diagnostics := checkSource(t, source); len(diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", diagnostics)
			}
		})
	}
}

func TestNativeCompositeInferenceRejectsMismatches(t *testing.T) {
	for _, test := range []struct{ source, want string }{
		{`struct Box<T> { public value: T; } function choose<T>(left: Box<T>, right: Box<T>): T { return left.value; } function bad(left: Box<int>, right: Box<string>): int { return choose(left, right); }`, "was already inferred as int, not string"},
		{`struct Box<T> { public value: T; } struct Other<T> { public value: T; } function get<T>(box: Box<T>): T { return box.value; } function bad(other: Other<int>): int { return get<int>(other); }`, "cannot use"},
		{`class Box<T> { constructor(public value: T) {} } class Other<T> { constructor(public value: T) {} } function get<T>(box: Box<T>): T { return box.value; } function bad(other: Other<int>): int { return get<int>(other); }`, "cannot use"},
		{`constraint Integer = ~int; struct Box<T> { public value: T; } function get<T extends Integer>(box: Box<T>): T { return box.value; } function bad(box: Box<string>): string { return get(box); }`, "does not satisfy T type parameter constraint"},
	} {
		t.Run(test.want, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}
