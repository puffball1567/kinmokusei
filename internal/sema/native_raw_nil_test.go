package sema

import (
	"strings"
	"testing"
)

func TestRejectsRawNilInNativeNonNullReferences(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"class binding":       `class User{} const user:User=nil;`,
		"class return":        `class User{} function f():User{return nil;}`,
		"class field":         `class User{} class Holder{public user:User=nil;}`,
		"struct field":        `class User{} struct Holder{public user:User;} const h=Holder{user:nil};`,
		"collection element":  `class User{} const users:User[]=[nil];`,
		"argument":            `class User{} function f(user:User):void{} function main():void{f(nil);}`,
		"constructor":         `class User{} class Holder<T>{constructor(public value:T){}}const h=new Holder<User>(nil);`,
		"reassignment":        `class User{} function main():void{let user=new User();user=nil;}`,
		"alias":               `class User{} alias Person=User;const user:Person=nil;`,
		"native interface":    `interface Reader{function read():int;}const r:Reader=nil;`,
		"inherited interface": `interface Reader{function read():int;}interface Child extends Reader{}function f():Child{return nil;}`,
		"anonymous contract":  `const r:interface{read():int;}=nil;`,
		"function binding":    `const f:()=>int=nil;`,
		"function return":     `function f():()=>int{return nil;}`,
		"function alias":      `alias Callback=()=>int;const f:Callback=nil;`,
		"defined function":    `type Callback=distinct ()=>int;const f:Callback=nil;`,
		"class constraint":    `class User{}constraint Users=User;function f<T extends Users>():T{return nil;}`,
		"function constraint": `constraint Callback=()=>int;function f<T extends Callback>():T{return nil;}`,
		"function argument":   `function f(callback:()=>int):void{}function main():void{f(nil);}`,
		"for function":        `function main():void{for(let i:(v:int)=>int=nil;i===nil;){break;}}`,
		"for interface":       `function main():void{for(let i:interface{read():int;}=nil;i===nil;){break;}}`,
		"result":              `class User{} function f():Result<User>{return ok(nil);}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if diagnostics := strings.Join(checkSource(t, input), "\n"); !strings.Contains(diagnostics, "cannot use nil as") {
				t.Fatalf("expected native raw-nil diagnostic, got %q", diagnostics)
			}
		})
	}
}

func TestNativeAbsenceAndRawGoNilRemainDistinct(t *testing.T) {
	t.Parallel()
	input := `import go http from "net/http";
import go time from "time";
class User{}
interface Reader{function read():int;}
alias Callback=()=>int;
const user:User|null=null;
const reader:Reader|null=null;
const callback:Callback|null=null;
const pointer:*int=nil;
const failure:error=nil;
const values:int[]=nil;
const lookup:Map<string,int>=nil;
const channel:GoChannel<int>=nil;
const handler:http.HandlerFunc=nil;
alias GoHandler=http.HandlerFunc;
type CustomGoHandler=distinct http.HandlerFunc;
const aliasedHandler:GoHandler=nil;
const customHandler:CustomGoHandler=nil;
function schedule():void{const timer=time.AfterFunc(0,nil);}
function f(value:User|null):User{if(value===null){return new User();}return value;}
`
	if diagnostics := checkSource(t, input); len(diagnostics) != 0 {
		t.Fatalf("valid nullable/Go boundary cases: %v", diagnostics)
	}
}

func TestRejectsRawNilNativeConversions(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"function alias":      `alias Callback=()=>int;const f=Callback(nil);`,
		"defined function":    `type Callback=distinct ()=>int;const f=Callback(nil);`,
		"class constraint":    `class User{}constraint Users=User;function f<T extends Users>():T{return T(nil);}`,
		"function constraint": `constraint Callback=()=>int;function f<T extends Callback>():T{return T(nil);}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if diagnostics := strings.Join(checkSource(t, input), "\n"); !strings.Contains(diagnostics, "cannot convert nil to") {
				t.Fatalf("expected native raw-nil conversion diagnostic, got %q", diagnostics)
			}
		})
	}
}
