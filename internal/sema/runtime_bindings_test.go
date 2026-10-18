package sema

import (
	"strings"
	"testing"
)

func TestRejectsRuntimeHelperBindings(t *testing.T) {
	t.Parallel()
	contexts := map[string]string{
		"const":                 `function f():int{const NAME=5;try{return NAME;}finally{}}`,
		"let":                   `function f():int{let NAME=5;return NAME;}`,
		"parameter":             `function f(NAME:int):int{try{return NAME;}finally{}}`,
		"arrow parameter":       `const f=(NAME:int):int=>NAME;`,
		"constructor parameter": `class C{constructor(NAME:int){const n=NAME;}}`,
		"method parameter":      `class C{public function f(NAME:int):int{return NAME;}}`,
		"multi binding":         `function pair():(int,int){return 1,2;}function f():int{const [NAME,n]=pair();return NAME+n;}`,
		"range binding":         `function f():int{for(const NAME of [1]){return NAME;}return 0;}`,
		"range pair":            `function f():int{for(const [i,NAME] of [1]){return i+NAME;}return 0;}`,
		"select binding":        `function f(ch:GoChannel<int>):int{select{case const NAME=<-ch{return NAME;}default{return 0;}}}`,
		"type switch binding":   `import go os from "os";function f(value:error):void{switch(value){case const NAME as *os.PathError{const n=NAME;}default{}}}`,
		"catch binding":         `function f(value:error):void{try{throw value;}catch(NAME:error){const n=NAME;}}`,
		"for initializer":       `function f():void{for(let NAME=0;NAME<1;NAME++){}}`,
		"type parameter":        `function f<NAME>(value:NAME):NAME{return value;}`,
		"named import alias":    `import go {Println as NAME} from "fmt";function f():void{NAME(1);}`,
	}
	for context, template := range contexts {
		t.Run("ordinary/"+context, func(t *testing.T) {
			if got := checkSource(t, strings.ReplaceAll(template, "NAME", "ordinary")); len(got) != 0 {
				t.Fatalf("invalid test context: %v", got)
			}
		})
	}
	for _, name := range []string{
		"__kinmokusei", "__kinmokusei_existing_helper",
		"__kinmokuseiReturn", "__kinmokuseiReturnValue", "__kinmokuseiException",
		"__kinmokuseiThrown", "__kinmokuseiInitException", "__kinmokuseiExceptionFromError",
		"__kinmokuseiTask", "__kinmokuseiVoidTask", "__kinmokuseiResultTask", "__kinmokuseiVoidResultTask",
		"__kinmokuseiDecodeUTF8", "__kinmokuseiDecoratorAdapterError", "__kinmokuseiFutureHelper",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for context, template := range contexts {
				t.Run(context, func(t *testing.T) {
					input := strings.ReplaceAll(template, "NAME", name)
					if got := strings.Join(checkSource(t, input), "\n"); !strings.Contains(got, "compiler built-in") {
						t.Fatalf("expected reserved binding diagnostic, got %q", got)
					}
				})
			}
		})
	}
}

func TestRejectsShadowedConstructorFactory(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"exception local":          `function f():Exception{const NewException=1;return new Exception("message");}`,
		"exception parameter":      `function f(NewException:int):Exception{return new Exception("message");}`,
		"exception type parameter": `function f<NewException>():Exception{return new Exception("message");}`,
		"native local":             `class C{}function f():C{const NewC=1;return new C();}`,
		"native type parameter":    `class C{}function f<NewC>():C{return new C();}`,
		"generic class":            `class C<T>{constructor(public value:T){}}function f(NewC:int):C<int>{return new C<int>(1);}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := strings.Join(checkSource(t, input), "\n"); !strings.Contains(got, "shadows generated constructor") {
				t.Fatalf("expected constructor shadowing diagnostic, got %q", got)
			}
		})
	}
}

func TestRuntimeNameReservationDoesNotRejectUnrelatedNames(t *testing.T) {
	t.Parallel()
	input := `class C {public __kinmokuseiReturn:int=1;}
function f():int{
 const __kinmokuse=1;
 const kinmokuseiReturn=2;
 const __KinmokuseiReturn=3;
 const NewException=4;
 const NewC=5;
 const c:C|null=null;
 if(c!==null){return c.__kinmokuseiReturn;}
 try{return __kinmokuse+kinmokuseiReturn+__KinmokuseiReturn+NewException+NewC;}finally{}
}
`
	if got := checkSource(t, input); len(got) != 0 {
		t.Fatalf("unrelated names should remain usable: %v", got)
	}
}
