package sema

import (
	goast "go/ast"
	goparser "go/parser"
	"go/token"
	gotypes "go/types"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestGoCallContractRequiresFunctionIdentityAndSignature(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, path, source string
		want               bool
	}{
		{"exact", "reflect", `func DeepEqual(a,b any)bool{return false}`, true},
		{"different package", "example.test/reflect", `func DeepEqual(a,b any)bool{return false}`, false},
		{"different function", "reflect", `func Other(a,b any)bool{return false}`, false},
		{"function variable", "reflect", `var DeepEqual func(a,b any)bool`, false},
		{"wrong parameters", "reflect", `func DeepEqual(a,b int)bool{return false}`, false},
		{"wrong arity", "reflect", `func DeepEqual(a any)bool{return false}`, false},
		{"variadic", "reflect", `func DeepEqual(a any,b ...any)bool{return false}`, false},
		{"generic", "reflect", `func DeepEqual[T any](a,b any)bool{return false}`, false},
		{"alias result", "reflect", `func DeepEqual(a,b any)any{return a}`, false},
		{"multiple results", "reflect", `func DeepEqual(a,b any)(bool,any){return false,a}`, false},
		{"defined boolean", "reflect", `type Flag bool;func DeepEqual(a,b any)Flag{return false}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			set := token.NewFileSet()
			file, err := goparser.ParseFile(set, "contract.go", "package reflect\n"+test.source, 0)
			if err != nil {
				t.Fatal(err)
			}
			pkg, err := (&gotypes.Config{}).Check(test.path, set, []*goast.File{file}, nil)
			if err != nil {
				t.Fatal(err)
			}
			function, _ := pkg.Scope().Lookup("DeepEqual").(*gotypes.Func)
			contract, ok := trustedGoCallContract(function)
			if ok != test.want || ok && contract.borrowedParameters != 2 {
				t.Fatalf("contract = %#v, %v; want match %v", contract, ok, test.want)
			}
			if function != nil {
				local := gotypes.NewFunc(token.NoPos, pkg, "DeepEqual", function.Type().(*gotypes.Signature))
				if _, ok := trustedGoCallContract(local); ok {
					t.Fatal("unbound/local function acquired the package function contract")
				}
			}
		})
	}
}

func TestBorrowedGoCallRejectsUnresolvedStorage(t *testing.T) {
	callee := &ast.IdentifierExpr{Name: "checked"}
	c := &Checker{goCallContracts: map[ast.Expression]goCallContract{callee: {borrowedParameters: 2, resultKind: gotypes.Bool}}}
	call := &ast.CallExpr{Callee: callee}
	expected := Type{Kind: GoInterface, GoType: gotypes.NewInterfaceType(nil, nil).Complete()}
	unresolved := gotypes.NewNamed(gotypes.NewTypeName(token.NoPos, nil, "Pending", nil), nil, nil)
	if c.borrowedGoCallArgumentAssignable(call, 0, expected, Type{Kind: GoNamed, GoType: unresolved}) {
		t.Fatal("unfinished native storage accepted")
	}
	for _, index := range []int{-1, 2} {
		if c.borrowedGoCallArgumentAssignable(call, index, expected, builtins["string"]) {
			t.Fatal("parameter outside the checked contract accepted")
		}
	}
}
