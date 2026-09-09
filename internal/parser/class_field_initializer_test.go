package parser

import (
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestClassFieldInitializersParse(t *testing.T) {
	program, count := parseSource(t, `class Box<T> { public values: T[] = []; private count: int = 1 + 2; public plain: int; constructor() {} }`)
	if count != 0 {
		t.Fatalf("diagnostics = %d", count)
	}
	class := program.Declarations[0].(*ast.ClassDecl)
	if len(class.Fields) != 3 || class.Fields[0].Initializer == nil || class.Fields[1].Initializer == nil || class.Fields[2].Initializer != nil || class.Constructor == nil {
		t.Fatalf("class = %#v", class)
	}
	for _, input := range []string{
		`class Bad { public value: int = ; }`,
		`class Bad { public value: int =`,
		`class Bad { public value: int = 1 public next: int; }`,
		`struct Bad { public value: int = 1; }`,
		`class Bad { public static value: int = 1; }`,
	} {
		if _, count := parseSource(t, input); count == 0 {
			t.Errorf("accepted %q", input)
		}
	}
}
