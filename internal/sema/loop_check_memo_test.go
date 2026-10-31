package sema

import (
	"reflect"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func loopMemoChecker() *Checker {
	value := Type{Kind: GoPointer, Element: &Type{Kind: Int, Name: "int"}}
	maybe := Type{Kind: Nullable, Element: &value}
	return &Checker{
		scopes:     []map[string]valueSymbol{{"p": {declaredType: maybe, typeInfo: value}}},
		memberFlow: map[memberFlowKey]memberFlowState{}, memberTypes: map[memberFlowKey]Type{},
	}
}

func TestLoopCheckMemoPreservesExitAndDiagnostics(t *testing.T) {
	c := loopMemoChecker()
	statement := &ast.WhileStmt{}
	entry := c.snapshotNullableFlow()
	calls := 0
	iteration := func() (nullableFlowSnapshot, bool) {
		calls++
		symbol := c.scopes[0]["p"]
		symbol.typeInfo = symbol.declaredType
		c.scopes[0]["p"] = symbol
		c.report(source.Span{Path: "memo.km"}, "body diagnostic")
		return c.snapshotNullableFlow(), true
	}
	c.checkLoopFixedPoint(statement, entry, iteration)
	exit := c.snapshotNullableFlow()
	diagnostics := append(c.diagnostics[:0:0], c.diagnostics...)
	checked := calls
	c.restoreNullableFlow(entry)
	c.diagnostics = nil
	c.checkLoopFixedPoint(statement, c.snapshotNullableFlow(), iteration)
	if calls != checked || !reflect.DeepEqual(exit, c.snapshotNullableFlow()) || !reflect.DeepEqual(diagnostics, c.diagnostics) {
		t.Fatalf("repeated loop changed its checked exit or diagnostics: calls=%d/%d diagnostics=%v", calls, checked, c.diagnostics)
	}
}

func TestLoopCheckMemoRequiresCompleteEntry(t *testing.T) {
	for name, change := range map[string]func(*Checker){
		"nullable fact": func(c *Checker) { s := c.scopes[0]["p"]; s.typeInfo = s.declaredType; c.scopes[0]["p"] = s },
		"escape":        func(c *Checker) { s := c.scopes[0]["p"]; s.flowEscaped = true; c.scopes[0]["p"] = s },
		"cause": func(c *Checker) {
			s := c.scopes[0]["p"]
			s.flowInvalidationCause = "another cause"
			c.scopes[0]["p"] = s
		},
		"task":          func(c *Checker) { s := c.scopes[0]["p"]; s.taskState = taskPending; c.scopes[0]["p"] = s },
		"declaration":   func(c *Checker) { s := c.scopes[0]["p"]; s.declarationSpan.Path = "other.km"; c.scopes[0]["p"] = s },
		"declared type": func(c *Checker) { s := c.scopes[0]["p"]; s.declaredType.Name = "another type"; c.scopes[0]["p"] = s },
		"member fact":   func(c *Checker) { c.memberFlow[memberFlowKey{path: "item"}] = memberFlowState{nonNull: true} },
		"member type":   func(c *Checker) { c.memberTypes[memberFlowKey{path: "item"}] = builtins["int"] },
		"reachability":  func(c *Checker) { c.suppressFlowEffects++ },
	} {
		t.Run(name, func(t *testing.T) {
			c := loopMemoChecker()
			statement := &ast.WhileStmt{}
			_, complete := c.memoizeLoopCheck(statement, c.snapshotNullableFlow())
			complete()
			change(c)
			if reused, _ := c.memoizeLoopCheck(statement, c.snapshotNullableFlow()); reused {
				t.Fatal("a different semantic entry reused a previous result")
			}
		})
	}
}

func TestLoopCheckMemoKeepsExternalCollectors(t *testing.T) {
	for name, effect := range map[string]func(*Checker){
		"outer break": func(c *Checker) {
			c.loopFlowContexts[0].breaks = append(c.loopFlowContexts[0].breaks, c.snapshotNullableFlow())
		},
		"outer continue": func(c *Checker) {
			c.loopFlowContexts[0].continues = append(c.loopFlowContexts[0].continues, c.snapshotNullableFlow())
		},
		"switch break": func(c *Checker) {
			c.breakFlowContexts[0].breaks = append(c.breakFlowContexts[0].breaks, c.snapshotNullableFlow())
		},
		"capture":             func(c *Checker) { c.capturedWrites[0][source.Span{Path: "capture.km"}] = source.Span{Path: "write.km"} },
		"member capture":      func(c *Checker) { c.capturedMemberWrites[0] = source.Span{Path: "write.km"} },
		"exception collector": func(c *Checker) { c.exceptionFlows = append(c.exceptionFlows, &exceptionFlowContext{}) },
		"label collector":     func(c *Checker) { c.taskLabelFlows = map[source.Span]*taskLabelFlow{{}: {}} },
	} {
		t.Run(name, func(t *testing.T) {
			c := loopMemoChecker()
			c.loopFlowContexts = []loopFlowContext{{}}
			c.breakFlowContexts = []breakFlowContext{{}}
			c.capturedWrites = []map[source.Span]source.Span{{}}
			c.capturedMemberWrites = []source.Span{{}}
			statement := &ast.WhileStmt{}
			_, complete := c.memoizeLoopCheck(statement, c.snapshotNullableFlow())
			effect(c)
			complete()
			if _, stored := c.loopCheckMemos[statement]; stored {
				t.Fatal("loop with effects on enclosing collectors was cached")
			}
		})
	}
}

func TestLoopCheckMemoCallableIsolation(t *testing.T) {
	c := loopMemoChecker()
	statement := &ast.WhileStmt{}
	_, complete := c.memoizeLoopCheck(statement, c.snapshotNullableFlow())
	complete()
	outer := c.enterCallableFlow()
	if len(c.loopCheckMemos) != 0 {
		t.Fatal("enclosing loop cache leaked into another callable")
	}
	c.leaveCallableFlow(outer)
	if _, present := c.loopCheckMemos[statement]; !present {
		t.Fatal("enclosing loop cache was not restored")
	}
}
