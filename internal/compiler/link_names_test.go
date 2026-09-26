package compiler

import (
	"reflect"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/lexer"
	"github.com/puffball1567/kinmokusei/internal/parser"
)

func TestLexicalLinkNamesPreserveSourceAST(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		source string
		names  []string
	}{
		{`function f<T>(p:T):void{const a=()=>b();const b=():int=>1;let [x,y]=pair();for(let i=0;i<2;i++){const inner=i;}for(const n of [1]){}const arrow=(arg:int):int=>arg;}`, []string{"T", "p", "a", "b", "x", "y", "i", "inner", "n", "arrow", "arg"}},
		{`class C<U>{constructor(public param:U){}public function method<V>(arg:V):void{const local=arg;}}struct S<W>{public value:W;}alias A<X>=X[];interface I<Y>{function read(p:Y):Y;}`, []string{"U", "param", "V", "arg", "local", "W", "X", "Y"}},
		{`function f(ch:GoChannel<int>):void{try{throw new Exception("x");}catch(failure:Exception){}select{case const [value,open]=<-ch{}default{}}}`, []string{"ch", "failure", "value", "open"}},
	} {
		parse := func() *ast.Program {
			tokens, ld := lexer.Lex("names.km", test.source)
			program, pd := parser.Parse(tokens)
			if len(ld) != 0 || len(pd) != 0 {
				t.Fatalf("parse: %v %v", ld, pd)
			}
			return program
		}
		program, original := parse(), parse()
		names := lexicalLinkNames(map[string]*ast.Program{"names.km": program})
		for _, name := range test.names {
			if !names[name] {
				t.Errorf("missing binding %s: %v", name, names)
			}
		}
		if !reflect.DeepEqual(program, original) {
			t.Fatalf("discovery mutated AST: %s", test.source)
		}
		if names["f"] || names["C"] || names["read"] {
			t.Fatalf("global or member name treated as lexical binding: %v", names)
		}
	}
}
