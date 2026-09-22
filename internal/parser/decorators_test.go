package parser

import (
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestDecoratorSyntaxAndTargets(t *testing.T) {
	program, count := parseSource(t, `@api.Controller("/users")
export class Users {
  @Field("name") public name:string="";
  constructor(@Inject("users") private service:int){}
  @Get("/:id") public function find(@Param("id") id:string):string{return id;}
  @Read public get title():string{return this.name;}
}`)
	if count != 0 {
		t.Fatalf("diagnostics=%d", count)
	}
	class := program.Declarations[0].(*ast.ClassDecl)
	if len(program.Decorators) != 6 || len(class.Decorators) != 1 || len(class.Fields[0].Decorators) != 1 || len(class.Constructor.Parameters[0].Decorators) != 1 || len(class.Methods[0].Decorators) != 1 || len(class.Methods[0].Parameters[0].Decorators) != 1 || len(class.Methods[1].Decorators) != 1 {
		t.Fatalf("decorators not attached: %+v", class)
	}
	call := class.Decorators[0].Expression.(*ast.CallExpr)
	member := call.Callee.(*ast.MemberExpr)
	if member.Name != "Controller" || len(call.Arguments) != 1 || class.Decorators[0].Span.Start.Offset != 0 || len(program.Exports) != 1 {
		t.Fatal("application identity/span/export lost")
	}
}

func TestDecoratorOrderingAndSpeculation(t *testing.T) {
	program, count := parseSource(t, `export @First @Second(1) class C{} function f():void{const a=(@Arg x:int)=>x;}`)
	if count != 0 || len(program.Decorators) != 3 {
		t.Fatalf("count=%d decorators=%d", count, len(program.Decorators))
	}
	class := program.Declarations[0].(*ast.ClassDecl)
	if len(class.Decorators) != 2 || class.Decorators[0].Expression.(*ast.IdentifierExpr).Name != "First" {
		t.Fatal("decorator order lost")
	}
}

func TestMalformedDecorators(t *testing.T) {
	for _, input := range []string{`@`, `@api. class C{}`, `@D( class C{}`, `@D function f():void{}`, `class C{@}`, `class C{constructor(@){}}`} {
		t.Run(input, func(t *testing.T) {
			_, count := parseSource(t, input)
			if count == 0 {
				t.Fatal("expected diagnostics")
			}
		})
	}
}
