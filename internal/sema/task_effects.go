package sema

import (
	"fmt"
	"sort"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func (c *Checker) checkTaskStart(expr *ast.TaskStartExpr) Type {
	c.usesTasks = true
	previous := c.taskLaunchCall
	c.taskLaunchCall = expr.Call
	result := c.checkExpression(expr.Call)
	c.taskLaunchCall = previous
	if expr.Call.Conversion {
		c.report(expr.Call.Span, "go expression requires a function or method call; type conversions are not calls")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if expr.Call.Builtin != ast.NotBuiltinCall {
		c.report(expr.Call.Span, "go expression does not support compiler built-ins; wrap the operation in a function")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if result.Kind == Invalid {
		return result
	}
	if result.Kind == MultiValue {
		c.report(expr.Call.Span, "go expression cannot start a raw multiple-result Go call; wrap it in a Result-returning Kinmokusei function")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	expr.ResultTask = result.Kind == Result
	expr.Void = result.Kind == Void || expr.ResultTask && result.Element != nil && result.Element.Kind == Void
	value := result
	if result.Kind == Result && result.Element != nil {
		value = *result.Element
	}
	c.prepareGoTypeForEmission(&value, expr.Span)
	expr.ValueType = typeRefFromType(value, expr.Span)
	return Type{Kind: Task, Name: "Task", Element: &result}
}

func (c *Checker) checkAwait(expr *ast.AwaitExpr) Type {
	c.usesTasks = true
	c.taskOperandDepth++
	task := c.singleValue(c.checkExpression(expr.Value), expr.Value.GetSpan())
	c.taskOperandDepth--
	if task.Kind != Task || task.Element == nil {
		if task.Kind != Invalid {
			c.report(expr.Value.GetSpan(), fmt.Sprintf("await requires Task<T>, got %s", task.String()))
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	c.consumeTask(expr.Value)
	result := *task.Element
	expr.ResultTask = result.Kind == Result
	expr.Void = result.Kind == Void || expr.ResultTask && result.Element != nil && result.Element.Kind == Void
	value := result
	if expr.ResultTask && result.Element != nil {
		value = *result.Element
	}
	c.prepareGoTypeForEmission(&value, expr.Span)
	expr.ValueType = typeRefFromType(value, expr.Span)
	return result
}

func (c *Checker) rejectTaskAPIType(value Type, span source.Span, context string) {
	if containsTaskType(value) {
		c.report(span, fmt.Sprintf("%s cannot contain Task; Task is a non-escaping local capability", context))
	}
}

func containsTaskType(value Type) bool {
	return containsTaskTypeSeen(value, map[string]bool{})
}

func containsTaskTypeSeen(value Type, visiting map[string]bool) bool {
	if value.Kind == Task {
		return true
	}
	if value.Kind == Struct {
		if visiting[value.Name] {
			return false
		}
		visiting[value.Name] = true
		defer delete(visiting, value.Name)
	}
	if value.Element != nil && containsTaskTypeSeen(*value.Element, visiting) {
		return true
	}
	if value.Key != nil && containsTaskTypeSeen(*value.Key, visiting) {
		return true
	}
	if value.Result != nil && containsTaskTypeSeen(*value.Result, visiting) {
		return true
	}
	for _, parameter := range value.Parameters {
		if containsTaskTypeSeen(parameter, visiting) {
			return true
		}
	}
	if value.Kind == Object || value.Kind == Struct {
		for _, field := range value.Fields {
			if containsTaskTypeSeen(field, visiting) {
				return true
			}
		}
	}
	return false
}

func (c *Checker) consumeTask(expression ast.Expression) {
	if _, direct := expression.(*ast.TaskStartExpr); direct {
		return
	}
	identifier, ok := expression.(*ast.IdentifierExpr)
	if !ok {
		c.report(expression.GetSpan(), "await and detach require a Task variable or a direct go expression")
		return
	}
	for index := len(c.scopes) - 1; index >= 0; index-- {
		symbol, exists := c.scopes[index][identifier.Name]
		if !exists {
			continue
		}
		for _, callableBase := range c.callableScopeBases {
			if index < callableBase {
				c.report(identifier.Span, fmt.Sprintf("Task %q cannot be captured by a closure", identifier.Name))
				return
			}
		}
		switch symbol.taskState {
		case taskPending:
			symbol.taskState = taskConsumed
		case taskConsumed:
			c.report(identifier.Span, fmt.Sprintf("Task %q has already been consumed", identifier.Name))
		case taskMaybeConsumed:
			c.report(identifier.Span, fmt.Sprintf("Task %q may already have been consumed on another control-flow path", identifier.Name))
		default:
			c.report(identifier.Span, fmt.Sprintf("value %q is not a consumable Task binding", identifier.Name))
		}
		c.scopes[index][identifier.Name] = symbol
		return
	}
}

func (c *Checker) reportUnconsumedTasks(scope map[string]valueSymbol) {
	names := make([]string, 0, len(scope))
	for name := range scope {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		symbol := scope[name]
		switch symbol.taskState {
		case taskPending:
			c.report(symbol.declarationSpan, fmt.Sprintf("Task %q must be consumed with await or detach before leaving its scope", name))
		case taskMaybeConsumed:
			c.report(symbol.declarationSpan, fmt.Sprintf("Task %q is not consumed on every control-flow path", name))
		}
	}
}

func (c *Checker) reportPendingTasksBeforeExit() {
	// Returning or propagating from a callback does not exit its enclosing
	// callable. Task capture is checked separately by consumeTask.
	base := 0
	if len(c.callableScopeBases) != 0 {
		base = c.callableScopeBases[len(c.callableScopeBases)-1]
	}
	for _, scope := range c.scopes[base:] {
		c.reportUnconsumedTasks(scope)
	}
}

// A return (including the error edge of ?) crosses every enclosing finally.
// Keep its Task state until those handlers have had a chance to consume it.
func (c *Checker) reportOrDeferTasksBeforeReturn() {
	if c.suppressFlowEffects != 0 {
		return
	}
	c.reportOrDeferTasksInFlow(c.snapshotNullableFlow())
}

func (c *Checker) reportOrDeferTasksInFlow(flow nullableFlowSnapshot) {
	if len(flow.scopes) > len(c.scopes) {
		flow.scopes = flow.scopes[:len(c.scopes)]
	}
	for index := len(c.exceptionFlows) - 1; index >= 0; index-- {
		context := c.exceptionFlows[index]
		if !context.finally {
			continue
		}
		// Locals introduced inside the try/catch cannot be reached by its
		// finally. Their own scope exit checks still run, but a return also
		// needs to diagnose them on the terminating edge.
		for _, scope := range flow.scopes[len(context.entry.scopes):] {
			c.reportUnconsumedTasks(scope)
		}
		context.returns = append(context.returns, flow)
		return
	}
	c.reportPendingTasksInFlow(flow)
}

func (c *Checker) reportPendingTasksInFlow(flow nullableFlowSnapshot) {
	base := 0
	if len(c.callableScopeBases) != 0 {
		base = c.callableScopeBases[len(c.callableScopeBases)-1]
	}
	for _, scope := range flow.scopes[base:] {
		c.reportUnconsumedTasks(scope)
	}
}

// Apply the Task effect of a checked finally to a saved early-return edge.
// Nullable narrowing is intentionally not copied: only Task ownership is
// relevant to the return obligation.
func applyFinallyTaskEffect(flow *nullableFlowSnapshot, before, after nullableFlowSnapshot) {
	for index, scope := range flow.scopes {
		if index >= len(before.scopes) || index >= len(after.scopes) {
			break
		}
		for name, symbol := range scope {
			prior, exists := before.scopes[index][name]
			if !exists || symbol.declarationSpan != prior.declarationSpan || prior.taskState == taskNotTracked {
				continue
			}
			next, exists := after.scopes[index][name]
			if exists && next.declarationSpan == prior.declarationSpan && next.taskState != prior.taskState {
				symbol.taskState = next.taskState
				scope[name] = symbol
			}
		}
	}
}

func (c *Checker) restoreTaskStates(flow nullableFlowSnapshot) {
	for index, scope := range c.scopes {
		if index >= len(flow.scopes) {
			break
		}
		for name, symbol := range scope {
			original, exists := flow.scopes[index][name]
			if exists && original.declarationSpan == symbol.declarationSpan && symbol.taskState != taskNotTracked {
				symbol.taskState = original.taskState
				scope[name] = symbol
			}
		}
	}
}
