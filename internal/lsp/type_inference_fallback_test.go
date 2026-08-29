package lsp

import (
	"testing"

	"ontama.local/ontama/internal/ast"
	"ontama.local/ontama/internal/source"
)

func testSpan(path string, start, end int) source.Span {
	return source.Span{
		Path:  path,
		Start: source.Position{Offset: start, Line: 1, Column: start + 1},
		End:   source.Position{Offset: end, Line: 1, Column: end + 1},
	}
}

func TestCallableDeclarationReturnFallbackMatrix(t *testing.T) {
	const path = "fallback.otm"
	box := ast.TypeRef{Name: "Box"}
	functionSpan := testSpan(path, 10, 15)
	classMethodSpan := testSpan(path, 20, 25)
	structMethodSpan := testSpan(path, 30, 35)
	externalMethodSpan := testSpan(path, 40, 45)
	interfaceMethodSpan := testSpan(path, 50, 55)
	program := &ast.Program{Declarations: []ast.Declaration{
		&ast.FunctionDecl{Name: "build", NameSpan: functionSpan, ReturnType: box},
		&ast.ClassDecl{Name: "Factory", Methods: []*ast.MethodDecl{{Name: "makeClass", NameSpan: classMethodSpan, ReturnType: box}}},
		&ast.StructDecl{Name: "Builder", Methods: []*ast.MethodDecl{{Name: "makeStruct", NameSpan: structMethodSpan, ReturnType: box}}},
		&ast.MethodDecl{Name: "makeExternal", NameSpan: externalMethodSpan, ReturnType: box, External: true},
		&ast.InterfaceDecl{Name: "Provider", Methods: []ast.InterfaceMethod{{Name: "makeInterface", NameSpan: interfaceMethodSpan, ReturnType: box}}},
	}}
	tests := []struct {
		name   string
		callee ast.Expression
		wantOK bool
	}{
		{"resolved function", &ast.IdentifierExpr{Name: "ignored", ResolvedDeclaration: functionSpan}, true},
		{"function name fallback", &ast.IdentifierExpr{Name: "build"}, true},
		{"resolved class method", &ast.MemberExpr{Name: "ignored", ResolvedDeclaration: classMethodSpan}, true},
		{"resolved struct method", &ast.MemberExpr{Name: "ignored", ResolvedDeclaration: structMethodSpan}, true},
		{"resolved external method", &ast.MemberExpr{Name: "ignored", ResolvedDeclaration: externalMethodSpan}, true},
		{"resolved interface method", &ast.MemberExpr{Name: "ignored", ResolvedDeclaration: interfaceMethodSpan}, true},
		{"unique class method fallback", &ast.MemberExpr{Name: "makeClass"}, true},
		{"unique struct method fallback", &ast.MemberExpr{Name: "makeStruct"}, true},
		{"unique external method fallback", &ast.MemberExpr{Name: "makeExternal"}, true},
		{"unique interface method fallback", &ast.MemberExpr{Name: "makeInterface"}, true},
		{"unknown function", &ast.IdentifierExpr{Name: "missing"}, false},
		{"unsupported callee", &ast.LiteralExpr{Kind: ast.IntegerLiteral, Text: "1"}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := callableDeclarationReturn(program, test.callee)
			if ok != test.wantOK {
				t.Fatalf("callableDeclarationReturn() ok = %v, want %v", ok, test.wantOK)
			}
			if ok && got.Name != "Box" {
				t.Fatalf("callableDeclarationReturn() = %#v, want Box", got)
			}
		})
	}

	ambiguous := &ast.Program{Declarations: []ast.Declaration{
		&ast.ClassDecl{Name: "First", Methods: []*ast.MethodDecl{{Name: "make", ReturnType: box}}},
		&ast.ClassDecl{Name: "Second", Methods: []*ast.MethodDecl{{Name: "make", ReturnType: ast.TypeRef{Name: "Other"}}}},
	}}
	if got, ok := callableDeclarationReturn(ambiguous, &ast.MemberExpr{Name: "make"}); ok {
		t.Fatalf("ambiguous member fallback = %#v, want unresolved", got)
	}
}

func TestSimpleSignatureTypeMatrix(t *testing.T) {
	tests := []struct {
		input        string
		wantName     string
		wantPointee  string
		wantNullable bool
		wantOK       bool
	}{
		{"Widget", "Widget", "", false, true},
		{"Widget | null", "Widget", "", true, true},
		{"*Widget", "", "Widget", false, true},
		{"*Widget | null", "", "Widget", true, true},
		{"", "", "", false, false},
		{"void", "", "", false, false},
		{"<invalid>", "", "", false, false},
		{"Result<Widget>", "", "", false, false},
		{"Widget[]", "", "", false, false},
		{"(int) => Widget", "", "", false, false},
		{"*Result<Widget>", "", "", false, false},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, ok := simpleSignatureType(test.input)
			if ok != test.wantOK {
				t.Fatalf("simpleSignatureType(%q) ok = %v, want %v", test.input, ok, test.wantOK)
			}
			if !ok {
				return
			}
			if got.Name != test.wantName || got.Nullable != test.wantNullable {
				t.Fatalf("simpleSignatureType(%q) = %#v", test.input, got)
			}
			if test.wantPointee != "" && (got.Pointee == nil || got.Pointee.Name != test.wantPointee) {
				t.Fatalf("simpleSignatureType(%q) pointee = %#v", test.input, got.Pointee)
			}
		})
	}
}

func TestExpressionCompletionTypeFallbackMatrix(t *testing.T) {
	const path = "expression.otm"
	box := ast.TypeRef{Name: "Box"}
	functionSpan := testSpan(path, 1, 6)
	program := &ast.Program{Declarations: []ast.Declaration{
		&ast.FunctionDecl{Name: "build", NameSpan: functionSpan, ReturnType: box},
	}}
	tests := []struct {
		name     string
		value    ast.Expression
		wantName string
		wantOK   bool
	}{
		{"new class", &ast.NewExpr{ClassName: "Box", ResolvedDeclaration: functionSpan}, "Box", true},
		{"Go composite", &ast.GoCompositeLiteralExpr{Type: ast.TypeRef{Name: "Request", Go: true}}, "Request", true},
		{"class upcast", &ast.ClassUpcastExpr{TargetClass: "Base"}, "Base", true},
		{"propagated value", &ast.PropagateExpr{ValueType: box}, "Box", true},
		{"unresolved propagated value", &ast.PropagateExpr{}, "", false},
		{"awaited value", &ast.AwaitExpr{ValueType: box}, "Box", true},
		{"unresolved awaited value", &ast.AwaitExpr{}, "", false},
		{"declared call", &ast.CallExpr{Callee: &ast.IdentifierExpr{Name: "build", ResolvedDeclaration: functionSpan}}, "Box", true},
		{"signature call", &ast.CallExpr{Callee: &ast.IdentifierExpr{Name: "external"}, Signature: &ast.CallableSignature{Result: "External"}}, "External", true},
		{"unsupported signature result", &ast.CallExpr{Callee: &ast.IdentifierExpr{Name: "external"}, Signature: &ast.CallableSignature{Result: "External[]"}}, "", false},
		{"unresolved call", &ast.CallExpr{Callee: &ast.IdentifierExpr{Name: "missing"}}, "", false},
		{"literal", &ast.LiteralExpr{Kind: ast.IntegerLiteral, Text: "1"}, "", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := expressionCompletionType(program, test.value)
			if ok != test.wantOK || got.Name != test.wantName {
				t.Fatalf("expressionCompletionType() = (%#v, %v), want name=%q ok=%v", got, ok, test.wantName, test.wantOK)
			}
		})
	}

	explicit, ok := variableType(program, &ast.VariableDecl{Type: box})
	if !ok || explicit.Name != "Box" {
		t.Fatalf("explicit variable type = (%#v, %v)", explicit, ok)
	}
	resolved, ok := variableType(program, &ast.VariableDecl{ResolvedType: ast.TypeRef{Name: "Resolved"}})
	if !ok || resolved.Name != "Resolved" {
		t.Fatalf("resolved variable type = (%#v, %v)", resolved, ok)
	}
	inferred, ok := variableType(program, &ast.VariableDecl{Value: &ast.NewExpr{ClassName: "Inferred"}})
	if !ok || inferred.Name != "Inferred" {
		t.Fatalf("inferred variable type = (%#v, %v)", inferred, ok)
	}
}

func TestFunctionValueSignatureFormattingMatrix(t *testing.T) {
	integer := ast.TypeRef{Name: "int"}
	stringType := ast.TypeRef{Name: "string"}
	functionType := ast.TypeRef{Parameters: []ast.TypeRef{integer, stringType}, Return: &integer}
	fromType := signatureFromTypeRef(functionType)
	if len(fromType.ParameterNames) != 0 || len(fromType.ParameterTypes) != 2 || fromType.ParameterTypes[0] != "int" || fromType.ParameterTypes[1] != "string" || fromType.Result != "int" {
		t.Fatalf("function type signature = %#v", fromType)
	}

	arrow := &ast.ArrowExpr{
		Parameters:         []ast.Parameter{{Name: "left", Type: integer}, {Name: "right", Type: stringType}},
		ResolvedReturnType: integer,
	}
	fromArrow := signatureFromArrow(arrow)
	if len(fromArrow.ParameterNames) != 2 || fromArrow.ParameterNames[0] != "left" || fromArrow.ParameterNames[1] != "right" || fromArrow.ParameterTypes[0] != "int" || fromArrow.ParameterTypes[1] != "string" || fromArrow.Result != "int" {
		t.Fatalf("inferred arrow signature = %#v", fromArrow)
	}
	explicit := stringType
	arrow.ReturnType = &explicit
	if got := signatureFromArrow(arrow); got.Result != "string" {
		t.Fatalf("explicit arrow signature = %#v", got)
	}
}

func TestVisibleCallableDeclarationMatrix(t *testing.T) {
	const path = "callable_scope.otm"
	integer := ast.TypeRef{Name: "int"}
	functionType := ast.TypeRef{Parameters: []ast.TypeRef{integer}, Return: &integer}
	arrow := &ast.ArrowExpr{Parameters: []ast.Parameter{{Name: "value", Type: integer}}, ResolvedReturnType: integer}
	parameter := ast.Parameter{Name: "operation", Type: functionType}
	local := &ast.VariableDecl{Name: "operation", Value: arrow, Span: testSpan(path, 20, 40)}
	body := &ast.BlockStmt{Statements: []ast.Statement{local}, Span: testSpan(path, 10, 90)}
	tests := []struct {
		name    string
		program *ast.Program
		offset  int
	}{
		{"function parameter", &ast.Program{Declarations: []ast.Declaration{&ast.FunctionDecl{Parameters: []ast.Parameter{parameter}, Body: body, Span: testSpan(path, 0, 100)}}}, 50},
		{"function local", &ast.Program{Declarations: []ast.Declaration{&ast.FunctionDecl{Body: body, Span: testSpan(path, 0, 100)}}}, 50},
		{"external method parameter", &ast.Program{Declarations: []ast.Declaration{&ast.MethodDecl{Parameters: []ast.Parameter{parameter}, Body: body, Span: testSpan(path, 0, 100)}}}, 50},
		{"external method local", &ast.Program{Declarations: []ast.Declaration{&ast.MethodDecl{Body: body, Span: testSpan(path, 0, 100)}}}, 50},
		{"constructor parameter", &ast.Program{Declarations: []ast.Declaration{&ast.ClassDecl{Constructor: &ast.ConstructorDecl{Parameters: []ast.Parameter{parameter}, Body: body, Span: testSpan(path, 5, 95)}, Span: testSpan(path, 0, 100)}}}, 50},
		{"constructor local", &ast.Program{Declarations: []ast.Declaration{&ast.ClassDecl{Constructor: &ast.ConstructorDecl{Body: body, Span: testSpan(path, 5, 95)}, Span: testSpan(path, 0, 100)}}}, 50},
		{"class method parameter", &ast.Program{Declarations: []ast.Declaration{&ast.ClassDecl{Methods: []*ast.MethodDecl{{Parameters: []ast.Parameter{parameter}, Body: body, Span: testSpan(path, 5, 95)}}, Span: testSpan(path, 0, 100)}}}, 50},
		{"class method local", &ast.Program{Declarations: []ast.Declaration{&ast.ClassDecl{Methods: []*ast.MethodDecl{{Body: body, Span: testSpan(path, 5, 95)}}, Span: testSpan(path, 0, 100)}}}, 50},
		{"global function type", &ast.Program{Declarations: []ast.Declaration{&ast.VariableDecl{Name: "operation", Type: functionType}}}, 50},
		{"global arrow", &ast.Program{Declarations: []ast.Declaration{&ast.VariableDecl{Name: "operation", Value: arrow}}}, 50},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ref, gotArrow, ok := visibleCallable(test.program, path, test.offset, "operation")
			if !ok || (!ref.IsFunction() && gotArrow == nil) {
				t.Fatalf("visibleCallable() = (%#v, %#v, %v)", ref, gotArrow, ok)
			}
		})
	}
	if ref, gotArrow, ok := visibleCallable(&ast.Program{}, path, 50, "missing"); ok || ref.IsSpecified() || gotArrow != nil {
		t.Fatalf("missing callable = (%#v, %#v, %v)", ref, gotArrow, ok)
	}
}

func TestVisibleCallableNestedBlockMatrix(t *testing.T) {
	const path = "nested_callable.otm"
	integer := ast.TypeRef{Name: "int"}
	arrow := &ast.ArrowExpr{Parameters: []ast.Parameter{{Name: "value", Type: integer}}, ResolvedReturnType: integer}
	child := func(start, end int) *ast.BlockStmt {
		return &ast.BlockStmt{
			Statements: []ast.Statement{&ast.VariableDecl{Name: "operation", Value: arrow, Span: testSpan(path, start+1, start+4)}},
			Span:       testSpan(path, start, end),
		}
	}
	tests := []struct {
		name   string
		item   ast.Statement
		offset int
	}{
		{"block", child(10, 90), 50},
		{"if then", &ast.IfStmt{Then: child(10, 90), Span: testSpan(path, 5, 95)}, 50},
		{"if else", &ast.IfStmt{Then: child(10, 30), Else: child(40, 90), Span: testSpan(path, 5, 95)}, 50},
		{"try body", &ast.TryStmt{Body: child(10, 90), Span: testSpan(path, 5, 95)}, 50},
		{"catch body", &ast.TryStmt{Body: child(10, 30), Catches: []*ast.CatchClause{{Body: child(40, 90)}}, Span: testSpan(path, 5, 95)}, 50},
		{"finally body", &ast.TryStmt{Body: child(10, 20), FinallyBody: child(40, 90), Span: testSpan(path, 5, 95)}, 50},
		{"while body", &ast.WhileStmt{Body: child(10, 90), Span: testSpan(path, 5, 95)}, 50},
		{"for body", &ast.ForStmt{Body: child(10, 90), Span: testSpan(path, 5, 95)}, 50},
		{"range body", &ast.ForRangeStmt{Body: child(10, 90), Span: testSpan(path, 5, 95)}, 50},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			block := &ast.BlockStmt{Statements: []ast.Statement{test.item}, Span: testSpan(path, 0, 100)}
			ref, gotArrow, ok := visibleCallableInBlock(block, path, test.offset, "operation")
			if !ok || ref.IsSpecified() || gotArrow == nil {
				t.Fatalf("visibleCallableInBlock() = (%#v, %#v, %v)", ref, gotArrow, ok)
			}
		})
	}
	if ref, gotArrow, ok := visibleCallableInBlock(nil, path, 50, "operation"); ok || ref.IsSpecified() || gotArrow != nil {
		t.Fatalf("nil block callable = (%#v, %#v, %v)", ref, gotArrow, ok)
	}
}
