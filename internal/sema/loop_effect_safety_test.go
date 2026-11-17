package sema

import (
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/lexer"
	"github.com/puffball1567/kinmokusei/internal/parser"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func TestLoopEffectSummariesPreserveSafetyChecks(t *testing.T) {
	t.Parallel()
	open := strings.Repeat("while(b){p=q;", 8)
	close := strings.Repeat("}", 8)
	prelude := `class Item{public value:int=1;}function work():int{return 7;}`
	for name, test := range map[string]struct{ input, want string }{
		"intermediate catch":                  {`function f(b:boolean,q:Item,err:error):void{let p:Item|null=q;try{` + open + `p=null;if(b){throw err;}` + close + `p=q;}catch(_:error){const n=p.value;}}`, "must be checked against null"},
		"intermediate finally return":         {`function f(b:boolean,q:Item):void{let p:Item|null=q;try{` + open + `p=null;if(b){return;}` + close + `p=q;}finally{const n=p.value;}}`, "must be checked against null"},
		"guarded catch":                       {`function f(b:boolean,q:Item,err:error):void{let p:Item|null=q;try{` + open + `p=null;if(b){throw err;}` + close + `p=q;}catch(_:error){if(p!==null){const n=p.value;}}}`, ""},
		"finally consumes return task":        {`function f(b:boolean,q:Item):void{const task=go work();let p:Item|null=q;try{` + open + `p=null;if(b){return;}` + close + `}finally{const n=await task;}}`, ""},
		"return cannot skip task consumption": {`function f(b:boolean,q:Item):void{const task=go work();let p:Item|null=q;try{` + open + `p=null;if(b){return;}` + close + `}finally{}detach task;}`, `Task "task" must be consumed`},
		"captured write":                      {`function f(b:boolean,q:Item):void{let p:Item|null=q;const mutate=():void=>{` + open + `p=null;` + close + `};const n=p.value;}`, "must be checked against null"},
		"inferred conflicting return":         {`function f(b:boolean,q:Item):void{const run=()=>{let p:Item|null=q;` + open + `p=null;if(b){return 1;}` + close + `return "bad";};run();}`, "cannot use"},
		"constructor cannot assume iteration": {`class Holder{public value:Item;constructor(b:boolean,q:Item){let p:Item|null=q;` + open + `p=null;this.value=q;` + close + `}}`, "must be initialized on every constructor path"},
		"constructor initialized before loop": {`class Holder{public value:Item;constructor(b:boolean,q:Item){this.value=q;let p:Item|null=q;` + open + `p=null;` + close + `}}`, ""},
		"outer continue nullable exit":        {`function f(b:boolean,q:Item):void{let p:Item|null=q;outer:while(b){` + open + `p=null;if(b){continue outer;}` + close + `}const n=p.value;}`, "must be checked against null"},
	} {
		t.Run(name, func(t *testing.T) {
			messages := checkSource(t, prelude+test.input)
			if test.want == "" {
				if len(messages) != 0 {
					t.Fatalf("safe loop rejected: %v", messages)
				}
			} else if !strings.Contains(strings.Join(messages, "\n"), test.want) {
				t.Fatalf("want %q, got %v", test.want, messages)
			}
		})
	}
}

func TestLoopMemoReturnObligationsAreImmutable(t *testing.T) {
	c := loopMemoChecker()
	c.scopes[0]["task"] = valueSymbol{taskState: taskPending, declarationSpan: source.Span{Path: "task.km", Start: source.Position{Line: 1}}}
	entry := c.snapshotNullableFlow()
	c.exceptionFlows = []*exceptionFlowContext{{entry: entry, flow: entry, finally: true}}
	statement := &ast.WhileStmt{}
	_, complete := c.memoizeLoopCheck(statement, entry)
	c.exceptionFlows[0].returns = append(c.exceptionFlows[0].returns, c.snapshotNullableFlow())
	complete()
	after := c.snapshotNullableFlow()
	symbol := after.scopes[0]["task"]
	symbol.taskState = taskConsumed
	after.scopes[0]["task"] = symbol
	for i := 0; i < 2; i++ {
		applyFinallyTaskEffect(&c.exceptionFlows[0].returns[0], entry, after)
		c.exceptionFlows = []*exceptionFlowContext{{entry: entry, flow: entry, finally: true}}
		if reused, _ := c.memoizeLoopCheck(statement, c.snapshotNullableFlow()); !reused {
			t.Fatal("identical Task entry was not reused")
		}
		if c.exceptionFlows[0].returns[0].scopes[0]["task"].taskState != taskPending {
			t.Fatal("finally mutated the saved pending return obligation")
		}
	}
}

func TestLoopMemoKeepsFirstCaptureCauseAtDestination(t *testing.T) {
	c := loopMemoChecker()
	declaration := source.Span{Path: "capture.km"}
	first := source.Span{Path: "capture.km", Start: source.Position{Line: 1}}
	inside := source.Span{Path: "capture.km", Start: source.Position{Line: 2}}
	later := source.Span{Path: "capture.km", Start: source.Position{Line: 3}}
	c.capturedWrites = []map[source.Span]source.Span{{declaration: first}}
	c.capturedMemberWrites = []source.Span{first}
	statement := &ast.WhileStmt{}
	_, complete := c.memoizeLoopCheck(statement, c.snapshotNullableFlow())
	c.capturedWrites[0][declaration] = inside
	c.capturedMemberWrites[0] = inside
	complete()
	if c.capturedWrites[0][declaration] != first || c.capturedMemberWrites[0] != first {
		t.Fatal("loop replaced the earlier capture cause")
	}
	c.capturedWrites = []map[source.Span]source.Span{{declaration: later}}
	c.capturedMemberWrites = []source.Span{later}
	if reused, _ := c.memoizeLoopCheck(statement, c.snapshotNullableFlow()); !reused {
		t.Fatal("write-only capture collector blocked reuse")
	}
	if c.capturedWrites[0][declaration] != later || c.capturedMemberWrites[0] != later {
		t.Fatal("a previous destination's capture cause leaked into replay")
	}
}

func TestLoopRechecksReplaceCatchMatchingMetadata(t *testing.T) {
	tokens, diagnostics := lexer.Lex("catch.km", `class Item{}class Fault extends Exception{constructor(){super("fault");}}function f(b:boolean,q:Item):void{let p:Item|null=q;while(b){p=q;try{p=null;}catch(_:Fault){}}}`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	program, diagnostics := parser.Parse(tokens)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	fn := program.Declarations[2].(*ast.FunctionDecl)
	clause := fn.Body.Statements[1].(*ast.WhileStmt).Body.Statements[1].(*ast.TryStmt).Catches[0]
	if diagnostics := Check(program); len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	if len(clause.MatchingClasses) != 1 || clause.MatchingClasses[0] != "Fault" {
		t.Fatalf("matching classes accumulated across passes: %v", clause.MatchingClasses)
	}
}

func TestLoopMemoTracksLazyInferenceChanges(t *testing.T) {
	c := loopMemoChecker()
	arrow := &localArrowInference{}
	symbol := c.scopes[0]["p"]
	symbol.localArrowInference = arrow
	c.scopes[0]["p"] = symbol
	statement := &ast.WhileStmt{}
	_, complete := c.memoizeLoopCheck(statement, c.snapshotNullableFlow())
	complete()
	arrow.checked = true
	if reused, _ := c.memoizeLoopCheck(statement, c.snapshotNullableFlow()); reused {
		t.Fatal("aliased inference status changed without invalidating the entry")
	}
}
