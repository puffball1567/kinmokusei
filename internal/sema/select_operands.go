package sema

import "github.com/puffball1567/kinmokusei/internal/ast"

// Go evaluates every receive channel and every send's channel/value once,
// in source order, before choosing a case. Their effects reach all bodies,
// including default. Receive assignment targets are evaluated only after
// selection, so checkSelectReceive handles those separately in case scope.
func (c *Checker) checkSelectOperands(stmt *ast.SelectStmt) []Type {
	received := make([]Type, len(stmt.Cases))
	for index := range stmt.Cases {
		clause := &stmt.Cases[index]
		switch clause.Kind {
		case ast.SelectSend:
			send := &ast.ChannelSendStmt{Channel: clause.Channel, Value: clause.Value, Span: clause.Span}
			c.checkChannelSend(send)
			clause.Value = send.Value // Preserve contextual conversions, including class upcasts.
		case ast.SelectReceive:
			receive := &ast.UnaryExpr{Operator: "<-", Operand: clause.Channel, Span: clause.Channel.GetSpan()}
			received[index] = c.checkChannelReceiveAccess(receive, selectReceiveTargetCount(clause) == 2, selectReceiveDiscarded(clause))
		}
	}
	return received
}

func selectReceiveDiscarded(clause *ast.SelectCase) bool {
	count := selectReceiveTargetCount(clause)
	if count == 0 {
		return true
	}
	if count != 1 {
		return false
	}
	if clause.Declare {
		return clause.Bindings[0].Name == "_"
	}
	identifier, ok := clause.Targets[0].(*ast.IdentifierExpr)
	return ok && identifier.Name == "_"
}

func selectReceiveTargetCount(clause *ast.SelectCase) int {
	if clause.Declare {
		return len(clause.Bindings)
	}
	return len(clause.Targets)
}
