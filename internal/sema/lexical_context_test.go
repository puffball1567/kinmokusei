package sema

import (
	"reflect"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/source"
)

func TestUnreachableOuterCodeDoesNotSuppressArrowBodyChecks(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"aliased field write":   `const box=new Box(new Item());if(box.item!==null){const alias=box;alias.item=null;return box.item.value;}return 0;`,
		"call invalidation":     `const box=new Box(new Item());if(box.item!==null){clear(box);return box.item.value;}return 0;`,
		"nested captured write": `const box=new Box(new Item());if(box.item!==null){const mutate=():void=>{box.item=null;};mutate();return box.item.value;}return 0;`,
	} {
		for wrapperName, wrapper := range map[string]string{
			"single arrow":          `function outer():void{return;const callback=():int=>{BODY};}`,
			"inferred peers":        `function outer():void{return;const first=()=>second();const second=()=>{BODY};}`,
			"unreachable loop post": `function accept(callback:()=>int):void{}function outer():void{for(;true;accept(()=>{BODY})){break;}}`,
		} {
			t.Run(name+"/"+wrapperName, func(t *testing.T) {
				diagnostics := checkSource(t, nullableConditionPrelude+strings.ReplaceAll(wrapper, "BODY", body))
				if !strings.Contains(strings.Join(diagnostics, "\n"), "must be checked against null") {
					t.Fatalf("unsafe closure body accepted: %v", diagnostics)
				}
			})
		}
	}
}

func TestClosureLexicalRestorationAndDeferredCapturePublication(t *testing.T) {
	t.Parallel()
	declaration := source.Span{Path: "scope.km", Start: source.Position{Line: 1}}
	key := memberFlowKey{root: declaration, path: "item"}
	value := builtins["int"]
	maybe := Type{Kind: Nullable, Element: &value}
	c := &Checker{
		scopes:           []map[string]valueSymbol{{"value": {typeInfo: value, declaredType: maybe, declarationSpan: declaration}}},
		loopFlowContexts: []loopFlowContext{{}}, breakFlowContexts: []breakFlowContext{{}},
		suppressFlowEffects: 2,
		memberFlow:          map[memberFlowKey]memberFlowState{key: {nonNull: true}}, memberTypes: map[memberFlowKey]Type{key: maybe},
		callableScopeBases: []int{1}, capturedWrites: []map[source.Span]source.Span{{}},
		capturedMemberWrites: []source.Span{{}}, capturedMemberRoots: []map[source.Span]bool{{declaration: true}},
	}
	outerFlow := c.snapshotNullableFlow()
	context := c.enterClosureLexical()
	if c.suppressFlowEffects != 0 || len(c.loopFlowContexts) != 0 || len(c.breakFlowContexts) != 0 || len(c.memberFlow) != 0 || len(c.memberTypes) != 0 {
		t.Fatal("enclosing body flow leaked into closure")
	}
	if c.scopes[0]["value"].typeInfo.Kind != Nullable || !c.capturedMemberRoots[1][declaration] {
		t.Fatal("mutable capture narrowing or receiver roots not handled")
	}
	c.recordCapturedWrite(0, declaration, declaration)
	c.recordMemberWrite(declaration)
	c.declareLocal("inner", value, true, nil, declaration)
	if len(c.capturedWrites[0]) != 0 {
		t.Fatal("write was published before restoring enclosing reachability")
	}
	effects := c.leaveClosureLexical(context)
	if len(effects.writes) != 1 || effects.memberWrite != declaration || !reflect.DeepEqual(c.snapshotNullableFlow(), outerFlow) {
		t.Fatalf("closure summary/restoration mismatch: %#v", effects)
	}
	if c.suppressFlowEffects != 2 || len(c.loopFlowContexts) != 1 || len(c.breakFlowContexts) != 1 || !reflect.DeepEqual(c.memberTypes, map[memberFlowKey]Type{key: maybe}) {
		t.Fatal("enclosing flow context was not restored")
	}
	c.publishClosureEffects(effects, declaration)
	if len(c.capturedWrites[0]) != 0 || c.capturedMemberWrites[0].Start.Line != 0 {
		t.Fatal("unreachable closure creation published enclosing capture effects")
	}
}

func TestClosureCapturePublicationReachability(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"unreachable nested arrow": `const callback=():void=>{return;const skipped=():void=>{value=null;};};`,
		"unreachable peer group":   `const callback=():void=>{return;const first=()=>second();const second=():void=>{value=null;};};`,
		"unreachable post":         `const callback=():void=>{for(;true;accept(()=>{value=null;})){break;}};`,
	} {
		t.Run(name, func(t *testing.T) {
			input := nullableConditionPrelude + `function accept(f:()=>void):void{}function outer():int{let value:Item|null=new Item();` + body + `return value.value;}`
			if diagnostics := checkSource(t, input); len(diagnostics) != 0 {
				t.Fatalf("unreachable nested capture leaked: %v", diagnostics)
			}
		})
	}
	for name, body := range map[string]string{
		"nested arrow":      `const callback=():void=>{const nested=():void=>{value=null;};};`,
		"nested peer group": `const callback=():void=>{const first=()=>second();const second=():void=>{value=null;};};`,
	} {
		t.Run(name, func(t *testing.T) {
			diagnostics := checkSource(t, nullableConditionPrelude+`function outer():int{let value:Item|null=new Item();`+body+`return value.value;}`)
			if !strings.Contains(strings.Join(diagnostics, "\n"), "must be checked against null") {
				t.Fatalf("reachable nested capture was not propagated: %v", diagnostics)
			}
		})
	}
}

func TestUnreachableOuterCodeDoesNotSuppressArrowTaskExits(t *testing.T) {
	t.Parallel()
	input := `function value():int{return 1;}
function outer():void{return;const callback=(flag:boolean):void=>{const task=go value();if(flag){return;}detach task;};}`
	diagnostics := checkSource(t, input)
	if !strings.Contains(strings.Join(diagnostics, "\n"), `Task "task" must be consumed`) {
		t.Fatalf("unconsumed Task on closure return accepted: %v", diagnostics)
	}
}
