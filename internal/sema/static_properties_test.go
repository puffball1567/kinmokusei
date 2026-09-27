package sema

import (
	"strings"
	"testing"
)

func TestStaticPropertyContracts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"pair", `let raw=0;class C{public static get x():int{return raw;}public static set x(v:int){raw=v;}}function f():int{C.x=1;C.x+=2;C.x++;return C.x;}`, ""},
		{"inherited", `class B{public static get x():int{return 1;}protected static set x(v:int){}}class C extends B{public static function f():void{C.x++;B.x++;}}function f():int{return C.x;}`, ""},
		{"generic independent", `class B<T>{public static get x():int{return 1;}public static set x(v:int){}public function id(v:T):T{return v;}}class C<T> extends B<T>{}function f():int{C.x++;return B.x;}`, ""},
		{"generic getter signature", `class C<T>{public static get x():T{throw new Exception("bad");}}`, "unknown type"},
		{"generic setter signature", `class C<T>{public static set x(v:T){}}`, "unknown type"},
		{"generic body", `class C<T>{public static get x():int{let v:T=1;return 1;}}`, "unknown type"},
		{"generic nested body", `class C<T>{public static get x():int{const f=():T=>{throw new Exception("bad");};return 1;}}`, "unknown type"},
		{"mixed pair", `class C{public static get x():int{return 1;}public set x(v:int){}}`, "both be static"},
		{"reverse mixed pair", `class C{public get x():int{return 1;}public static set x(v:int){}}`, "both be static"},
		{"instance read", `class C{public static get x():int{return 1;}}function f(c:C):int{return c.x;}`, "through a class name"},
		{"instance write", `class C{public static set x(v:int){}}function f(c:C):void{c.x=1;}`, "through a class name"},
		{"class read instance", `class C{public get x():int{return 1;}}function f():int{return C.x;}`, "requires an instance"},
		{"class write instance", `class C{public set x(v:int){}}function f():void{C.x=1;}`, "requires an instance"},
		{"this", `class C{public n:int=0;public static get x():int{return this.n;}}`, "undefined name"},
		{"super", `class B{public static get x():int{return 1;}}class C extends B{public static get y():int{return super.x;}}`, "super cannot"},
		{"readonly", `class C{public static get x():int{return 1;}}function f():void{C.x=1;}`, "has no setter"},
		{"writeonly", `class C{public static set x(v:int){}}function f():int{return C.x;}`, "has no getter"},
		{"writeonly update", `class C{public static set x(v:int){}}function f():void{C.x++;}`, "accessible getter"},
		{"private read", `class C{private static get x():int{return 1;}}function f():int{return C.x;}`, "private"},
		{"private setter", `class C{public static get x():int{return 1;}private static set x(v:int){}}function f():void{C.x++;}`, "private"},
		{"protected setter", `class C{protected static set x(v:int){}}function f():void{C.x=1;}`, "protected"},
		{"private inherited", `class B{private static get x():int{return 1;}}class C extends B{public static function f():int{return C.x;}}`, "private"},
		{"abstract", `abstract class C{public static abstract get x():int;}`, "cannot be static"},
		{"virtual", `class C{public static virtual get x():int{return 1;}}`, "cannot be virtual"},
		{"override", `class B{public static get x():int{return 1;}}class C extends B{public static override get x():int{return 2;}}`, "cannot be overridden"},
		{"hide inherited", `class B{public static get x():int{return 1;}}class C extends B{public static get x():int{return 2;}}`, "replaces inherited"},
		{"add inherited half", `class B{public static get x():int{return 1;}}class C extends B{public static set x(v:int){}}`, "cannot be redeclared"},
		{"interface", `interface I{get x():int;}class C implements I{public static get x():int{return 1;}}`, "cannot be static"},
		{"Go collision", `class C{public static get x():int{return 1;}}function CGetX():int{return 2;}`, "collides"},
		{"method collision", `class C{public static get x():int{return 1;}public static function getX():int{return 2;}}`, "collides"},
		{"separate Go namespaces", `class C{public static get x():int{return 1;}public function getX():int{return 2;}}`, ""},
		{"static function instance property", `class C{public get x():int{return 1;}public static function getX():int{return 2;}}`, ""},
		{"address", `class C{public static get x():int{return 1;}}function f():void{const p=&C.x;}`, "address"},
		{"null getter", `class V{public n:int=1;}class C{public static get x():V|null{return null;}}function f():int{if(C.x!==null){return C.x.n;}return 0;}`, "nullable"},
		{"Result", `class C{public static get x():Result<int>{return ok(1);}}`, "properties"},
		{"initialization cycle", `const value:int=C.x;class C{public static get x():int{return value;}}`, "initialization cycle"},
		{"inherited cycle", `const value:int=D.x;class C{public static get x():int{return value;}}class D extends C{}`, "initialization cycle"},
		{"helper cycle", `const value:int=C.x;class C{public static get x():int{return C.read();}public static function read():int{return value;}}`, "initialization cycle"},
		{"closure cycle", `const value:int=C.x();class C{public static get x():()=>int{return ():int=>value;}}`, "initialization cycle"},
		{"compound getter cycle", `const value:int=initialize();function initialize():int{C.x++;return 1;}class C{public static get x():int{return value;}public static set x(v:int){}}`, "initialization cycle"},
		{"setter cycle", `let value:int=initialize();function initialize():int{C.x=1;return 1;}class C{public static set x(v:int){value=v;}}`, "initialization cycle"},
		{"assignment does not read getter", `const value:int=initialize();function initialize():int{C.x=1;return 1;}class C{public static get x():int{return value;}public static set x(v:int){}}`, ""},
		{"recursive accessors not initialization cycle", `class C{public static get x():int{return C.y;}public static get y():int{return C.x;}}`, ""},
		{"getter local shadow", `class C{public static get x():int{return 1;}}function f():int{const CGetX=():int=>99;return C.x;}`, "shadows generated static member"},
		{"setter local shadow", `class C{public static set x(v:int){}}function f():void{const CSetX=(v:int):void=>{};C.x=1;}`, "shadows generated static member"},
		{"compound getter shadow", `class C{public static get x():int{return 1;}public static set x(v:int){}}function f():void{const CGetX=():int=>99;C.x++;}`, "shadows generated static member"},
		{"unused getter shadow", `class C{public static get x():int{return 1;}public static set x(v:int){}}function f():int{const CGetX=():int=>99;C.x=1;return CGetX();}`, ""},
		{"type parameter shadow", `class C{public static get x():int{return 1;}}function f<CGetX>():int{return C.x;}`, "shadows generated static member"},
		{"method local shadow", `class C{public static function read():int{return 1;}}function f():int{const CRead=():int=>99;return C.read();}`, "shadows generated static member"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.source), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
