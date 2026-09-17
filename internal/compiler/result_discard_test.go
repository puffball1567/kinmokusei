package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResultDiscardMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module result-discard.test\n\ngo 1.23\n",
		"load.km": `import go errors from "errors";
export function load(failNow:boolean):Result<int>{if(failNow){return fail(errors.New("failed"));}return ok(7);}
export function notify(failNow:boolean):Result<void>{if(failNow){return fail(errors.New("failed"));}return ok();}`,
		"entry.km": `import {load,notify} from "./load";
import go strconv from "strconv";
export const Arrow = (failNow:boolean):int=>{const [value,err]=load(failNow);if(err!==nil){return -1;}return value;};
export function Discard(failNow:boolean):int{
 let calls=0;
 const operation=():Result<int>=>{calls++;return load(failNow);};
 const signal=():Result<void>=>{calls++;return notify(failNow);};
 const _=operation();let _=signal();_=operation();_=signal();
 const [_,_]=operation();const [_]=signal();
 const [value,err]=operation();_=err;
 for(const _=operation();calls<10;_=signal()){const _=operation();}
 const _:int=1;const _:uint64=18446744073709551615;
 const _:error=nil;
 for(const _:error=nil;false;){}
 for(const _:uint64=18446744073709551615;false;){}
 const _=():int=>1;const _=():int=>2;
 _=strconv.Atoi("invalid");const _=strconv.Atoi("3");
 return calls*100+value;
}
export function Propagate(failNow:boolean):Result<int>{
 const _=load(failNow)?;
 const _:int=load(failNow)?;
 const _=notify(failNow)?;
 return ok(42);
}
export function Panic():void{const f=():Result<int>=>{let zero=0;return ok(1/zero);};_=f();}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "resultdiscard")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	if strings.Contains(string(generated), "_ = _") {
		t.Fatalf("blank binding was read:\n%s", generated)
	}
	reference := `package reference
import "errors"
func load(reject bool)(int,error){if reject{return 0,errors.New("failed")};return 7,nil}
func notify(reject bool)error{if reject{return errors.New("failed")};return nil}
func Arrow(reject bool)int{value,err:=load(reject);if err!=nil{return -1};return value}
func Discard(reject bool)int{
 calls:=0;operation:=func()(int,error){calls++;return load(reject)};signal:=func()error{calls++;return notify(reject)}
 _,_=operation();_=signal();_,_=operation();_=signal();_,_=operation();_=signal();value,err:=operation();_=err
 for _,_=operation();calls<10;_=signal(){_,_=operation()};return calls*100+value
}
func Propagate(reject bool)(int,error){if _,err:=load(reject);err!=nil{return 0,err};if _,err:=load(reject);err!=nil{return 0,err};if err:=notify(reject);err!=nil{return 0,err};return 42,nil}
func Panic(){zero:=0;_=1/zero}
`
	comparison := `package resultdiscard_test
import("testing";"fmt";g "result-discard.test";r "result-discard.test/reference")
func capture(f func())(value any){defer func(){value=recover()}();f();return}
func TestArrow(t *testing.T){for _,reject:=range []bool{false,true}{if got,want:=g.Arrow(reject),r.Arrow(reject);got!=want{t.Fatalf("got %d want %d",got,want)}}}
func TestContracts(t *testing.T){for _,reject:=range []bool{false,true}{if got,want:=g.Discard(reject),r.Discard(reject);got!=want{t.Errorf("Discard(%v)=%d want %d",reject,got,want)};gv,ge:=g.Propagate(reject);rv,re:=r.Propagate(reject);if gv!=rv||fmt.Sprint(ge)!=fmt.Sprint(re){t.Errorf("Propagate(%v)=(%d,%v) want (%d,%v)",reject,gv,ge,rv,re)}};if got,want:=capture(g.Panic),capture(r.Panic);got!=want{t.Errorf("panic=%v want %v",got,want)}}
`
	runGeneratedGoDifferentialTest(t, root, "result-discard.test", generated, reference, comparison)
}
