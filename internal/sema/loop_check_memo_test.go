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
		"presence fact": func(c *Checker) {
			s := c.scopes[0]["p"]
			s.presence = presenceFlow{required: true, checked: true}
			c.scopes[0]["p"] = s
		},
		"escape": func(c *Checker) { s := c.scopes[0]["p"]; s.flowEscaped = true; c.scopes[0]["p"] = s },
		"cause": func(c *Checker) {
			s := c.scopes[0]["p"]
			s.flowInvalidationCause = "another cause"
			c.scopes[0]["p"] = s
		},
		"task":                     func(c *Checker) { s := c.scopes[0]["p"]; s.taskState = taskPending; c.scopes[0]["p"] = s },
		"declaration":              func(c *Checker) { s := c.scopes[0]["p"]; s.declarationSpan.Path = "other.km"; c.scopes[0]["p"] = s },
		"declared type":            func(c *Checker) { s := c.scopes[0]["p"]; s.declaredType.Name = "another type"; c.scopes[0]["p"] = s },
		"member fact":              func(c *Checker) { c.memberFlow[memberFlowKey{path: "item"}] = memberFlowState{nonNull: true} },
		"member type":              func(c *Checker) { c.memberTypes[memberFlowKey{path: "item"}] = builtins["int"] },
		"reachability":             func(c *Checker) { c.suppressFlowEffects++ },
		"constructor":              func(c *Checker) { c.inConstructor = true },
		"initializer availability": func(c *Checker) { c.fieldInitializerAvailable = map[string]bool{"field": true} },
		"initializer depth":        func(c *Checker) { c.fieldInitializerArrowDepth++ },
		"label incoming": func(c *Checker) {
			c.taskLabelFlows = map[source.Span]*taskLabelFlow{{}: {incoming: map[source.Span]uint8{{}: taskPending}}}
		},
		"exception entry": func(c *Checker) {
			entry := c.snapshotNullableFlow()
			c.exceptionFlows = []*exceptionFlowContext{{entry: entry, flow: entry}}
		},
		"capture boundary": func(c *Checker) {
			c.callableScopeBases = []int{1}
			c.capturedWrites = []map[source.Span]source.Span{{}}
			c.capturedMemberWrites = []source.Span{{}}
		},
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

func TestLoopCheckMemoReplaysExternalCollectors(t *testing.T) {
	returned := &ast.ReturnStmt{}
	tried := &ast.TryStmt{}
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
		"capture": func(c *Checker) { c.capturedWrites[0][source.Span{Path: "capture.km"}] = source.Span{Path: "write.km"} },
		"member capture": func(c *Checker) {
			c.capturedMemberWrites[0] = source.Span{Path: "write.km", Start: source.Position{Line: 7}}
		},
		"return": func(c *Checker) {
			c.arrowReturns.values = append(c.arrowReturns.values, arrowReturnValue{returned, builtins["int"]})
			c.arrowReturns.tries = append(c.arrowReturns.tries, tried)
		},
		"exception return": func(c *Checker) {
			c.exceptionFlows[0].returns = append(c.exceptionFlows[0].returns, c.snapshotNullableFlow())
		},
		"exception join": func(c *Checker) {
			flow := c.snapshotNullableFlow()
			s := flow.scopes[0]["p"]
			s.typeInfo = s.declaredType
			flow.scopes[0]["p"] = s
			c.exceptionFlows[0].flow = c.mergeNullableFlow(c.exceptionFlows[0].entry, flow)
		},
		"label": func(c *Checker) {
			c.taskLabelFlows = map[source.Span]*taskLabelFlow{{}: {incoming: map[source.Span]uint8{{}: taskPending}}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := loopMemoChecker()
			initialize := func() {
				c.loopFlowContexts = []loopFlowContext{{}}
				c.breakFlowContexts = []breakFlowContext{{}}
				c.capturedWrites = []map[source.Span]source.Span{{}}
				c.capturedMemberWrites = []source.Span{{}}
				entry := c.snapshotNullableFlow()
				c.exceptionFlows = []*exceptionFlowContext{{entry: entry, flow: entry, finally: true}}
				c.arrowReturns = &arrowReturnInference{}
				c.taskLabelFlows = nil
			}
			initialize()
			statement := &ast.WhileStmt{}
			entry := c.snapshotNullableFlow()
			_, complete := c.memoizeLoopCheck(statement, entry)
			effect(c)
			complete()
			want, labels := c.loopEffects(), cloneTaskLabels(c.taskLabelFlows)
			c.restoreNullableFlow(entry)
			initialize()
			if reused, _ := c.memoizeLoopCheck(statement, c.snapshotNullableFlow()); !reused {
				t.Fatal("identical entry did not reuse loop transfer")
			}
			if !reflect.DeepEqual(want, c.loopEffects()) || !reflect.DeepEqual(labels, c.taskLabelFlows) {
				t.Fatalf("%s effects were not replayed", name)
			}
		})
	}
}

func TestLoopCheckMemoExceptionSummaryExcludesPreviousPaths(t *testing.T) {
	c := loopMemoChecker()
	entry := c.snapshotNullableFlow()
	maybe := c.snapshotNullableFlow()
	s := maybe.scopes[0]["p"]
	s.typeInfo = s.declaredType
	maybe.scopes[0]["p"] = s
	c.exceptionFlows = []*exceptionFlowContext{{entry: entry, flow: maybe}}
	statement := &ast.WhileStmt{}
	_, complete := c.memoizeLoopCheck(statement, entry)
	c.recordExceptionFlow()
	complete()
	c.exceptionFlows = []*exceptionFlowContext{{entry: entry, flow: entry}}
	if reused, _ := c.memoizeLoopCheck(statement, c.snapshotNullableFlow()); !reused {
		t.Fatal("write-only exception join blocked reuse")
	}
	if c.exceptionFlows[0].flow.scopes[0]["p"].typeInfo.Kind == Nullable {
		t.Fatal("an unrelated earlier path leaked into the loop summary")
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
