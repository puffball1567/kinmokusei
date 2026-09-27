package sema

import (
	"strings"
	"testing"
)

const selectFlowPrelude = `class Item{public value:int=1;}
class Box{constructor(public item:Item|null){}}
function clear(box:Box):int{box.item=null;return 0;}
function channel(box:Box,input:GoChannel<int>):GoChannel<int>{box.item=null;return input;}
function work():int{return 1;}
`

func TestSelectOperandsInvalidateNullableFlow(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"default after send":  `select{case output<-clear(box){} default{const n=box.item.value;}}`,
		"earlier case body":   `select{case <-input{const n=box.item.value;} case output<-clear(box){} default{}}`,
		"later send operand":  `select{case output<-clear(box){} case output<-box.item.value{} default{}}`,
		"receive channel":     `select{case <-channel(box,input){} default{const n=box.item.value;}}`,
		"default before send": `select{default{const n=box.item.value;} case output<-clear(box){}}`,
		"receive target":      `select{case box.item.value=<-input{} case output<-clear(box){} default{}}`,
	} {
		t.Run(name, func(t *testing.T) {
			input := selectFlowPrelude + `function f(box:Box,input:GoChannel<int>,output:GoChannel<int>):void{if(box.item===null){return;}` + body + `}`
			if diagnostics := checkSource(t, input); !strings.Contains(strings.Join(diagnostics, "\n"), "must be checked against null") {
				t.Fatalf("expected nullable diagnostic: %v", diagnostics)
			}
		})
	}
}

func TestSelectOperandsConsumeTasksBeforeSelection(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"repeated send operand": `select{case output<-await task{} case output<-await task{} default{const n=await task;}}`,
		"default after send":    `select{case output<-await task{} default{const n=await task;}}`,
		"earlier receive body":  `select{case <-input{const n=await task;} case output<-await task{} default{const n=await task;}}`,
	} {
		t.Run(name, func(t *testing.T) {
			input := selectFlowPrelude + `function f(input:GoChannel<int>,output:GoChannel<int>):void{const task=go work();` + body + `}`
			if diagnostics := checkSource(t, input); !strings.Contains(strings.Join(diagnostics, "\n"), "already") {
				t.Fatalf("expected repeated Task consumption: %v", diagnostics)
			}
		})
	}
}

func TestSelectFlowPreservesSafePrograms(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"send awaits even on default":    `const task=go work();select{case output<-await task{} default{}}`,
		"receive awaits even on default": `const get=():GoChannel<int>=>{return input;};const task=go get();select{case <-await task{} default{}}`,
		"receive target remains lazy":    `if(box.item===null){return;}let values:int[]=[0];select{case values[clear(box)]=<-input{} default{const n=box.item.value;}}`,
		"narrow again inside body":       `select{case output<-clear(box){} default{if(box.item!==null){const n=box.item.value;}}}`,
		"case local shadowing":           `select{case const input=<-input{const n=input;} case const input=<-input{const n=input;} default{}}`,
	} {
		t.Run(name, func(t *testing.T) {
			input := selectFlowPrelude + `function f(box:Box,input:GoChannel<int>,output:GoChannel<int>):void{` + body + `}`
			if diagnostics := checkSource(t, input); len(diagnostics) != 0 {
				t.Fatalf("safe select rejected: %v", diagnostics)
			}
		})
	}
}
