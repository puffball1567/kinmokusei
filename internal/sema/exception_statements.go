package sema

import (
	"fmt"
	gotypes "go/types"
	"sort"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) checkTryStatement(stmt *ast.TryStmt) {
	c.usesExceptions = true
	stmt.HandlesReturn = c.exceptionDepth == 0
	if stmt.HandlesReturn {
		stmt.ReturnType = typeRefFromType(c.result, stmt.Span)
	}
	entry := c.snapshotNullableFlow()

	c.restoreNullableFlow(entry)
	c.checkExceptionBlock(stmt.Body)
	tryFlow := c.snapshotNullableFlow()
	continuing := []nullableFlowSnapshot{}
	if !statementDefinitelyStopsBlock(stmt.Body) {
		continuing = append(continuing, tryFlow)
	}

	seenCatchTypes := []Type{}
	for _, clause := range stmt.Catches {
		catchType := c.resolveType(clause.Type)
		if catchType.Kind != Invalid && !c.isAssignable(builtins["error"], catchType) {
			c.report(clause.Type.Span, fmt.Sprintf("catch binding type must implement error, got %s", catchType.String()))
		}
		if catchType.Kind != Invalid && c.isAssignable(builtins["error"], catchType) {
			for _, earlier := range seenCatchTypes {
				if c.catchTypeCovers(earlier, catchType) {
					c.report(clause.Type.Span, fmt.Sprintf("catch for %s is unreachable because an earlier catch for %s already handles it", catchType.String(), earlier.String()))
					break
				}
			}
			seenCatchTypes = append(seenCatchTypes, catchType)
		}
		if catchType.Kind == Class {
			clause.MatchingClasses = append(clause.MatchingClasses, catchType.Name)
			for name := range c.classes {
				if name != catchType.Name && c.classExtends(name, catchType.Name) {
					clause.MatchingClasses = append(clause.MatchingClasses, name)
				}
			}
			sort.Strings(clause.MatchingClasses[1:])
		}
		c.restoreNullableFlow(c.mergeNullableFlow(entry, entry, tryFlow))
		c.pushScope()
		if clause.Name != "_" {
			c.declareCatchLocal(clause, catchType)
		}
		c.catchTargets = append(c.catchTargets, stmt.Span.Start.Offset)
		c.checkExceptionBlock(clause.Body)
		c.catchTargets = c.catchTargets[:len(c.catchTargets)-1]
		catchFlow := c.snapshotNullableFlow()
		c.popScope()
		if !statementDefinitelyStopsBlock(clause.Body) {
			continuing = append(continuing, catchFlow)
		}
	}

	c.restoreNullableFlow(c.mergeNullableFlow(entry, continuing...))
	if stmt.FinallyBody != nil {
		// finally also runs while an exception is propagating from any point in
		// the try/catch path, so it must not inherit facts established only by a
		// normally completing path.
		current := c.snapshotNullableFlow()
		c.restoreNullableFlow(c.mergeNullableFlow(entry, entry, current))
		c.checkExceptionBlock(stmt.FinallyBody)
	}
	stmt.Terminal = statementDefinitelyStopsBlock(stmt)
}

func (c *Checker) catchTypeCovers(earlier, current Type) bool {
	if exactType(earlier, builtins["error"]) || earlier.Kind == Class && earlier.Name == "Exception" {
		return true
	}
	if earlier.Kind == Class && current.Kind == Class {
		return earlier.Name == current.Name || c.classExtends(current.Name, earlier.Name)
	}
	earlierGo, earlierIsGo := goTypeOf(earlier)
	currentGo, currentIsGo := goTypeOf(current)
	return earlierIsGo && currentIsGo && gotypes.AssignableTo(currentGo, earlierGo)
}

func (c *Checker) checkExceptionBlock(block *ast.BlockStmt) {
	previousLoopDepth := c.loopDepth
	previousBreakableDepth := c.breakableDepth
	c.loopDepth = 0
	c.breakableDepth = 0
	c.exceptionDepth++
	c.checkBlock(block, true)
	c.exceptionDepth--
	c.loopDepth = previousLoopDepth
	c.breakableDepth = previousBreakableDepth
}
