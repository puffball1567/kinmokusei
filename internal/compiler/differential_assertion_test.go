package compiler

import (
	"fmt"
	"go/ast"
	"path"
	"strconv"
)

type differentialOrigin uint8

const (
	differentialReference differentialOrigin = 1 << iota
	differentialGenerated
)

var goTestBuiltins = map[string]bool{
	"append": true, "cap": true, "clear": true, "close": true, "complex": true,
	"copy": true, "delete": true, "imag": true, "len": true, "make": true,
	"max": true, "min": true, "new": true, "panic": true, "print": true,
	"println": true, "real": true, "recover": true,
}

var testingFailureMethods = map[string]bool{
	"Error": true, "Errorf": true, "Fail": true, "FailNow": true,
	"Fatal": true, "Fatalf": true,
}

func validateDifferentialAssertions(file *ast.File, modulePath string) error {
	referenceAliases := map[string]bool{}
	generatedAliases := map[string]bool{}
	for _, imported := range file.Imports {
		importPath, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			return fmt.Errorf("invalid import path %s", imported.Path.Value)
		}
		alias := path.Base(importPath)
		if imported.Name != nil {
			alias = imported.Name.Name
		}
		if importPath == modulePath+"/reference" {
			referenceAliases[alias] = true
		}
		if importPath == modulePath {
			generatedAliases[alias] = true
		}
	}

	localFunctions := map[string]bool{}
	testingVariables := map[string]bool{}
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok {
			continue
		}
		localFunctions[function.Name.Name] = true
		if function.Type.Params == nil {
			continue
		}
		for _, field := range function.Type.Params.List {
			pointer, ok := field.Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			selector, ok := pointer.X.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "T" {
				continue
			}
			packageName, ok := selector.X.(*ast.Ident)
			if !ok || packageName.Name != "testing" {
				continue
			}
			for _, name := range field.Names {
				testingVariables[name.Name] = true
			}
		}
	}

	origins := map[string]differentialOrigin{}
	for changed := true; changed; {
		changed = false
		ast.Inspect(file, func(node ast.Node) bool {
			switch statement := node.(type) {
			case *ast.AssignStmt:
				changed = mergeAssignmentOrigins(statement.Lhs, statement.Rhs, origins, referenceAliases, generatedAliases, localFunctions) || changed
			case *ast.ValueSpec:
				left := make([]ast.Expr, len(statement.Names))
				for index, name := range statement.Names {
					left[index] = name
				}
				changed = mergeAssignmentOrigins(left, statement.Values, origins, referenceAliases, generatedAliases, localFunctions) || changed
			}
			return true
		})
	}

	var observed differentialOrigin
	assertions := 0
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if ok {
			observed |= differentialExpressionOrigin(call, origins, referenceAliases, generatedAliases, localFunctions)
		}
		statement, ok := node.(*ast.IfStmt)
		if !ok {
			return true
		}
		conditionOrigin := differentialExpressionOrigin(statement.Cond, origins, referenceAliases, generatedAliases, localFunctions)
		if conditionOrigin&(differentialReference|differentialGenerated) != differentialReference|differentialGenerated {
			return true
		}
		if blockContainsTestingFailure(statement.Body, testingVariables) {
			assertions++
		}
		return true
	})
	if observed&differentialReference == 0 {
		return fmt.Errorf("independent Go reference is imported but never evaluated")
	}
	if observed&differentialGenerated == 0 {
		return fmt.Errorf("generated API is never evaluated")
	}
	if assertions == 0 {
		return fmt.Errorf("no failing test assertion depends on both generated and independent Go results")
	}
	return nil
}

func mergeAssignmentOrigins(left, right []ast.Expr, origins map[string]differentialOrigin, referenceAliases, generatedAliases, localFunctions map[string]bool) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	combined := differentialOrigin(0)
	for _, expression := range right {
		combined |= differentialExpressionOrigin(expression, origins, referenceAliases, generatedAliases, localFunctions)
	}
	changed := false
	for index, target := range left {
		identifier, ok := target.(*ast.Ident)
		if !ok || identifier.Name == "_" {
			continue
		}
		origin := combined
		if len(left) == len(right) {
			origin = differentialExpressionOrigin(right[index], origins, referenceAliases, generatedAliases, localFunctions)
		}
		merged := origins[identifier.Name] | origin
		if merged != origins[identifier.Name] {
			origins[identifier.Name] = merged
			changed = true
		}
	}
	return changed
}

func differentialExpressionOrigin(expression ast.Expr, origins map[string]differentialOrigin, referenceAliases, generatedAliases, localFunctions map[string]bool) differentialOrigin {
	var result differentialOrigin
	ast.Inspect(expression, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.Ident:
			result |= origins[node.Name]
		case *ast.SelectorExpr:
			if qualifier, ok := node.X.(*ast.Ident); ok {
				if referenceAliases[qualifier.Name] {
					result |= differentialReference
				}
				if generatedAliases[qualifier.Name] {
					result |= differentialGenerated
				}
			}
		case *ast.CallExpr:
			if function, ok := node.Fun.(*ast.Ident); ok && !goTestBuiltins[function.Name] && !localFunctions[function.Name] {
				result |= differentialGenerated
			}
		}
		return true
	})
	return result
}

func blockContainsTestingFailure(block *ast.BlockStmt, testingVariables map[string]bool) bool {
	found := false
	ast.Inspect(block, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !testingFailureMethods[selector.Sel.Name] {
			return true
		}
		receiver, ok := selector.X.(*ast.Ident)
		if ok && testingVariables[receiver.Name] {
			found = true
		}
		return !found
	})
	return found
}
