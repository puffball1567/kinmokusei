package sema

import (
	"strings"
	"testing"
)

const taskBoundaryPrelude = `import go errors from "errors";
import go strconv from "strconv";
function value():int{return 7;}
function load():Result<int>{return ok(2);}
function ensure():Result<void>{return ok();}
function raw():error{return errors.New("failed");}
`

func TestTaskCallableBoundaries(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"explicit arrow": `const callback=():int=>{return 1;};const n=callback();return await outer+n;`,
		"inferred arrow": `const callback=()=>{return 1;};const n=callback();return await outer+n;`,
		"nested arrows":  `const callback=():int=>{const inner=go value();const nested=():int=>{return 1;};return await inner+nested();};const n=callback();return await outer+n;`,
		"mutual arrows":  `const a=(n:int):int=>{if(n==0){return 1;}return b(n-1);};const b=(n:int):int=>{return a(n);};return await outer+a(2);`,
		"result arrow":   `const callback=():Result<int>=>{const n=load()?;return ok(n);};const [n,err]=callback();if(err!==nil){return await outer;}return await outer+n;`,
		"throwing arrow": `const callback=():void=>{throw errors.New("failed");};return await outer;`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			diagnostics := checkSource(t, taskBoundaryPrelude+`function f():int{const outer=go value();`+body+`}`)
			if len(diagnostics) != 0 {
				t.Fatalf("unrelated outer Task rejected: %v", diagnostics)
			}
		})
	}
}

func TestRejectsPendingTasksAtPropagation(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"native result":        `const n=load()?;`,
		"void result":          `ensure()?;`,
		"raw multiple results": `const n=strconv.Atoi("2")?;`,
		"raw error":            `raw()?;`,
		"discard binding":      `const _=load()?;`,
		"second task result":   `const inner=go load();const n=await inner?;`,
		"nested block":         `{const n=load()?;}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			diagnostics := checkSource(t, taskBoundaryPrelude+`function f():Result<int>{const pending=go value();`+body+`const joined=await pending;return ok(joined);}`)
			if joined := strings.Join(diagnostics, "\n"); !strings.Contains(joined, `Task "pending" must be consumed`) {
				t.Fatalf("expected early-exit Task diagnostic, got %v", diagnostics)
			}
		})
	}
}

func TestTaskBoundariesKeepOwnershipChecks(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"arrow return":            `const callback=():int=>{const inner=go value();return 1;};`,
		"arrow propagation":       `const callback=():Result<int>=>{const inner=go value();const n=load()?;const v=await inner;return ok(v+n);};`,
		"outer still pending":     `const outer=go value();const callback=():int=>{return 1;};`,
		"capture still forbidden": `const outer=go value();const callback=():int=>{return await outer;};const n=await outer;`,
	} {
		t.Run(name, func(t *testing.T) {
			diagnostics := checkSource(t, taskBoundaryPrelude+`function f():void{`+body+`}`)
			if joined := strings.Join(diagnostics, "\n"); !strings.Contains(joined, "Task") {
				t.Fatalf("expected Task ownership diagnostic, got %v", diagnostics)
			}
		})
	}
	if diagnostics := checkSource(t, taskBoundaryPrelude+`
function joined():Result<int>{const task=go value();const n=await task;const v=load()?;return ok(n+v);}
function detached():Result<int>{const task=go value();detach task;const v=load()?;return ok(v);}
function awaited():Result<int>{const task=go load();const v=await task?;return ok(v);}
function parallel():Result<int>{const first=go load();const second=go load();const [a,e]=await first;const [b,f]=await second;if(e!==nil){return fail(e);}if(f!==nil){return fail(f);}return ok(a+b);}`); len(diagnostics) != 0 {
		t.Fatalf("consumed Tasks rejected: %v", diagnostics)
	}
}
