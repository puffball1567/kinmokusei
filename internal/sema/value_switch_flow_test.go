package sema

import (
	"strings"
	"testing"
)

const valueSwitchFlowPrelude = `class Item{public value:int=1;}
class Box{constructor(public item:Item|null){}}
function clear(box:Box):int{box.item=null;return 1;}
function work():int{return 1;}
`

func TestValueSwitchCaseEffectsReachLaterPaths(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"later expression":          `switch(tag){case clear(box){} case box.item.value{}}`,
		"later body":                `switch(tag){case clear(box){} case 2{const n=box.item.value;}}`,
		"default first":             `switch(tag){default{const n=box.item.value;} case clear(box){}}`,
		"unmatched exit":            `switch(tag){case clear(box){return;}}const n=box.item.value;`,
		"fallthrough nullable body": `switch(tag){case 0{box.item=null;fallthrough;}case clear(box){const n=box.item.value;}}`,
	} {
		t.Run(name, func(t *testing.T) {
			diagnostics := checkSource(t, valueSwitchFlowPrelude+`function f(box:Box,tag:int):void{if(box.item===null){return;}`+body+`}`)
			if !strings.Contains(strings.Join(diagnostics, "\n"), "must be checked against null") {
				t.Fatalf("expected nullable diagnostic: %v", diagnostics)
			}
		})
	}
}

func TestValueSwitchRejectsTaskPathErrors(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"repeated expressions":       `switch(tag){case await task{}case await task{}default{const n=await task;}}`,
		"reuse in default":           `switch(tag){case await task{}default{const n=await task;}}`,
		"reuse after unmatched exit": `switch(tag){case await task{return;}}const n=await task;`,
		"grouped label skips await":  `switch(tag){case 0,await task{}default{const n=await task;}}`,
		"fallthrough skips await":    `switch(tag){case 0{fallthrough;}case await task{}default{const n=await task;}}`,
	} {
		t.Run(name, func(t *testing.T) {
			diagnostics := checkSource(t, valueSwitchFlowPrelude+`function f(tag:int):void{const task=go work();`+body+`}`)
			if !strings.Contains(strings.Join(diagnostics, "\n"), `Task "task"`) {
				t.Fatalf("expected Task ownership diagnostic: %v", diagnostics)
			}
		})
	}
}

func TestValueSwitchPreservesSafeCasePaths(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"await before default":                   `const task=go work();switch(tag){case await task{}default{}}`,
		"default first still follows tests":      `const task=go work();switch(tag){default{}case await task{}}`,
		"await before unmatched exit":            `const task=go work();switch(tag){case await task{}}`,
		"fallthrough does not repeat await":      `const task=go work();switch(tag){case 0{const n=await task;fallthrough;}case await task{}default{}}`,
		"matched body skips later effects":       `if(box.item===null){return;}switch(tag){case 0{const n=box.item.value;}case clear(box){}default{}}`,
		"body does not affect other expressions": `if(box.item===null){return;}switch(tag){case 0{box.item=null;}case box.item.value{}default{}}`,
		"case scope stays local":                 `switch(tag){case 0{const tag=1;const n=tag;}case tag{}}`,
	} {
		t.Run(name, func(t *testing.T) {
			if diagnostics := checkSource(t, valueSwitchFlowPrelude+`function f(box:Box,tag:int):void{`+body+`}`); len(diagnostics) != 0 {
				t.Fatalf("safe case flow rejected: %v", diagnostics)
			}
		})
	}
}
