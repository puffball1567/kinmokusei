package ast

import (
	"testing"

	"ontama.local/ontama/internal/source"
)

func TestTypeRefShapePredicates(t *testing.T) {
	plain := TypeRef{Name: "int"}
	array := TypeRef{Element: &plain}
	length := int64(3)
	fixedArray := TypeRef{Element: &plain, FixedLength: &length}
	function := TypeRef{Parameters: []TypeRef{plain}, Return: &plain}
	object := TypeRef{Object: true}
	pointer := TypeRef{Pointee: &plain}
	tests := []struct {
		name                                                     string
		ref                                                      TypeRef
		isFunction, isArray, isSlice, isFixedArray, isObjectType bool
		isPointer, isSpecified                                   bool
	}{
		{"empty", TypeRef{}, false, false, false, false, false, false, false},
		{"plain", plain, false, false, false, false, false, false, true},
		{"slice", array, false, true, true, false, false, false, true},
		{"fixed array", fixedArray, false, true, false, true, false, false, true},
		{"function", function, true, false, false, false, false, false, true},
		{"object", object, false, false, false, false, true, false, true},
		{"pointer", pointer, false, false, false, false, false, true, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.ref.IsFunction() != test.isFunction || test.ref.IsArray() != test.isArray || test.ref.IsSlice() != test.isSlice || test.ref.IsFixedArray() != test.isFixedArray || test.ref.IsObject() != test.isObjectType || test.ref.IsPointer() != test.isPointer || test.ref.IsSpecified() != test.isSpecified {
				t.Fatalf("predicates for %#v = function:%v array:%v slice:%v fixed:%v object:%v pointer:%v specified:%v", test.ref, test.ref.IsFunction(), test.ref.IsArray(), test.ref.IsSlice(), test.ref.IsFixedArray(), test.ref.IsObject(), test.ref.IsPointer(), test.ref.IsSpecified())
			}
		})
	}
}

func TestEveryNodeReturnsItsSpan(t *testing.T) {
	span := source.Span{Path: "node.otm", Start: source.Position{Offset: 1, Line: 2, Column: 3}, End: source.Position{Offset: 4, Line: 2, Column: 6}}
	nodes := []Node{
		ImportDecl{Span: span}, TypeRef{Span: span},
		&FunctionDecl{Span: span}, &ClassDecl{Span: span}, &InterfaceDecl{Span: span}, &VariableDecl{Span: span},
		&BlockStmt{Span: span}, &ReturnStmt{Span: span}, &ThrowStmt{Span: span}, &TryStmt{Span: span}, &IfStmt{Span: span}, &ExpressionStmt{Span: span},
		&AssignmentStmt{Span: span}, &IncDecStmt{Span: span}, &MultiVariableDecl{Span: span}, &MultiAssignmentStmt{Span: span}, &WhileStmt{Span: span}, &ForStmt{Span: span}, &ForRangeStmt{Span: span}, &SelectCase{Span: span}, &SelectStmt{Span: span}, &ValueSwitchCase{Span: span}, &ValueSwitchStmt{Span: span}, &TypeSwitchCase{Span: span}, &TypeSwitchStmt{Span: span}, &BranchStmt{Span: span}, &CallControlStmt{Span: span}, &DetachStmt{Span: span}, &ChannelSendStmt{Span: span},
		&IdentifierExpr{Span: span}, &LiteralExpr{Span: span}, &UnaryExpr{Span: span}, &BinaryExpr{Span: span}, &GoTypeAssertionExpr{Span: span}, &ClassUpcastExpr{Span: span}, &PropagateExpr{Span: span}, &TaskStartExpr{Span: span}, &AwaitExpr{Span: span},
		&CallExpr{Span: span}, &ArrowExpr{Span: span}, &ArrayLiteralExpr{Span: span}, &ObjectLiteralExpr{Span: span}, &GoCompositeLiteralExpr{Span: span},
		&MemberExpr{Span: span}, &IndexExpr{Span: span}, &SliceExpr{Span: span}, &NewExpr{Span: span},
	}
	for i, node := range nodes {
		if got := node.GetSpan(); got != span {
			t.Errorf("node %d (%T) span = %#v, want %#v", i, node, got, span)
		}
	}
}

func TestNodeCategoryImplementations(t *testing.T) {
	declarations := []Declaration{&FunctionDecl{}, &ClassDecl{}, &InterfaceDecl{}, &VariableDecl{}}
	statements := []Statement{&VariableDecl{}, &MultiVariableDecl{}, &BlockStmt{}, &ReturnStmt{}, &ThrowStmt{}, &TryStmt{}, &IfStmt{}, &ExpressionStmt{}, &AssignmentStmt{}, &IncDecStmt{}, &MultiAssignmentStmt{}, &WhileStmt{}, &ForStmt{}, &ForRangeStmt{}, &SelectStmt{}, &ValueSwitchStmt{}, &TypeSwitchStmt{}, &BranchStmt{}, &CallControlStmt{}, &DetachStmt{}, &ChannelSendStmt{}}
	expressions := []Expression{&IdentifierExpr{}, &LiteralExpr{}, &UnaryExpr{}, &BinaryExpr{}, &GoTypeAssertionExpr{}, &ClassUpcastExpr{}, &PropagateExpr{}, &TaskStartExpr{}, &AwaitExpr{}, &CallExpr{}, &ArrowExpr{}, &ArrayLiteralExpr{}, &ObjectLiteralExpr{}, &GoCompositeLiteralExpr{}, &MemberExpr{}, &IndexExpr{}, &SliceExpr{}, &NewExpr{}}
	if len(declarations) != 4 || len(statements) != 21 || len(expressions) != 18 {
		t.Fatal("node category matrix is incomplete")
	}
}
