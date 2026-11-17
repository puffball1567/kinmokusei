package sema

import (
	"maps"
	"reflect"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/diagnostic"
	"github.com/puffball1567/kinmokusei/internal/source"
)

// Only the latest entry is retained: the syntax node's semantic annotations
// still describe that check. Reusing older entries would require restoring all
// flow-sensitive annotations as well as the exit state.
type loopCheckMemo struct {
	entry, exit               nullableFlowSnapshot
	entryMembers, exitMembers map[memberFlowKey]Type
	context                   loopMemoContext
	effects                   loopCheckEffects
	labels                    map[source.Span]*taskLabelFlow
	diagnostics               []diagnostic.Diagnostic
}

func (c *Checker) memoizeLoopCheck(statement ast.Statement, entry nullableFlowSnapshot) (bool, func()) {
	memo, exists := c.loopCheckMemos[statement]
	delete(c.loopCheckMemos, statement)
	context := c.loopMemoContext()
	if exists && reflect.DeepEqual(memo.entry, entry) && reflect.DeepEqual(memo.entryMembers, c.memberTypes) && memo.context.equal(context) {
		c.restoreNullableFlow(memo.exit)
		c.memberTypes = maps.Clone(memo.exitMembers)
		c.replayLoopEffects(memo.effects)
		c.taskLabelFlows = cloneTaskLabels(memo.labels)
		c.diagnostics = append(c.diagnostics, memo.diagnostics...)
		c.loopCheckMemos[statement] = memo
		return true, func() {}
	}
	members := maps.Clone(c.memberTypes)
	previous := c.collectLoopEffects(entry)
	diagnosticStart := len(c.diagnostics)
	return false, func() {
		effects := c.finishLoopEffects(previous)
		if c.loopCheckMemos == nil {
			c.loopCheckMemos = map[ast.Statement]loopCheckMemo{}
		}
		c.loopCheckMemos[statement] = loopCheckMemo{
			entry: entry, exit: c.snapshotNullableFlow(), entryMembers: members, exitMembers: maps.Clone(c.memberTypes),
			context: context, effects: effects, labels: cloneTaskLabels(c.taskLabelFlows),
			diagnostics: append([]diagnostic.Diagnostic(nil), c.diagnostics[diagnosticStart:]...),
		}
	}
}
