package sema

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/lexer"
	"github.com/puffball1567/kinmokusei/internal/parser"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func TestConstructorAnalysisDoesNotMutateInputs(t *testing.T) {
	tokens, diagnostics := lexer.Lex("constructor.km", `
class Leaf {}
class Holder {
  private value: Leaf;
  constructor(ready: boolean) {
    if (ready) { this.value = new Leaf(); }
    else { this.value = new Leaf(); }
  }
}`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	program, diagnostics := parser.Parse(tokens)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	if diagnostics = Check(program); len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	class := program.Declarations[1].(*ast.ClassDecl)
	body := class.Constructor.Body
	before, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	required := map[string]source.Span{"value": class.Fields[0].NameSpan}
	initial := map[string]bool{"preset": true}
	analyzer := constructorInitializationAnalyzer{required: required}

	// One analyzer and checked AST can be reused without sharing branch facts.
	for iteration := 0; iteration < 8; iteration++ {
		t.Run("independent branch state", func(t *testing.T) {
			t.Parallel()
			flow := analyzer.block(body, initial)
			if !reflect.DeepEqual(flow.continuing, map[string]bool{"preset": true, "value": true}) {
				t.Fatalf("unexpected initialization: %v", flow.continuing)
			}
			flow.continuing["preset"] = false
			flow.continuing["local"] = true
			if !reflect.DeepEqual(initial, map[string]bool{"preset": true}) {
				t.Fatalf("analysis mutated input state: %v", initial)
			}
			if !reflect.DeepEqual(required, map[string]source.Span{"value": class.Fields[0].NameSpan}) {
				t.Fatalf("analysis mutated required fields: %v", required)
			}
			after, err := json.Marshal(body)
			if err != nil || string(after) != string(before) {
				t.Fatalf("analysis changed checked AST: %v", err)
			}
			other := constructorInitializationAnalyzer{required: map[string]source.Span{"other": {}}}
			if result := other.block(body, initial); !reflect.DeepEqual(result.continuing, initial) {
				t.Fatalf("required fields leaked between analyses: %v", result.continuing)
			}
		})
	}
}

func TestConstructorAnalysisDistinguishesEmptyAndTerminatedPaths(t *testing.T) {
	analyzer := constructorInitializationAnalyzer{}
	for _, body := range []*ast.BlockStmt{nil, {}} {
		if flow := analyzer.block(body, nil); flow.continuing == nil || len(flow.continuing) != 0 {
			t.Fatalf("empty body must continue with no initialized fields: %v", flow)
		}
	}
	for _, statement := range []ast.Statement{&ast.ReturnStmt{}, &ast.ThrowStmt{}} {
		flow := analyzer.block(&ast.BlockStmt{Statements: []ast.Statement{statement}}, nil)
		if flow.continuing != nil {
			t.Fatalf("terminating body must not have a continuing path: %v", flow)
		}
	}
}
