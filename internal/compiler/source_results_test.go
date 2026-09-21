package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourceResultForwardingMatchesGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input := `import go strings from "strings";
import go strconv from "strconv";
import {Split} from "./types";
function Cut(s:string):(string,string,boolean){return strings.Cut(s,":");}
function apply(f:(s:string)=>(string,string,boolean),s:string):(string,string,boolean){return f(s);}
function Callback(s:string):(string,string,boolean){return apply((text)=>strings.Cut(text,":"),s);}
const Arrow=(s:string):(string,string,boolean)=>strings.Cut(s,":");
function generic<T>(f:(s:T)=>(T,T,boolean),s:T):(T,T,boolean){return f(s);}
function Generic(s:string):(string,string,boolean){return generic(Cut,s);}
function Alias(s:string):(string,string,boolean){const split:Split<string>=(text)=>strings.Cut(text,":");return split(s);}
function Block(s:string):(string,string,boolean){const split:Split<string>=(text)=>{return strings.Cut(text,":");};return split(s);}
interface Cutter<T>{function cut(s:T):(T,T,boolean);}
class Splitter implements Cutter<string> {public function cut(s:string):(string,string,boolean){return Cut(s);}}
function Method(s:string):(string,string,boolean){const splitter=new Splitter();return splitter.cut(s);}
function Interface(s:string):(string,string,boolean){const splitter:Cutter<string>=new Splitter();return splitter.cut(s);}
function Anonymous(s:string):(string,string,boolean){const splitter:interface{cut(s:string):(string,string,boolean);}=new Splitter();return splitter.cut(s);}
function Destructure(s:string):string{const [left,right,found]=Cut(s);if(found){return right+left;}return left;}
class Echo<T>{public function echo(value:T):T{return value;}}
function EchoValue(s:string):string{const value:interface{echo(value:string):string;}=new Echo<string>();return value.echo(s);}
function Parse(s:string):(int,error){return strconv.Atoi(s);}
function Twice(s:string):Result<int>{const value=Parse(s)?;return ok(value*2);}
`
	path := filepath.Join(root, "entry.km")
	if err := os.WriteFile(filepath.Join(root, "types.km"), []byte(`alias Split<T>=(s:T)=>(T,T,boolean);`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module source-results.test\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "results")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import ("strings";"strconv")
func Cut(s string)(string,string,bool){return strings.Cut(s,":")}
func EchoValue(s string)string{return s}
func Twice(s string)(int,error){value,err:=strconv.Atoi(s);if err!=nil{return 0,err};return value*2,nil}
`
	comparison := `package results_test
import("testing";g "source-results.test";r "source-results.test/reference")
func TestForwarding(t *testing.T){for _,s:=range []string{"a:b","missing",":","a:b:c","日本語:値"}{
 a,b,c:=r.Cut(s)
 for _,f:=range []func(string)(string,string,bool){g.Cut,g.Callback,g.Arrow,g.Method,g.Alias,g.Block,g.Interface,g.Anonymous,g.Generic}{
  x,y,z:=f(s);if x!=a||y!=b||z!=c{t.Fatalf("%q: got %q %q %v",s,x,y,z)}
 }
 want:=a;if c{want=b+a};if got:=g.Destructure(s);got!=want{t.Fatalf("destructure: %q != %q",got,want)}
 if g.EchoValue(s)!=r.EchoValue(s){t.Fatal("generic structural interface")}
}}
func TestErrors(t *testing.T){for _,s:=range []string{"42","-7","bad",""}{a,ae:=g.Twice(s);b,be:=r.Twice(s);if a!=b||(ae==nil)!=(be==nil){t.Fatalf("%q: %v %v != %v %v",s,a,ae,b,be)}}}
`
	runGeneratedGoDifferentialTest(t, root, "source-results.test", generated, reference, comparison)
}
