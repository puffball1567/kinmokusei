package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) validateLabels(body *ast.BlockStmt) {
	if body == nil {
		return
	}
	labels := map[string]*ast.LabeledStmt{}
	used := map[string]bool{}
	var walk func(ast.Statement, func(ast.Statement))
	walk = func(statement ast.Statement, visit func(ast.Statement)) {
		if statement == nil {
			return
		}
		visit(statement)
		switch statement := statement.(type) {
		case *ast.LabeledStmt:
			if statement == nil {
				return
			}
			walk(statement.Statement, visit)
		case *ast.BlockStmt:
			if statement == nil {
				return
			}
			for _, nested := range statement.Statements {
				walk(nested, visit)
			}
		case *ast.IfStmt:
			walk(statement.Then, visit)
			walk(statement.Else, visit)
		case *ast.WhileStmt:
			walk(statement.Body, visit)
		case *ast.ForStmt:
			walk(statement.Body, visit)
		case *ast.ForRangeStmt:
			walk(statement.Body, visit)
		case *ast.SelectStmt:
			for index := range statement.Cases {
				walk(statement.Cases[index].Body, visit)
			}
		case *ast.ValueSwitchStmt:
			for index := range statement.Cases {
				walk(statement.Cases[index].Body, visit)
			}
		case *ast.TypeSwitchStmt:
			for index := range statement.Cases {
				walk(statement.Cases[index].Body, visit)
			}
		case *ast.TryStmt:
			walk(statement.Body, visit)
			for _, clause := range statement.Catches {
				walk(clause.Body, visit)
			}
			if statement.FinallyBody != nil {
				walk(statement.FinallyBody, visit)
			}
		}
	}
	walk(body, func(statement ast.Statement) {
		labeled, ok := statement.(*ast.LabeledStmt)
		if !ok {
			return
		}
		if previous, duplicate := labels[labeled.Label]; duplicate {
			c.report(labeled.LabelSpan, fmt.Sprintf("duplicate label %q; first declared at %d:%d", labeled.Label, previous.LabelSpan.Start.Line, previous.LabelSpan.Start.Column))
			return
		}
		switch labeled.Statement.(type) {
		case *ast.VariableDecl, *ast.MultiVariableDecl:
			c.report(labeled.LabelSpan, fmt.Sprintf("label %q cannot be attached to a variable declaration", labeled.Label))
		}
		labels[labeled.Label] = labeled
	})

	type controlLocation struct {
		blocks []*ast.BlockStmt
		region string
	}
	labelLocations := map[*ast.LabeledStmt]controlLocation{}
	branchLocations := map[*ast.BranchStmt]controlLocation{}
	var locate func(ast.Statement, []*ast.BlockStmt, string)
	locate = func(statement ast.Statement, blocks []*ast.BlockStmt, region string) {
		if statement == nil {
			return
		}
		switch statement := statement.(type) {
		case *ast.LabeledStmt:
			if statement == nil {
				return
			}
			labelLocations[statement] = controlLocation{blocks: append([]*ast.BlockStmt(nil), blocks...), region: region}
			locate(statement.Statement, blocks, region)
		case *ast.BranchStmt:
			branchLocations[statement] = controlLocation{blocks: append([]*ast.BlockStmt(nil), blocks...), region: region}
		case *ast.BlockStmt:
			if statement == nil {
				return
			}
			nestedBlocks := append(append([]*ast.BlockStmt(nil), blocks...), statement)
			for _, nested := range statement.Statements {
				locate(nested, nestedBlocks, region)
			}
		case *ast.IfStmt:
			locate(statement.Then, blocks, region)
			locate(statement.Else, blocks, region)
		case *ast.WhileStmt:
			locate(statement.Body, blocks, region)
		case *ast.ForStmt:
			locate(statement.Body, blocks, region)
		case *ast.ForRangeStmt:
			locate(statement.Body, blocks, region)
		case *ast.SelectStmt:
			for index := range statement.Cases {
				locate(statement.Cases[index].Body, blocks, region)
			}
		case *ast.ValueSwitchStmt:
			for index := range statement.Cases {
				locate(statement.Cases[index].Body, blocks, region)
			}
		case *ast.TypeSwitchStmt:
			for index := range statement.Cases {
				locate(statement.Cases[index].Body, blocks, region)
			}
		case *ast.TryStmt:
			base := fmt.Sprintf("try:%d", statement.Span.Start.Offset)
			locate(statement.Body, blocks, base+":body")
			for index, clause := range statement.Catches {
				locate(clause.Body, blocks, fmt.Sprintf("%s:catch:%d", base, index))
			}
			if statement.FinallyBody != nil {
				locate(statement.FinallyBody, blocks, base+":finally")
			}
		}
	}
	locate(body, nil, "root")

	var validate func(ast.Statement, []*ast.LabeledStmt)
	validate = func(statement ast.Statement, enclosing []*ast.LabeledStmt) {
		if statement == nil {
			return
		}
		switch statement := statement.(type) {
		case *ast.BranchStmt:
			if statement.Label == "" {
				return
			}
			target, exists := labels[statement.Label]
			if !exists {
				c.report(statement.LabelSpan, fmt.Sprintf("undefined label %q", statement.Label))
				return
			}
			statement.ResolvedDeclaration = target.LabelSpan
			used[statement.Label] = true
			if statement.Kind == ast.GotoBranch {
				gotoLocation := branchLocations[statement]
				targetLocation := labelLocations[target]
				if gotoLocation.region != targetLocation.region {
					c.report(statement.LabelSpan, fmt.Sprintf("goto %q cannot cross a try, catch, or finally boundary", statement.Label))
					return
				}
				if !blockPathContains(targetLocation.blocks, gotoLocation.blocks) {
					c.report(statement.LabelSpan, fmt.Sprintf("goto %q cannot jump into a nested block", statement.Label))
					return
				}
				if statement.Span.Start.Offset < target.LabelSpan.Start.Offset && len(targetLocation.blocks) != 0 {
					targetBlock := targetLocation.blocks[len(targetLocation.blocks)-1]
					for _, candidate := range targetBlock.Statements {
						if candidate.GetSpan().Start.Offset <= statement.Span.Start.Offset || candidate.GetSpan().Start.Offset >= target.LabelSpan.Start.Offset {
							continue
						}
						switch declaration := candidate.(type) {
						case *ast.VariableDecl:
							c.report(statement.LabelSpan, fmt.Sprintf("goto %q jumps over declaration of %q", statement.Label, declaration.Name))
							return
						case *ast.MultiVariableDecl:
							name := "_"
							for _, binding := range declaration.Bindings {
								if binding.Name != "_" {
									name = binding.Name
									break
								}
							}
							c.report(statement.LabelSpan, fmt.Sprintf("goto %q jumps over declaration of %q", statement.Label, name))
							return
						}
					}
				}
				return
			}
			enclosingTarget := false
			for index := len(enclosing) - 1; index >= 0; index-- {
				if enclosing[index] == target {
					enclosingTarget = true
					break
				}
			}
			if !enclosingTarget {
				c.report(statement.LabelSpan, fmt.Sprintf("label %q does not enclose this branch", statement.Label))
				return
			}
			if statement.Kind == ast.ContinueBranch && !isContinueLabelTarget(target.Statement) {
				c.report(statement.LabelSpan, fmt.Sprintf("continue label %q must target a loop", statement.Label))
			} else if statement.Kind == ast.BreakBranch && !isBreakLabelTarget(target.Statement) {
				c.report(statement.LabelSpan, fmt.Sprintf("break label %q must target a loop, switch, or select", statement.Label))
			}
		case *ast.LabeledStmt:
			if statement == nil {
				return
			}
			validate(statement.Statement, append(enclosing, statement))
		case *ast.BlockStmt:
			if statement == nil {
				return
			}
			for _, nested := range statement.Statements {
				validate(nested, enclosing)
			}
		case *ast.IfStmt:
			validate(statement.Then, enclosing)
			validate(statement.Else, enclosing)
		case *ast.WhileStmt:
			validate(statement.Body, enclosing)
		case *ast.ForStmt:
			validate(statement.Body, enclosing)
		case *ast.ForRangeStmt:
			validate(statement.Body, enclosing)
		case *ast.SelectStmt:
			for index := range statement.Cases {
				validate(statement.Cases[index].Body, enclosing)
			}
		case *ast.ValueSwitchStmt:
			for index := range statement.Cases {
				validate(statement.Cases[index].Body, enclosing)
			}
		case *ast.TypeSwitchStmt:
			for index := range statement.Cases {
				validate(statement.Cases[index].Body, enclosing)
			}
		case *ast.TryStmt:
			validate(statement.Body, enclosing)
			for _, clause := range statement.Catches {
				validate(clause.Body, enclosing)
			}
			if statement.FinallyBody != nil {
				validate(statement.FinallyBody, enclosing)
			}
		}
	}
	validate(body, nil)
	for name, label := range labels {
		if !used[name] {
			c.report(label.LabelSpan, fmt.Sprintf("label %q is declared but not used", name))
		}
	}
}

func blockPathContains(prefix, path []*ast.BlockStmt) bool {
	if len(prefix) > len(path) {
		return false
	}
	for index := range prefix {
		if prefix[index] != path[index] {
			return false
		}
	}
	return true
}

func isContinueLabelTarget(statement ast.Statement) bool {
	switch statement.(type) {
	case *ast.WhileStmt, *ast.ForStmt, *ast.ForRangeStmt:
		return true
	default:
		return false
	}
}

func isBreakLabelTarget(statement ast.Statement) bool {
	if isContinueLabelTarget(statement) {
		return true
	}
	switch statement.(type) {
	case *ast.SelectStmt, *ast.ValueSwitchStmt, *ast.TypeSwitchStmt:
		return true
	default:
		return false
	}
}
