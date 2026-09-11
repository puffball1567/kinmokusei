package sema

import (
	"fmt"
	"sort"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

// Initializers have class type parameters and module lexical bindings, but no
// receiver or constructor parameters. In particular, callbacks cannot capture
// a partially initialized receiver through this context.
func (c *Checker) checkClassFieldInitializers(decl *ast.ClassDecl) {
	previousInitializer, previousConstructor := c.inFieldInitializer, c.inConstructor
	previousFlow, previousResult := c.memberFlow, c.result
	c.inFieldInitializer, c.inConstructor = true, false
	c.memberFlow, c.result = map[memberFlowKey]memberFlowState{}, builtins["void"]
	c.pushScope()
	defer func() {
		c.popScope()
		c.inFieldInitializer, c.inConstructor = previousInitializer, previousConstructor
		c.memberFlow, c.result = previousFlow, previousResult
	}()
	for i := range decl.Fields {
		field := &decl.Fields[i]
		if field.Initializer == nil {
			continue
		}
		expected := c.resolveType(field.Type)
		actual := c.checkExpressionExpectedSlot(&field.Initializer, expected)
		c.requireAssignable(expected, actual, field.Initializer.GetSpan())
	}
}

func (c *Checker) checkClassFieldInitialization(decl *ast.ClassDecl) {
	class := c.classes[decl.Name]
	if class == nil {
		return
	}
	required := map[string]source.Span{}
	for _, field := range decl.Fields {
		symbol, exists := class.fields[field.Name]
		if !exists || symbol.typeInfo.Kind == Invalid || symbol.typeInfo.Kind == Nullable || !isNullableBaseType(symbol.typeInfo) {
			continue
		}
		required[field.Name] = field.NameSpan
	}
	if len(required) == 0 {
		return
	}
	initialized := map[string]bool{}
	for _, field := range decl.Fields {
		if field.Initializer != nil {
			initialized[field.Name] = true
		}
	}
	if decl.Constructor != nil {
		flow := constructorInitializationBlock(decl.Constructor.Body, initialized, required)
		initialized = flow.continuing
		if initialized == nil {
			initialized = map[string]bool{}
		}
	}
	names := make([]string, 0, len(required))
	for name := range required {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if initialized[name] {
			continue
		}
		fieldType := class.fields[name].typeInfo
		c.report(required[name], fmt.Sprintf("non-null field %q of type %s must be initialized on every constructor path; assign this.%s or declare it as %s | null", name, fieldType.String(), name, fieldType.String()))
	}
}

type constructorInitializationFlow struct {
	continuing   map[string]bool
	breaks       []map[string]bool
	continues    []map[string]bool
	fallthroughs []map[string]bool
}

// constructorInitializationAnalyzer reads checked AST metadata and tracks only
// required fields. It neither owns a Checker nor changes symbol/type state;
// branch-local initialization maps and range proofs are explicit inputs/results.
type constructorInitializationAnalyzer struct {
	required map[string]source.Span
}

func constructorInitializationBlock(block *ast.BlockStmt, initial map[string]bool, required map[string]source.Span) constructorInitializationFlow {
	analyzer := constructorInitializationAnalyzer{required: required}
	return analyzer.block(block, initial)
}

func (a constructorInitializationAnalyzer) block(block *ast.BlockStmt, initial map[string]bool) constructorInitializationFlow {
	return a.blockWithRangeProof(block, initial, nil)
}

// blockWithRangeProof carries a condition-derived
// non-empty proof through local declarations with side-effect-free initializers.
// Other intervening statements discard it: they could reassign the collection
// or mutate a map before the range expression is evaluated. Proofs use resolved
// declaration identity, so a shadowing declaration cannot inherit the fact.
func (a constructorInitializationAnalyzer) blockWithRangeProof(block *ast.BlockStmt, initial map[string]bool, nonEmpty constructorRangeProofs) constructorInitializationFlow {
	state := cloneFieldInitialization(initial)
	if block == nil {
		return constructorInitializationFlow{continuing: state}
	}
	var breaks []map[string]bool
	var continues []map[string]bool
	var fallthroughs []map[string]bool
	pendingNonEmpty := nonEmpty
	for _, statement := range block.Statements {
		if state == nil {
			break
		}
		var flow constructorInitializationFlow
		if len(pendingNonEmpty) != 0 {
			flow = a.statementWithRangeProof(statement, state, pendingNonEmpty)
		} else {
			flow = a.statement(statement, state)
		}
		if declaration, ok := statement.(*ast.VariableDecl); !ok || !constructorRangeGuardStable(declaration.Value) {
			pendingNonEmpty = constructorFollowingRangeProof(statement)
		}
		breaks = append(breaks, flow.breaks...)
		continues = append(continues, flow.continues...)
		fallthroughs = append(fallthroughs, flow.fallthroughs...)
		state = flow.continuing
	}
	return constructorInitializationFlow{continuing: state, breaks: breaks, continues: continues, fallthroughs: fallthroughs}
}

func constructorFollowingRangeProof(statement ast.Statement) constructorRangeProofs {
	guard, ok := statement.(*ast.IfStmt)
	if !ok || guard.Else != nil || !statementDefinitelyStopsBlock(guard.Then) {
		return nil
	}
	_, whenFalse := constructorNonEmptyRangeGuard(guard.Condition)
	return whenFalse
}

func (a constructorInitializationAnalyzer) statementWithRangeProof(statement ast.Statement, initial map[string]bool, nonEmpty constructorRangeProofs) constructorInitializationFlow {
	switch statement := statement.(type) {
	case *ast.LabeledStmt:
		return a.statementWithRangeProof(statement.Statement, initial, nonEmpty)
	case *ast.BlockStmt:
		return a.blockWithRangeProof(statement, initial, nonEmpty)
	case *ast.IfStmt:
		// A side-effect-free nested condition cannot invalidate an outer
		// collection-length fact. Carry it into both branches and combine any
		// additional length facts established by the nested condition itself.
		if constructorRangeGuardStable(statement.Condition) {
			thenAdditional, elseAdditional := constructorNonEmptyRangeGuard(statement.Condition)
			thenFlow := a.blockWithRangeProof(statement.Then, initial, unionConstructorRangeProofs(nonEmpty, thenAdditional))
			elseFlow := constructorInitializationFlow{continuing: cloneFieldInitialization(initial)}
			if statement.Else != nil {
				elseFlow = a.statementWithRangeProof(statement.Else, initial, unionConstructorRangeProofs(nonEmpty, elseAdditional))
			}
			return constructorInitializationFlow{
				continuing:   intersectCompletingFieldInitialization(thenFlow.continuing, elseFlow.continuing),
				breaks:       append(thenFlow.breaks, elseFlow.breaks...),
				continues:    append(thenFlow.continues, elseFlow.continues...),
				fallthroughs: append(thenFlow.fallthroughs, elseFlow.fallthroughs...),
			}
		}
	case *ast.ForRangeStmt:
		if statement.Kind == ast.CollectionRange && nonEmpty.contains(constructorRangeSourceDeclaration(statement.Source)) {
			return a.loop(statement.Body, initial, true, true)
		}
	}
	return a.statement(statement, initial)
}

func (a constructorInitializationAnalyzer) statement(statement ast.Statement, initial map[string]bool) constructorInitializationFlow {
	switch statement := statement.(type) {
	case *ast.LabeledStmt:
		return a.statement(statement.Statement, initial)
	case *ast.AssignmentStmt:
		state := cloneFieldInitialization(initial)
		if statement.Operator != "" && statement.Operator != "=" {
			return constructorInitializationFlow{continuing: state}
		}
		member, ok := statement.Target.(*ast.MemberExpr)
		if !ok {
			return constructorInitializationFlow{continuing: state}
		}
		receiver, ok := member.Object.(*ast.IdentifierExpr)
		if ok && receiver.Name == "this" {
			if _, tracked := a.required[member.Name]; tracked {
				state[member.Name] = true
			}
		}
		return constructorInitializationFlow{continuing: state}
	case *ast.BlockStmt:
		return a.block(statement, initial)
	case *ast.IfStmt:
		thenNonEmpty, elseNonEmpty := constructorNonEmptyRangeGuard(statement.Condition)
		thenFlow := a.blockWithRangeProof(statement.Then, initial, thenNonEmpty)
		elseFlow := constructorInitializationFlow{continuing: cloneFieldInitialization(initial)}
		if statement.Else != nil {
			elseFlow = a.statementWithRangeProof(statement.Else, initial, elseNonEmpty)
		}
		return constructorInitializationFlow{
			continuing:   intersectCompletingFieldInitialization(thenFlow.continuing, elseFlow.continuing),
			breaks:       append(thenFlow.breaks, elseFlow.breaks...),
			continues:    append(thenFlow.continues, elseFlow.continues...),
			fallthroughs: append(thenFlow.fallthroughs, elseFlow.fallthroughs...),
		}
	case *ast.ValueSwitchStmt:
		states := make([]map[string]bool, 0, len(statement.Cases)+1)
		var continues []map[string]bool
		var incomingFallthrough map[string]bool
		caseNonEmpty := constructorNonEmptyRangeSwitch(statement)
		hasDefault := false
		for index := range statement.Cases {
			clause := &statement.Cases[index]
			hasDefault = hasDefault || clause.Default
			caseInitial := initial
			nonEmpty := caseNonEmpty[index]
			if incomingFallthrough != nil {
				caseInitial = intersectFieldInitialization(initial, incomingFallthrough)
				// A fallthrough enters this body without satisfying its case
				// expressions, so its length fact does not apply.
				nonEmpty = nil
			}
			flow := a.blockWithRangeProof(clause.Body, caseInitial, nonEmpty)
			states = append(states, flow.breaks...)
			if clause.FallsThrough {
				incomingFallthrough = intersectFieldInitializationStates(flow.fallthroughs)
			} else {
				incomingFallthrough = nil
				if flow.continuing != nil {
					states = append(states, flow.continuing)
				}
			}
			continues = append(continues, flow.continues...)
		}
		if !hasDefault {
			states = append(states, cloneFieldInitialization(initial))
		}
		return constructorInitializationFlow{continuing: intersectFieldInitializationStates(states), continues: continues}
	case *ast.TypeSwitchStmt:
		states := make([]map[string]bool, 0, len(statement.Cases)+1)
		var continues []map[string]bool
		hasDefault := false
		for index := range statement.Cases {
			clause := &statement.Cases[index]
			hasDefault = hasDefault || clause.Default
			flow := a.block(clause.Body, initial)
			states = appendConstructorCompletingStates(states, flow)
			continues = append(continues, flow.continues...)
		}
		if !hasDefault {
			states = append(states, cloneFieldInitialization(initial))
		}
		return constructorInitializationFlow{continuing: intersectFieldInitializationStates(states), continues: continues}
	case *ast.SelectStmt:
		states := make([]map[string]bool, 0, len(statement.Cases))
		var continues []map[string]bool
		for index := range statement.Cases {
			flow := a.block(statement.Cases[index].Body, initial)
			states = appendConstructorCompletingStates(states, flow)
			continues = append(continues, flow.continues...)
		}
		return constructorInitializationFlow{continuing: intersectFieldInitializationStates(states), continues: continues}
	case *ast.WhileStmt:
		return a.loop(statement.Body, initial, statement.GuaranteedEntry || expressionAlwaysTrue(statement.Condition), false)
	case *ast.ForStmt:
		state := cloneFieldInitialization(initial)
		if statement.Initializer != nil {
			initializer := a.statement(statement.Initializer, state)
			state = initializer.continuing
			if state == nil {
				return initializer
			}
		}
		guaranteed := statement.Condition == nil || statement.GuaranteedEntry || expressionAlwaysTrue(statement.Condition)
		return a.loop(statement.Body, state, guaranteed, false)
	case *ast.ForRangeStmt:
		return a.loop(statement.Body, initial, statement.GuaranteedNonEmpty || rangeExpressionGuaranteedNonEmpty(statement.Source), true)
	case *ast.BranchStmt:
		if statement.Kind == ast.BreakBranch {
			return constructorInitializationFlow{breaks: []map[string]bool{cloneFieldInitialization(initial)}}
		}
		if statement.Kind == ast.ContinueBranch {
			return constructorInitializationFlow{continues: []map[string]bool{cloneFieldInitialization(initial)}}
		}
		if statement.Kind == ast.FallthroughBranch {
			return constructorInitializationFlow{fallthroughs: []map[string]bool{cloneFieldInitialization(initial)}}
		}
		return constructorInitializationFlow{}
	case *ast.ReturnStmt, *ast.ThrowStmt:
		return constructorInitializationFlow{}
	case *ast.TryStmt:
		body := a.block(statement.Body, initial)
		states := []map[string]bool{}
		if body.continuing != nil {
			states = append(states, body.continuing)
		}
		for _, clause := range statement.Catches {
			caught := a.block(clause.Body, initial)
			if caught.continuing != nil {
				states = append(states, caught.continuing)
			}
		}
		continuing := intersectFieldInitializationStates(states)
		if statement.FinallyBody != nil && continuing != nil {
			return a.block(statement.FinallyBody, continuing)
		}
		return constructorInitializationFlow{continuing: continuing}
	default:
		// Loops and other statements do not establish initialization. In
		// particular, a loop body may execute zero times.
		return constructorInitializationFlow{continuing: cloneFieldInitialization(initial)}
	}
}

func (a constructorInitializationAnalyzer) loop(body *ast.BlockStmt, initial map[string]bool, guaranteed, naturalExit bool) constructorInitializationFlow {
	if !guaranteed {
		return constructorInitializationFlow{continuing: cloneFieldInitialization(initial)}
	}
	flow := a.block(body, initial)
	exits := append([]map[string]bool(nil), flow.breaks...)
	if naturalExit {
		if flow.continuing != nil {
			exits = append(exits, flow.continuing)
		}
		exits = append(exits, flow.continues...)
	}
	return constructorInitializationFlow{continuing: intersectFieldInitializationStates(exits)}
}

func appendConstructorCompletingStates(states []map[string]bool, flow constructorInitializationFlow) []map[string]bool {
	if flow.continuing != nil {
		states = append(states, flow.continuing)
	}
	return append(states, flow.breaks...)
}

func intersectCompletingFieldInitialization(states ...map[string]bool) map[string]bool {
	continuing := states[:0]
	for _, state := range states {
		if state != nil {
			continuing = append(continuing, state)
		}
	}
	return intersectFieldInitializationStates(continuing)
}

func intersectFieldInitializationStates(states []map[string]bool) map[string]bool {
	if len(states) == 0 {
		return nil
	}
	result := cloneFieldInitialization(states[0])
	for _, state := range states[1:] {
		result = intersectFieldInitialization(result, state)
	}
	return result
}

func cloneFieldInitialization(state map[string]bool) map[string]bool {
	cloned := make(map[string]bool, len(state))
	for name, initialized := range state {
		if initialized {
			cloned[name] = true
		}
	}
	return cloned
}

func intersectFieldInitialization(left, right map[string]bool) map[string]bool {
	intersection := map[string]bool{}
	for name, initialized := range left {
		if initialized && right[name] {
			intersection[name] = true
		}
	}
	return intersection
}
