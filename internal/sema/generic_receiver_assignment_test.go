package sema

import (
	"strings"
	"testing"
)

func TestGenericParameterReceiverAssignment(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{"class receiver", `class Box<T>{public function bad():T{return this;}}`, "cannot use Box<T> as T"},
		{"struct receiver", `struct Box<T>{public function bad():T{return this;}}`, "cannot use Box<T> as T"},
		{"class comparable", `class Box<T extends comparable>{public function bad():T{return this;}}`, "cannot use Box<T> as T"},
		{"class value", `class Leaf{} function bad<T>():T{return new Leaf();}`, "cannot use Leaf as T"},
		{"struct value", `struct Leaf{public value:int;} function bad<T>():T{return Leaf{value:1};}`, "cannot use Leaf as T"},
		{"exact pointer constraint", `class Leaf{} constraint OnlyLeaf=Leaf; function makeLeaf<T extends OnlyLeaf>():T{return new Leaf();}`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if test.want == "" && len(diagnostics) != 0 || test.want != "" && !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics=%v want=%q", diagnostics, test.want)
			}
		})
	}
}
