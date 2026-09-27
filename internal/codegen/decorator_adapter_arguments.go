package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"
	"strconv"

	kinmokuseiAST "github.com/puffball1567/kinmokusei/internal/ast"
)

// Every dynamic argument crosses this checked boundary before a typed Go call.
func decoratorCheckedArguments(label string, parameters []kinmokuseiAST.TypeRef, contracts []string, variadic bool, arguments *goast.Ident, failure func(string) *goast.ReturnStmt) ([]goast.Stmt, []goast.Expr) {
	expectedCount := len(parameters)
	fixedCount := expectedCount
	if variadic {
		fixedCount--
	}
	argumentLabel := "arguments"
	if fixedCount == 1 {
		argumentLabel = "argument"
	}
	arityOperator := token.NEQ
	arityText := fmt.Sprintf("expects %d %s", expectedCount, argumentLabel)
	if variadic {
		arityOperator = token.LSS
		arityText = fmt.Sprintf("expects at least %d %s", fixedCount, argumentLabel)
	}
	statements := []goast.Stmt{&goast.IfStmt{
		Cond: &goast.BinaryExpr{
			X:  &goast.CallExpr{Fun: goast.NewIdent("len"), Args: []goast.Expr{arguments}},
			Op: arityOperator,
			Y:  &goast.BasicLit{Kind: token.INT, Value: strconv.Itoa(fixedCount)},
		},
		Body: &goast.BlockStmt{List: []goast.Stmt{failure(fmt.Sprintf("%s %s", label, arityText))}},
	}}
	callArguments := make([]goast.Expr, 0, expectedCount)
	for index, parameter := range parameters[:fixedCount] {
		name := goast.NewIdent(fmt.Sprintf("argument%d", index))
		ok := goast.NewIdent(fmt.Sprintf("argument%dOK", index))
		statements = append(statements,
			&goast.AssignStmt{
				Lhs: []goast.Expr{name, ok}, Tok: token.DEFINE,
				Rhs: []goast.Expr{decoratorDecode(&goast.IndexExpr{X: arguments, Index: &goast.BasicLit{Kind: token.INT, Value: strconv.Itoa(index)}}, parameter, contracts[index])},
			},
			&goast.IfStmt{Cond: &goast.UnaryExpr{Op: token.NOT, X: ok}, Body: &goast.BlockStmt{List: []goast.Stmt{
				failure(fmt.Sprintf("%s argument %d expects %s", label, index, decoratorTypeLabel(parameter))),
			}}},
		)
		callArguments = append(callArguments, name)
	}
	if variadic {
		parameter := parameters[expectedCount-1]
		restName := goast.NewIdent("variadicArguments")
		restCount := goast.NewIdent("variadicCount")
		restIndex := goast.NewIdent("variadicIndex")
		restValue := goast.NewIdent("variadicValue")
		restOK := goast.NewIdent("variadicValueOK")
		statements = append(statements,
			&goast.AssignStmt{
				Lhs: []goast.Expr{restCount}, Tok: token.DEFINE,
				Rhs: []goast.Expr{&goast.BinaryExpr{X: &goast.CallExpr{Fun: goast.NewIdent("len"), Args: []goast.Expr{arguments}}, Op: token.SUB, Y: &goast.BasicLit{Kind: token.INT, Value: strconv.Itoa(fixedCount)}}},
			},
			&goast.AssignStmt{
				Lhs: []goast.Expr{restName}, Tok: token.DEFINE,
				Rhs: []goast.Expr{&goast.CallExpr{Fun: goast.NewIdent("make"), Args: []goast.Expr{goType(parameter), restCount}}},
			},
		)
		element := parameter
		if parameter.Element != nil {
			element = *parameter.Element
		}
		argumentIndex := &goast.BinaryExpr{X: &goast.BasicLit{Kind: token.INT, Value: strconv.Itoa(fixedCount)}, Op: token.ADD, Y: restIndex}
		loopBody := &goast.BlockStmt{List: []goast.Stmt{
			&goast.AssignStmt{
				Lhs: []goast.Expr{restValue, restOK}, Tok: token.DEFINE,
				Rhs: []goast.Expr{decoratorDecode(&goast.IndexExpr{X: arguments, Index: argumentIndex}, element, contracts[expectedCount-1])},
			},
			&goast.IfStmt{Cond: &goast.UnaryExpr{Op: token.NOT, X: restOK}, Body: &goast.BlockStmt{List: []goast.Stmt{
				failure(fmt.Sprintf("%s variadic arguments expect %s", label, decoratorTypeLabel(element))),
			}}},
			&goast.AssignStmt{Lhs: []goast.Expr{&goast.IndexExpr{X: restName, Index: restIndex}}, Tok: token.ASSIGN, Rhs: []goast.Expr{restValue}},
		}}
		statements = append(statements, &goast.ForStmt{
			Init: &goast.AssignStmt{Lhs: []goast.Expr{restIndex}, Tok: token.DEFINE, Rhs: []goast.Expr{&goast.BasicLit{Kind: token.INT, Value: "0"}}},
			Cond: &goast.BinaryExpr{X: restIndex, Op: token.LSS, Y: restCount},
			Post: &goast.IncDecStmt{X: restIndex, Tok: token.INC},
			Body: loopBody,
		})
		callArguments = append(callArguments, restName)
	}
	return statements, callArguments
}
