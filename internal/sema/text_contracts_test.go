package sema

import (
	"strings"
	"testing"
)

func TestUTF8TextContracts(t *testing.T) {
	tests := []struct{ name, source, want string }{
		{"text inference", `function f():string { const x="湯"; return x+"a"; }`, ""},
		{"raw literal", `function f():bstring { return b"\xff\x00"; }`, ""},
		{"raw inference", `function f():bstring { const x=b"\xff"; return x+b"a"; }`, ""},
		{"invalid text literal", `const x="\xff";`, "not valid UTF-8"},
		{"invalid octal", `const x="\377";`, "not valid UTF-8"},
		{"valid byte escape", `const x:string="\xc3\xa9";`, ""},
		{"raw assignment", `const x:string=b"ok";`, "cannot use bstring as string"},
		{"Go return", `import go strings from "strings"; function f():string { return strings.ReplaceAll("ok","o","O"); }`, "cannot use bstring as string"},
		{"Go checked return", `import go strings from "strings"; function f():Result<string> { return string(strings.ReplaceAll("ok","o","O")); }`, ""},
		{"checked propagation", `function f(x:bstring):Result<string> { const text=string(x)?; return ok(text); }`, ""},
		{"checked bytes", `function f(x:byte[]):Result<string> { return string(x); }`, ""},
		{"discard checked decode", `function f(x:bstring):void { string(x); }`, "Result"},
		{"explicit error discard", `function f(x:bstring):void { const [text,_]=string(x); }`, ""},
		{"raw widening", `function f(x:string):bstring { return x; }`, ""},
		{"raw concatenation", `function f():bstring { return "safe"+b"\xff"; }`, ""},
		{"named raw concatenation", `type Text=distinct string; function f():Text { return Text("safe")+b"\xff"; }`, "cannot concatenate"},
		{"generic raw concatenation", `constraint Text=~string;function f<T extends Text>(x:T):T{return x+b"\xff";}`, "cannot concatenate"},
		{"named raw ordered builtin", `type Text=distinct string; function f():Text { return min(Text("safe"),b"\xff"); }`, "cannot mix"},
		{"Go assertion", `import go reflect from "reflect";function f(value:reflect.Value):string{return value.Interface() as! string;}`, "cannot establish"},
		{"Go raw assertion", `import go reflect from "reflect";function f(value:reflect.Value):bstring{return value.Interface() as! bstring;}`, ""},
		{"reflection mutable text", `import go json from "encoding/json";struct Row{public name:string;}function f(row:*Row):Result<void>{json.Unmarshal([],row)?;return ok();}`, "cannot use"},
		{"reflection raw bytes", `import go json from "encoding/json";struct Row{public name:bstring;}function f(row:*Row):Result<void>{json.Unmarshal([],row)?;return ok();}`, ""},
		{"opaque mutable slice", `import go fmt from "fmt";function f(items:string[]):void{fmt.Println(items);}`, "cannot use"},
		{"Go generic mutable slice", `import go slices from "slices";function f(items:string[]):void{slices.Sort(items);}`, "cannot use"},
		{"Go generic raw slice", `import go slices from "slices";function f(items:bstring[]):void{slices.Sort(items);}`, ""},
		{"Go generic verified type", `import go atomic from "sync/atomic";let value:atomic.Pointer<string>=atomic.Pointer<string>{};`, "cannot establish"},
		{"Go generic raw type", `import go atomic from "sync/atomic";let value:atomic.Pointer<bstring>=atomic.Pointer<bstring>{};`, ""},
		{"recursive generic storage", `import go json from "encoding/json";class Box<T>{public item:T;public next:Box<string>|null=null;constructor(item:T){this.item=item;}}function f(value:Box<bstring>):Result<void>{json.Unmarshal([],value)?;return ok();}`, "cannot use"},
		{"Go generic named text", `import go cmp from "cmp";type Text=distinct string;function f():int{return cmp.Compare(Text("a"),Text("b"));}`, "verified native text"},
		{"Go generic unknown result", `import go slices from "slices";function f<T>(items:T[]):T[]{return slices.Clone(items);}`, "verified native text"},
		{"Go generic bounded result", `import go slices from "slices";constraint Raw=~bstring;function f<T extends Raw>(items:T[]):T[]{return slices.Clone(items);}`, ""},
		{"Go generic unknown storage", `import go atomic from "sync/atomic";function f<T>():void{const value=atomic.Pointer<T>{};}`, "cannot establish"},
		{"opaque generic erasure", `import go json from "encoding/json";function f<T>(value:T):Result<void>{json.Unmarshal([],value)?;return ok();}`, "cannot use"},
		{"opaque interface erasure", `import go json from "encoding/json";interface Row{function label():bstring;}class TextRow implements Row{public text:string="";public function label():bstring{return "row";}}function f(value:Row):Result<void>{json.Unmarshal([],value)?;return ok();}`, "cannot use"},
		{"opaque base erasure", `import go json from "encoding/json";class Base{constructor(){}}class Child extends Base{public text:string="";constructor(){super();}}function f(value:Base):Result<void>{json.Unmarshal([],value)?;return ok();}`, "cannot use"},
		{"opaque structural interface erasure", `import go json from "encoding/json";class TextRow{public text:string="";public function label():bstring{return "row";}}function f(value:interface{label():bstring;}):Result<void>{json.Unmarshal([],value)?;return ok();}function use():Result<void>{return f(new TextRow());}`, "cannot use"},
		{"structural interface raw storage", `import go json from "encoding/json";class RawRow{public text:bstring="";public function label():bstring{return "row";}}function f(value:interface{label():bstring;}):Result<void>{json.Unmarshal([],value)?;return ok();}function use():Result<void>{return f(new RawRow());}`, ""},
		{"structural interface generic owner", `import go reflect from "reflect";class Row<T>{public text:string="";public function label(value:T):bstring{return "row";}}function f(value:interface{label(value:int):bstring;}):void{reflect.ValueOf(value);}`, "cannot use"},
		{"structural interface hidden generic field", `import go reflect from "reflect";class Row<T>{constructor(public text:T){}public function label():bstring{return "row";}}function f(value:interface{label():bstring;}):void{reflect.ValueOf(value);}`, "cannot use"},
		{"structural interface unrelated class", `import go reflect from "reflect";class TextRow{public text:string="";public function label(value:int):bstring{return "row";}}class RawRow{public text:bstring="";public function label():bstring{return "row";}}function f(value:interface{label():bstring;}):void{reflect.ValueOf(value);}function use():void{f(new RawRow());}`, ""},
		{"structural interface generic raw owner", `import go reflect from "reflect";class Row<T>{constructor(public text:T){}public function label(value:T):bstring{return "row";}}function f(value:interface{label(value:bstring):bstring;}):void{reflect.ValueOf(value);}function use():void{f(new Row<bstring>(b"ok"));}`, ""},
		{"structural interface alias", `import go reflect from "reflect";alias Label=interface{label():bstring;};class TextRow{public text:string="";public function label():bstring{return "row";}}function f(value:Label):void{reflect.ValueOf(value);}`, "cannot use"},
		{"structural interface nested slice", `import go reflect from "reflect";class TextRow{public text:string="";public function label():bstring{return "row";}}function f(values:interface{label():bstring;}[]):void{reflect.ValueOf(values);}`, "cannot use"},
		{"named interface generic implementation", `import go reflect from "reflect";interface Label<T>{function label(value:T):bstring;}class TextRow<T> implements Label<T>{public text:string="";public function label(value:T):bstring{return "row";}}function f(value:Label<int>):void{reflect.ValueOf(value);}function use():void{f(new TextRow<int>());}`, "cannot use"},
		{"named interface generic raw implementation", `import go reflect from "reflect";interface Label<T>{function label(value:T):bstring;}class RawRow<T> implements Label<T>{public text:bstring="";public function label(value:T):bstring{return "row";}}function f(value:Label<int>):void{reflect.ValueOf(value);}function use():void{f(new RawRow<int>());}`, ""},
		{"named interface inherited generic implementation", `import go reflect from "reflect";interface Label<T>{function label(value:T):bstring;}interface Derived<T> extends Label<T>{}class TextRow<T> implements Derived<T>{public text:string="";public function label(value:T):bstring{return "row";}}function f(value:Label<int>):void{reflect.ValueOf(value);}function use():void{f(new TextRow<int>());}`, "cannot use"},
		{"structural interface incompatible owner bound", `import go reflect from "reflect";constraint Number=~int;class TextRow<T extends Number>{public text:string="";public function label(value:T):bstring{return "row";}}class RawRow{public text:bstring="";public function label(value:bstring):bstring{return "row";}}function f(value:interface{label(value:bstring):bstring;}):void{reflect.ValueOf(value);}function use():void{f(new RawRow());}`, ""},
		{"structural interface incompatible text bound", `import go reflect from "reflect";constraint Text=~string;class TextRow<T extends Text>{public text:string="";public function label(value:T):bstring{return "row";}}class RawRow{public text:bstring="";public function label(value:bstring):bstring{return "row";}}function f(value:interface{label(value:bstring):bstring;}):void{reflect.ValueOf(value);}function use():void{f(new RawRow());}`, ""},
		{"structural interface recursive raw implementation", `import go reflect from "reflect";class RawRow{public next:interface{label():bstring;}|null=null;public function label():bstring{return "row";}}function f(value:interface{label():bstring;}):void{reflect.ValueOf(value);}function use():void{f(new RawRow());}`, ""},
		{"runtime name collision", `function f(__kinmokuseiDecodeUTF8:int):int{return __kinmokuseiDecodeUTF8;}`, "compiler built-in"},
		{"named verified text", `type Text=distinct string; function f(x:bstring):Result<Text> { return Text(x); }`, ""},
		{"named raw text", `type Bytes=distinct bstring; function f(x:Bytes):string { return x; }`, "cannot use"},
		{"shared slices", `function f(x:string[]):bstring[] { return x; }`, "cannot use"},
		{"explicit shared slices", `alias Raw=bstring[]; function f(x:string[]):bstring[] { return Raw(x); }`, "cannot convert"},
		{"callback", `import go regexp from "regexp"; function f(r:*regexp.Regexp):bstring { return r.ReplaceAllStringFunc("a",(x:string):string=>x); }`, "cannot use"},
		{"verified generic slice", `constraint Text=~string; function f<T extends Text>(x:T):T { return x[:]; }`, ""},
		{"raw generic slice", `constraint Bytes=~bstring; function f<T extends Bytes>(x:T):T { return x[:]; }`, ""},
		{"text generic rejects raw", `constraint Text=~string; function f<T extends Text>(x:T):T{return x;} const bad=f(b"ok");`, "type parameter constraint"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if test.want == "" && len(diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", diagnostics)
			}
			if test.want != "" && !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics = %v; want %q", diagnostics, test.want)
			}
		})
	}
}
