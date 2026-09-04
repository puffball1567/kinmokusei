package parser

import (
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestParsesCompoundAssignmentAndIncrementStatements(t *testing.T) {
	program, diagnostics := parseSource(t, `
function update(limit: int, items: int[], pointer: *int): int {
  let value = 1;
  value += 2;
  items[0] &^= value;
  (*pointer)++;
  for (; value < limit; value++) { items[0] >>= 1; }
  value--;
  return value;
}`)
	if diagnostics != 0 {
		t.Fatalf("diagnostics = %d", diagnostics)
	}
	statements := program.Declarations[0].(*ast.FunctionDecl).Body.Statements
	if assignment, ok := statements[1].(*ast.AssignmentStmt); !ok || assignment.Operator != "+=" {
		t.Fatalf("plus assignment = %#v", statements[1])
	}
	if assignment, ok := statements[2].(*ast.AssignmentStmt); !ok || assignment.Operator != "&^=" {
		t.Fatalf("and-not assignment = %#v", statements[2])
	}
	if update, ok := statements[3].(*ast.IncDecStmt); !ok || update.Operator != "++" {
		t.Fatalf("pointer increment = %#v", statements[3])
	}
	loop, ok := statements[4].(*ast.ForStmt)
	if !ok {
		t.Fatalf("loop = %#v", statements[4])
	}
	if post, ok := loop.Post.(*ast.IncDecStmt); !ok || post.Operator != "++" {
		t.Fatalf("loop post = %#v", loop.Post)
	}
	if body, ok := loop.Body.Statements[0].(*ast.AssignmentStmt); !ok || body.Operator != ">>=" {
		t.Fatalf("loop body = %#v", loop.Body.Statements[0])
	}
	if update, ok := statements[5].(*ast.IncDecStmt); !ok || update.Operator != "--" {
		t.Fatalf("decrement = %#v", statements[5])
	}
}

func TestUpdateStatementSyntaxFailureMatrix(t *testing.T) {
	tests := []string{
		`function bad(): void { ++value; }`,
		`function bad(value: int): int { return value++; }`,
		`function bad(): void { 1 += 2; }`,
		`function bad(): void { call()++; }`,
		`function bad(left: int, right: int): void { [left, right] += call(); }`,
		`function bad(value: int): void { value +=; }`,
		`function bad(value: int): void { value++ + 1; }`,
	}
	for _, source := range tests {
		if _, diagnostics := parseSource(t, source); diagnostics == 0 {
			t.Errorf("source parsed without diagnostics: %s", source)
		}
	}
}
