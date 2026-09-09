package sema

import (
	goast "go/ast"
	goparser "go/parser"
	gotoken "go/token"
	gotypes "go/types"
	"strings"
	"testing"
)

func TestConstrainedRangeSemanticMatrix(t *testing.T) {
	for _, test := range []struct{ name, input, want string }{
		{"slice", `constraint Values = ~int[]; function sum<T extends Values>(values: T): int { let total = 0; for (const value of values) { total += value; } return total; }`, ""},
		{"array", `constraint Values = ~[3]int; function sum<T extends Values>(values: T): int { let total = 0; for (const [index, value] of values) { total += index + value; } return total; }`, ""},
		{"pointer array", `constraint Values = *[3]int; function sum<T extends Values>(values: T): int { let total = 0; for (const value of values) { total += value; } return total; }`, ""},
		{"map", `constraint Values = ~Map<string, int>; function sum<T extends Values>(values: T): int { let total = 0; for (const [key, value] of values) { total += len(key) + value; } return total; }`, ""},
		{"string", `constraint Text = ~string; function sum<T extends Text>(text: T): int32 { let total: int32 = 0; for (const rune of text) { total += rune; } return total; }`, ""},
		{"named union same underlying", `type First = distinct int[]; type Second = distinct int[]; constraint Values = First | Second; function use<T extends Values>(values: T): void { for (const item of values) {} }`, ""},
		{"class element", `class Leaf { public value: int = 3; } constraint Leaves = ~Leaf[]; function sum<T extends Leaves>(values: T): int { let total = 0; for (const value of values) { total += value.value; } return total; }`, ""},
		{"class pointer array", `class Leaf { public value: int = 3; } constraint Leaves = *[2]Leaf; function sum<T extends Leaves>(values: T): int { let total = 0; for (const value of values) { total += value.value; } return total; }`, ""},
		{"generic class element", `class Leaf<T> { constructor(public value: T) {} } constraint Leaves = ~Leaf<int>[]; function sum<T extends Leaves>(values: T): int { let total = 0; for (const value of values) { total += value.value; } return total; }`, ""},
		{"struct element", `struct Point { public value: int; } constraint Points = ~Point[]; function sum<T extends Points>(values: T): int { let total = 0; for (const value of values) { total += value.value; } return total; }`, ""},
		{"generic struct element", `struct Point<V> { public value: V; } constraint Points = ~Point<int>[]; function sum<T extends Points>(values: T): int { let total = 0; for (const value of values) { total += value.value; } return total; }`, ""},
		{"interface element", `interface Reader { function read(): int; } constraint Readers = ~Reader[]; function sum<T extends Readers>(values: T): int { let total = 0; for (const value of values) { total += value.read(); } return total; }`, ""},
		{"channel union", `constraint Channels = GoChannel<int> | GoReceiveChannel<int>; function sum<T extends Channels>(values: T): int { let total=0; for(const value of values) {total+=value;} return total; }`, ""},
		{"send channel", `constraint Channels = GoSendChannel<int>; function use<T extends Channels>(values: T): void { for(const value of values) {} }`, "range type parameter requires"},
		{"channel pair", `constraint Channels = GoReceiveChannel<int>; function use<T extends Channels>(values: T): void { for(const [key,value] of values) {} }`, "channel range requires exactly one binding"},
		{"iterator", `constraint Seq = (yield: (value: int) => boolean) => void; function sum<T extends Seq>(values:T):int { let total=0;for(const value of values){total+=value;}return total; }`, ""},
		{"nullable iterator", `constraint Seq = (yield: (value: int) => boolean) => void; function use<T extends Seq>(values:T | null):void {for(const value of values){}}`, "nullable iterator must be narrowed before range"},
		{"positive array constructor", `class Leaf {} constraint Values = ~[2]int; class Box<T extends Values> { public leaf: Leaf; constructor(values: T) { for (const value of values) { this.leaf = new Leaf(); } } }`, ""},
		{"empty array constructor", `class Leaf {} constraint Values = ~[0]int; class Box<T extends Values> { public leaf: Leaf; constructor(values: T) { for (const value of values) { this.leaf = new Leaf(); } } }`, "must be initialized"},
		{"annotation cannot silently upcast", `class Base {} class Child extends Base {} constraint Values = ~Child[]; function use<T extends Values>(values: T): void { for (const value: Base of values) {} }`, "collection range binding must have the iterated type Child"},
		{"ordinary annotation cannot silently upcast", `class Base {} class Child extends Base {} function use(values: Child[]): void { for (const value: Base of values) {} }`, "collection range binding must have the iterated type Child"},
		{"mixed shapes", `constraint Values = ~int[] | ~[3]int; function use<T extends Values>(values: T): void { for (const item of values) {} }`, "range type parameter requires"},
		{"different lengths", `constraint Values = ~[2]int | ~[3]int; function use<T extends Values>(values: T): void { for (const item of values) {} }`, "range type parameter requires"},
		{"different elements", `constraint Values = ~int[] | ~string[]; function use<T extends Values>(values: T): void { for (const item of values) {} }`, "range type parameter requires"},
		{"unrestricted", `function use<T>(values: T): void { for (const item of values) {} }`, "range type parameter requires"},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.input)
			if test.want == "" && len(diagnostics) != 0 || test.want != "" && !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}

func TestConstrainedRangeGoCoreTypes(t *testing.T) {
	for _, test := range []struct{ name, constraint, want string }{
		{"intersection", "interface { ~[]int | ~string; ~[]int }", "[]int"},
		{"intersection selects later term", "interface { ~[]int | ~string; ~string }", "string"},
		{"comparable intersection", "interface { comparable; ~[]int | ~[2]int }", "[2]int"},
		{"empty intersection", "interface { ~[]int; ~[]string }", ""},
		{"unrestricted", "any", ""},
		{"channels", "interface { ~chan int | ~<-chan int }", "<-chan int"},
		{"send only", "interface { ~chan<- int }", ""},
		{"conflicting channels", "interface { ~chan<- int | ~<-chan int }", ""},
		{"different channel elements", "interface { ~chan int | ~<-chan string }", ""},
		{"iterator", "interface { ~func(func(int) bool) }", "func(func(int) bool)"},
		{"different arrays", "interface { ~[2]int | ~[3]int }", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			set := gotoken.NewFileSet()
			file, err := goparser.ParseFile(set, "constraint.go", "package fixture; type Constraint "+test.constraint+"; func use[T Constraint]() {}", 0)
			if err != nil {
				t.Fatal(err)
			}
			pkg, err := (&gotypes.Config{GoVersion: "go1.23"}).Check("fixture", set, []*goast.File{file}, nil)
			if err != nil {
				t.Fatal(err)
			}
			parameter := pkg.Scope().Lookup("use").Type().(*gotypes.Signature).TypeParams().At(0)
			core := goRangeCoreType(parameter)
			got := ""
			if core != nil {
				got = core.String()
			}
			if got != test.want {
				t.Fatalf("core = %q, want %q", got, test.want)
			}
		})
	}
}
