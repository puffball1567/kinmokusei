package sema

// callableControlState contains the return and control-transfer context saved
// across callable boundaries. Lexical scopes, receiver access, closure capture
// tracking, and nullable flow have separate lifetimes and are not reset here.
type callableControlState struct {
	result         Type
	loopDepth      int
	breakableDepth int
	exceptionDepth int
	catchTargets   []int
}

// enterCallableControl starts a new control-transfer boundary and returns the
// enclosing state for restoration at the end of the body check. Keep the result
// until the caller resolves its annotation: expression-bodied arrows may infer
// their result instead. A nil catch stack also prevents nested appends from
// modifying an enclosing callable's backing array.
func (c *Checker) enterCallableControl() callableControlState {
	previous := c.callableControlState
	c.callableControlState = callableControlState{result: previous.result}
	return previous
}
