package sema

import (
	"go/importer"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/lexer"
	"github.com/puffball1567/kinmokusei/internal/parser"
)

func TestParameterBoundaryContracts(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		prefix, parameter string
		want              bool
	}{
		"class":                {"class User{}", "value:User", true},
		"nullable class":       {"class User{}", "value:User|null", false},
		"class alias":          {"class User{}alias U=User;", "value:U", true},
		"interface":            {"interface View{function read():int;}", "value:View", true},
		"anonymous interface":  {"", "value:interface{read():int;}", true},
		"empty Go interface":   {"", "value:interface{}", false},
		"function":             {"", "value:()=>int", true},
		"nullable function":    {"alias F=()=>int;", "value:F|null", false},
		"function alias":       {"alias F=()=>int;", "value:F", true},
		"defined function":     {"type F=distinct ()=>int;", "value:F", true},
		"class rest":           {"class User{}", "...value:User[]", true},
		"function rest":        {"alias F=()=>int;", "...value:F[]", true},
		"nullable rest":        {"class User{}alias U=User|null;", "...value:U[]", false},
		"raw pointer":          {"", "value:*int", false},
		"raw error":            {"", "value:error", false},
		"raw callback":         {"import go http from \"net/http\";", "value:http.HandlerFunc", false},
		"raw callback alias":   {"import go http from \"net/http\";alias F=http.HandlerFunc;", "value:F", false},
		"defined raw callback": {"import go http from \"net/http\";type F=distinct http.HandlerFunc;", "value:F", false},
		"plain slice":          {"class User{}", "value:User[]", false},
		"blank discard":        {"", "_:()=>int", false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tokens, diagnostics := lexer.Lex("entry.km", test.prefix+"function check("+test.parameter+"):void{}")
			if len(diagnostics) != 0 {
				t.Fatal(diagnostics)
			}
			program, diagnostics := parser.Parse(tokens)
			if len(diagnostics) != 0 {
				t.Fatal(diagnostics)
			}
			check := program.Declarations[len(program.Declarations)-1].(*ast.FunctionDecl)
			for pass := 0; pass < 2; pass++ {
				// Rechecking must overwrite stale editor metadata in both directions.
				check.Parameters[0].NonNull = !test.want
				if diagnostics := CheckScopedWithGoImporterAndPolicy(program, nil, importer.Default(), GoInteropPolicy{}); len(diagnostics) != 0 {
					t.Fatal(diagnostics)
				}
				if got := check.Parameters[0].NonNull; got != test.want {
					t.Fatalf("check=%v want=%v", got, test.want)
				}
			}
		})
	}
}
