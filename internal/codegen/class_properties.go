package codegen

import (
	goast "go/ast"
	"go/token"
	"strconv"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func generatePropertyReceiver(member *ast.MemberExpr) (goast.Expr, error) {
	if member.Super {
		return &goast.UnaryExpr{Op: token.AND, X: &goast.SelectorExpr{X: goast.NewIdent("this"), Sel: goast.NewIdent(member.SuperBase)}}, nil
	}
	return generateExpression(member.Object)
}

func propertyCall(receiver goast.Expr, name string, arguments ...goast.Expr) *goast.CallExpr {
	return &goast.CallExpr{Fun: &goast.SelectorExpr{X: receiver, Sel: goast.NewIdent(goName(name))}, Args: arguments}
}

// Ordinary property calls use the existing virtual wrapper, including its
// zero-value fallback. super bypasses dispatch and calls the inherited slot.
func propertyAccessorName(member *ast.MemberExpr, setter bool) string {
	name, owner := member.PropertyGetter, member.PropertyGetterOwner
	if setter {
		name, owner = member.PropertySetter, member.PropertySetterOwner
	}
	if member.Super && owner != "" {
		return virtualSlotName(owner, name)
	}
	return name
}

func generatePropertyAssignment(member *ast.MemberExpr, operator string, value ast.Expression) (goast.Stmt, error) {
	receiver, err := generatePropertyReceiver(member)
	if err != nil {
		return nil, err
	}
	var rhs goast.Expr
	if value != nil {
		rhs, err = generateExpression(value)
		if err != nil {
			return nil, err
		}
	}
	if operator == "" || operator == "=" {
		return &goast.ExprStmt{X: propertyCall(receiver, propertyAccessorName(member, true), rhs)}, nil
	}
	// A zero-argument closure is also a valid for-post statement. Fresh names
	// avoid capturing user identifiers in either the receiver or RHS (including
	// identifiers introduced by prior lowering passes).
	used := map[string]bool{}
	for _, expression := range []goast.Expr{receiver, rhs} {
		if expression != nil {
			goast.Inspect(expression, func(node goast.Node) bool {
				if identifier, ok := node.(*goast.Ident); ok {
					used[identifier.Name] = true
				}
				return true
			})
		}
	}
	fresh := func(base string) *goast.Ident {
		name := base
		for n := 1; used[name]; n++ {
			name = base + strconv.Itoa(n)
		}
		used[name] = true
		return goast.NewIdent(name)
	}
	object, current := fresh("__propertyReceiver"), fresh("__propertyValue")
	statements := []goast.Stmt{
		&goast.AssignStmt{Lhs: []goast.Expr{object}, Tok: token.DEFINE, Rhs: []goast.Expr{receiver}},
		&goast.AssignStmt{Lhs: []goast.Expr{current}, Tok: token.DEFINE, Rhs: []goast.Expr{propertyCall(object, propertyAccessorName(member, false))}},
	}
	if operator == "++" || operator == "--" {
		op := token.INC
		if operator == "--" {
			op = token.DEC
		}
		statements = append(statements, &goast.IncDecStmt{X: current, Tok: op})
	} else {
		statements = append(statements, &goast.AssignStmt{Lhs: []goast.Expr{current}, Tok: goAssignmentToken(operator), Rhs: []goast.Expr{rhs}})
	}
	statements = append(statements, &goast.ExprStmt{X: propertyCall(object, propertyAccessorName(member, true), current)})
	return &goast.ExprStmt{X: &goast.CallExpr{Fun: &goast.FuncLit{Type: &goast.FuncType{Params: &goast.FieldList{}}, Body: &goast.BlockStmt{List: statements}}}}, nil
}
