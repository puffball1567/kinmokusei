package sema

import (
	"strings"
	"testing"
)

func TestDistinctTypeEmbedding(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, input, want string }{
		{"value method", `type Count=distinct int;public function read(this:Count):int{return int(this);}struct Outer{Count;}function use(v:Outer):int{return v.read();}`, ""},
		{"value method expression", `type Count=distinct int;public function read(this:Count):int{return int(this);}struct Outer{Count;}function use(v:Outer):int{const read=Outer.read;return read(v);}`, ""},
		{"pointer receiver via embedded pointer", `type Count=distinct int;public function read(this:*Count):int{return int(*this);}struct Outer{*Count;}function use(v:Outer):int{return Outer.read(v);}`, ""},
		{"pointer receiver on temporary", `type Count=distinct int;public function read(this:*Count):int{return int(*this);}struct Outer{Count;}function use():int{return Outer{Count:Count(1)}.read();}`, "requires an addressable"},
		{"nested field", `struct S{public n:int;}type Record=distinct S;struct Outer{Record;}function use(v:Outer):int{return v.n;}`, ""},
		{"verified structural field", `type Record=distinct { text:string };struct Outer{Record;}function use(v:Outer):string{return v.text;}`, ""},
		{"underlying Go struct field", `import go image from "image";type Point=distinct image.Point;struct Outer{Point;}function use(v:Outer):int{return v.X;}`, ""},
		{"underlying Go struct methods are not inherited", `import go image from "image";type Point=distinct image.Point;struct Outer{Point;}function use(v:Outer):image.Point{return v.Add(image.Point{});}`, "has no field"},
		{"underlying embedded methods", `struct Leaf{public function read():int{return 1;}}struct S{Leaf;}type Record=distinct S;struct Outer{Record;}function use(v:Outer):int{return v.read()+v.Record.read();}`, ""},
		{"base methods are not inherited", `struct S{public function read():int{return 1;}}type Record=distinct S;struct Outer{Record;}function use(v:Outer):int{return v.read();}`, "has no field"},
		{"distinguish declared types", `type A=distinct int; type B=distinct int; struct Outer{A;B;}function use(v:Outer):A{return v.B;}`, "cannot use B as A"},
		{"distinct pointer is invalid", `type Ref=distinct *int;struct Outer{Ref;}`, "cannot embed a distinct type with pointer or interface storage"},
		{"distinct interface is invalid", `import go io from "io";type Writer=distinct io.Writer;struct Outer{Writer;}`, "cannot embed a distinct type with pointer or interface storage"},
		{"embedded layout cycle", `type View=distinct Node;struct Node{View;}`, "recursively contains itself by value"},
		{"recursive pointer layout", `type View=distinct Node;struct Node{*View;}`, ""},
		{"emitted field collision", `type Count=distinct int;struct Outer{Count;public count:int;}`, "share the generated Go name"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.input), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%q want=%q", got, test.want)
			}
		})
	}
}
