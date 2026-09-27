package sema

import (
	"strings"
	"testing"
)

func TestNullableShortCircuitConditions(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"and operand":             `return p !== null && p.value > 0;`,
		"or operand":              `return p === null || p.value > 0;`,
		"negated operand":         `return !(p === null) && p.value > 0;`,
		"and branch":              `if(p !== null && flag){return p.value > 0;}return false;`,
		"or else":                 `if(p === null || flag){return false;}else{return p.value > 0;}`,
		"negated branch":          `if(!(p === null || !flag)){return p.value > 0;}return false;`,
		"guard return":            `if(p === null || !flag){return false;}return p.value > 0;`,
		"merged paths":            `if((flag && p !== null) || (!flag && p !== null)){return p.value > 0;}return false;`,
		"while":                   `while(p !== null && flag){return p.value > 0;}return false;`,
		"for":                     `for(;!(p === null) && flag;){return p.value > 0;}return false;`,
		"member":                  `return box.item !== null && box.item.value > 0;`,
		"member branch":           `if(box.item === null || !flag){return false;}return box.item.value > 0;`,
		"call before check":       `if(clear(box) && box.item !== null){return box.item.value > 0;}return false;`,
		"loop false narrowing":    `while(p === null || !flag){return false;}return p.value > 0;`,
		"loop repeated narrowing": `while(p !== null && p.value > 0){p=null;}return false;`,
		"named boolean":           `const ok=Flag(flag);if(ok && !ok){return false;}return p !== null && p.value > 0;`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if diagnostics := checkSource(t, nullableConditionPrelude+`function f(p:Item|null,box:Box,flag:boolean):boolean{`+body+`}`); len(diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", diagnostics)
			}
		})
	}
}

const nullableConditionPrelude = `class Item{public value:int=1;}
type Flag=distinct boolean;
class Box{constructor(public item:Item|null){}}
function clear(box:Box):boolean{box.item=null;return true;}
`

func TestRejectsUnsafeNullableShortCircuitConditions(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"unguarded and":                  `return p === null && p.value > 0;`,
		"unguarded or":                   `return p !== null || p.value > 0;`,
		"partial branch":                 `if(p !== null || flag){return p.value > 0;}return false;`,
		"partial else":                   `if(p === null && flag){return false;}return p.value > 0;`,
		"discarded test":                 `const ok=p !== null && flag;return p.value > 0;`,
		"call invalidates branch":        `if(box.item !== null && clear(box)){return box.item.value > 0;}return false;`,
		"call invalidates operand":       `return box.item !== null && clear(box) && box.item.value > 0;`,
		"call invalidates else":          `if(box.item === null || clear(box)){return false;}return box.item.value > 0;`,
		"alternative invalidation":       `if((box.item !== null && flag) || (clear(box) && flag)){return box.item.value > 0;}return false;`,
		"conditional call afterflow":     `if(box.item === null){return false;}const ok=flag && clear(box);return box.item.value > 0;`,
		"loop false exit mutation":       `if(box.item === null){return false;}while(clear(box) && flag){return false;}return box.item.value > 0;`,
		"for false exit mutation":        `if(box.item === null){return false;}for(;clear(box) || flag;){return false;}return box.item.value > 0;`,
		"loop backedge":                  `while(flag && p.value > 0){p=null;}return false;`,
		"captured write":                 `const mutate=():boolean=>{p=null;return true;};if(p!==null && mutate()){return p.value > 0;}return false;`,
		"loop break not narrowed":        `while(p === null || flag){break;}return p.value > 0;`,
		"closure in conditional operand": `if(p===null){return false;}const ok=flag && ((():boolean=>{p=null;return true;})());return p.value > 0;`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			diagnostics := checkSource(t, nullableConditionPrelude+`function f(p:Item|null,box:Box,flag:boolean):boolean{`+body+`}`)
			if joined := strings.Join(diagnostics, "\n"); !strings.Contains(joined, "must be checked against null") {
				t.Fatalf("expected nullable diagnostic, got %v", diagnostics)
			}
		})
	}
}

func TestShortCircuitTaskConsumption(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"and may skip await":             `const ok=flag && await task;`,
		"or may skip await":              `const ok=flag || await task;`,
		"second await after conditional": `const ok=flag && await task;const again=await task;`,
		"or branch may skip await":       `if(flag || await task){return;}`,
		"false and skips await":          `const ok=false && await task;`,
		"true or skips await":            `const ok=true || await task;`,
	} {
		t.Run(name, func(t *testing.T) {
			diagnostics := checkSource(t, `function value():boolean{return true;}function f(flag:boolean):void{const task=go value();`+body+`}`)
			if joined := strings.Join(diagnostics, "\n"); !strings.Contains(joined, `Task "task"`) || !strings.Contains(joined, "consumed") {
				t.Fatalf("expected conditional Task consumption diagnostic, got %v", diagnostics)
			}
		})
	}
	diagnostics := checkSource(t, `function value():boolean{return true;}
	function f(flag:boolean):void{const task=go value();const ok=await task && flag;}
	function g():void{const task=go value();const ok=true && await task;}
	function h():void{const task=go value();const ok=false || await task;}
	function skipped():void{const task=go value();const ok=false && await task;detach task;}`)
	if len(diagnostics) != 0 {
		t.Fatalf("unconditional consumption rejected: %v", diagnostics)
	}
}
