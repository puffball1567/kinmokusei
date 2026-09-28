package sema

// A handler cannot use only the normal end state: a throw, return or runtime
// panic can interrupt a path before a later assignment restores a nullable
// value. Keep a bounded join of intermediate states, restricted to the scopes
// visible on entry. Nested callable bodies have independent collectors.
type exceptionFlowContext struct {
	entry   nullableFlowSnapshot
	flow    nullableFlowSnapshot
	finally bool
	returns []nullableFlowSnapshot
}

func (c *Checker) recordExceptionFlow() {
	if len(c.exceptionFlows) == 0 || c.suppressFlowEffects != 0 {
		return
	}
	current := c.snapshotNullableFlow()
	for _, context := range c.exceptionFlows {
		context.flow = c.mergeNullableFlow(context.entry, context.flow, current)
	}
}
