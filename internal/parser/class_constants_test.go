package parser

import (
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestClassConstantsParse(t *testing.T) {
	t.Parallel()
	program, count := parseSource(t, "class C<T>{\npublic static const n:int=1\nprivate static const text:string=\"a\"\npublic static count:int=0\n}")
	if count != 0 {
		t.Fatalf("diagnostics=%d", count)
	}
	fields := program.Declarations[0].(*ast.ClassDecl).Fields
	if len(fields) != 3 || !fields[0].Constant || !fields[0].Static || !fields[1].Constant || fields[2].Constant || fields[0].Initializer == nil {
		t.Fatalf("fields=%#v", fields)
	}
	for _, input := range []string{
		`class C{public const n:int=1;}`,
		`class C{public static const n=1;}`,
		`class C{public static const :int=1;}`,
		`class C{public static const function f():int{return 1;}}`,
		`class C{public static virtual const n:int=1;}`,
		`class C{public static const n:int=;}`,
	} {
		if _, count := parseSource(t, input); count == 0 {
			t.Errorf("accepted %q", input)
		}
	}
}
