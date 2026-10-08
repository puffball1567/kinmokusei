package sema

import (
	"strings"
	"testing"
)

func TestNativeValuesSatisfyGoMethodConstraints(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`class Label {public function string():bstring{return "ok";}} function use():bstring{return show(new Label());}`,
		`class Label<T> {constructor(public value:T){} public function string():bstring{return "ok";}} function use():bstring{return show(new Label<int>(1));}`,
		`class Base {public virtual function string():bstring{return "base";}} class Child extends Base {public override function string():bstring{return "child";}} function use():bstring{return show(new Child());}`,
		`struct Label {public function string():bstring{return "ok";}} function use(value:Label):bstring{return show(value);}`,
		`struct Label<T> {public value:T;public function string():bstring{return "ok";}} function use(value:Label<int>):bstring{return show(value);}`,
		`struct Label {public pointer function string():bstring{return "ok";}} function use(value:*Label):bstring{return show(value);}`,
		`interface Label {function string():bstring;} function use(value:Label):bstring{return show(value);}`,
		`interface Label<T> {function string():bstring;} interface Derived<T> extends Label<T>{} function use(value:Derived<int>):bstring{return show(value);}`,
		`type Label=distinct int; public function string(this:Label):bstring{return "ok";} function use():bstring{return show(Label(1));}`,
	} {
		t.Run(input, func(t *testing.T) {
			input = `import go fmt from "fmt";function show<T extends fmt.Stringer>(value:T):bstring{return value.String();}` + input
			if diagnostics := checkSource(t, input); len(diagnostics) != 0 {
				t.Fatalf("%v", diagnostics)
			}
		})
	}
}

func TestNativeMethodConstraintRejections(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`class Label {} function use():bstring{return show(new Label());}`,
		`class Label {private function string():bstring{return "ok";}} function use():bstring{return show(new Label());}`,
		`class Label {public static function string():bstring{return "ok";}} function use():bstring{return show(new Label());}`,
		`class Label {public function string<T>():bstring{return "ok";}} function use():bstring{return show(new Label());}`,
		`class Label {public function string():int{return 1;}} function use():bstring{return show(new Label());}`,
		`class Label {public function string(value:int):bstring{return "ok";}} function use():bstring{return show(new Label());}`,
		`class Label {public function string():bstring{return "ok";}} function use(value:Label|null):bstring{return show(value);}`,
		`struct Label {public pointer function string():bstring{return "ok";}} function use(value:Label):bstring{return show(value);}`,
		`struct Label {private function string():bstring{return "ok";}} function use(value:Label):bstring{return show(value);}`,
		`class Label {public function string():string{return "ok";}} function use():bstring{return show(new Label());}`,
		`type Label=distinct int; public function string(this:Label):string{return "ok";} function use():bstring{return show(Label(1));}`,
	} {
		t.Run(input, func(t *testing.T) {
			input = `import go fmt from "fmt";function show<T extends fmt.Stringer>(value:T):bstring{return value.String();}` + input
			if diagnostics := strings.Join(checkSource(t, input), "\n"); !strings.Contains(diagnostics, "does not satisfy") {
				t.Fatalf("%s", diagnostics)
			}
		})
	}
}

func TestNativeMethodConstraintsRetainTypeSets(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, input, want string }{
		{"defined integer", `constraint A=fmt.Stringer&~int;type Label=distinct int;public function string(this:Label):bstring{return "ok";}function use():Label{return keep(Label(1));}`, ""},
		{"class is not integer", `constraint A=fmt.Stringer&~int;class Label{public function string():bstring{return "ok";}}function use():Label{return keep(new Label());}`, "does not satisfy"},
		{"comparable class", `constraint A=fmt.Stringer&comparable;class Label{public function string():bstring{return "ok";}}function use():Label{return keep(new Label());}`, ""},
		{"comparable struct", `constraint A=fmt.Stringer&comparable;struct Label{public value:int;public function string():bstring{return "ok";}}function use(value:Label):Label{return keep(value);}`, ""},
		{"noncomparable struct", `constraint A=fmt.Stringer&comparable;struct Label{public values:int[];public function string():bstring{return "ok";}}function use(value:Label):Label{return keep(value);}`, "does not satisfy"},
		{"comparable struct pointer", `constraint A=fmt.Stringer&comparable;struct Label{public values:int[];public function string():bstring{return "ok";}}function use(value:*Label):*Label{return keep(value);}`, ""},
		{"noncomparable defined slice", `constraint A=fmt.Stringer&comparable;type Label=distinct int[];public function string(this:Label):bstring{return "ok";}function use(value:Label):Label{return keep(value);}`, "does not satisfy"},
		{"nested comparable", `constraint B=fmt.Stringer&comparable;constraint A=B;type Label=distinct int[];public function string(this:Label):bstring{return "ok";}function use(value:Label):Label{return keep(value);}`, "does not satisfy"},
		{"nested methods", `constraint B=fmt.Stringer;constraint A=B&comparable;class Label{public function string():bstring{return "ok";}}function use():Label{return keep(new Label());}`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, `import go fmt from "fmt";function keep<T extends A>(value:T):T{return value;}`+test.input)
			if test.want == "" && len(diagnostics) != 0 || test.want != "" && !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics=%v want=%q", diagnostics, test.want)
			}
		})
	}
}

func TestNativeResultMethodsSatisfyGoConstraints(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`class Stream {public function close():Result<void>{return ok();}} function use():error{return close(new Stream());}`,
		`struct Stream {public function close():Result<void>{return ok();}} function use(value:Stream):error{return close(value);}`,
		`interface Stream {function close():Result<void>;} function use(value:Stream):error{return close(value);}`,
	} {
		t.Run(input, func(t *testing.T) {
			if diagnostics := checkSource(t, `import go io from "io";function close<T extends io.Closer>(value:T):error{return value.Close();}`+input); len(diagnostics) != 0 {
				t.Fatal(diagnostics)
			}
		})
	}
}
