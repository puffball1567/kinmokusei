package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUTF8StringsCompileAndRun(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "text.km")
	input := `
import go strings from "strings";
import go utf8 from "unicode/utf8";
alias Bytes=byte[];
type Text=distinct string;
constraint Textual=~string;
function decode(raw:bstring):Result<string> { return string(raw); }
function decodeBytes(raw:byte[]):Result<string> { return string(raw); }
function decodeNamed(raw:bstring):Result<Text> { return Text(raw); }
function upper(value:string):Result<string> { return string(strings.ToUpper(value)); }
function propagated(raw:bstring):Result<string> { const text=string(raw)?; return ok(text+"!"); }
function slice(value:string, low:int64, high:uint64):string { return value[low:high]; }
function tail(value:string, low:int):string { return value[low:]; }
function genericSlice<T extends Textual>(value:T):T { return value[:]; }
function namedSlice(value:Text):Text { return genericSlice(value)[0:3]; }
function rawSlice(value:string):bstring { return bstring(value)[:1]; }
function literal():bstring { const value=b"\xff\x00"; return value; }
function point(value:int32):string { return string(value); }
function valid(value:string):boolean { return utf8.ValidString(value); }
function snapshot(values:byte[]):Result<string> { const text=string(values)?; values[0]=0; return ok(text); }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "text")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	tests := `package text
import("testing"; "unicode/utf8"; reference "text.test/reference")
func TestText(t *testing.T) {
  for _, value:=range []string{"", "hello", "湯a", "\x00", "e\u0301"} {
    want,wantErr:=reference.Decode(value)
    got,err:=decode(value);if err!=nil||got!=value||!utf8.ValidString(got){t.Fatalf("decode(%q)=%q,%v",value,got,err)}
    if got!=want||(err==nil)!=(wantErr==nil){t.Fatal("reference mismatch")}
    named,err:=decodeNamed(value);if err!=nil||string(named)!=value{t.Fatalf("named decode: %q,%v",named,err)}
    got,err=propagated(value);if err!=nil||got!=value+"!"{t.Fatalf("propagation: %q,%v",got,err)}
  }
  for _,value:=range []string{"\xff", "\xc0\x80", "\xed\xa0\x80", "\xf4\x90\x80\x80", "\xe6\xb9"} {
    got,err:=decode(value);if err==nil||got!=""{t.Fatalf("invalid input accepted: %q -> %q,%v",value,got,err)}
    got,err=decodeBytes([]byte(value));if err==nil||got!=""{t.Fatal("invalid byte array accepted")}
    got,err=propagated(value);if err==nil||got!=""{t.Fatal("invalid propagation accepted")}
  }
  if got:=literal();got!="\xff\x00"{t.Fatalf("raw literal=%q",got)}
  if got:=rawSlice("湯a");got!="\xe6"{t.Fatalf("raw slice=%q",got)}
  if slice("湯a",0,3)!="湯"||tail("湯a",3)!="a"||namedSlice(Text("湯a"))!=Text("湯"){t.Fatal("valid slicing")}
  for _,bounds:=range[][2]int64{{0,1},{1,3},{-1,3},{4,3},{0,5}} {
    func(){defer func(){if recover()==nil{t.Errorf("slice did not reject %v",bounds)}}();slice("湯a",bounds[0],uint64(bounds[1]))}()
  }
  func(){defer func(){if recover()==nil{t.Error("overflow accepted")}}();slice("a",0,^uint64(0))}()
  bytes:=[]byte("湯");got,err:=snapshot(bytes);if err!=nil||got!="湯"||bytes[0]!=0{t.Fatal("snapshot aliases mutable bytes")}
  if point(65)!="A"||!valid("湯"){t.Fatal("code point or interop")}
  got,err=upper("hello");if err!=nil||got!="HELLO"{t.Fatal("checked Go return")}
}
`
	reference := `package reference
import("fmt"; "unicode/utf8")
func Decode(value string)(string,error){if !utf8.ValidString(value){return "",fmt.Errorf("invalid UTF-8")};return value,nil}
`
	runGeneratedGoDifferentialTest(t, temp, "text.test", generated, reference, tests)
}

func TestUTF8RuntimeInInferredArrows(t *testing.T) {
	for name, input := range map[string]string{
		"global":       `const decode=(raw:bstring):Result<string>=>{return string(raw);}; function execute(raw:bstring):Result<string>{return decode(raw);}`,
		"local":        `function execute(raw:bstring):Result<string>{const decode=(value:bstring):Result<string>=>{return string(value);};return decode(raw);}`,
		"named import": `import go {ValidString} from "unicode/utf8";function execute(raw:bstring):Result<string>{const text=string(raw)?;if(ValidString(text)){return ok(text);}return string(raw);}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			temp := t.TempDir()
			source := filepath.Join(temp, "text.km")
			if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
				t.Fatal(err)
			}
			generated, diagnostics, err := EmitGo([]string{source}, "text")
			if err != nil || len(diagnostics) != 0 {
				t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
			}
			reference := `package reference
import("errors";"unicode/utf8")
func Decode(raw string)(string,error){if !utf8.ValidString(raw){return "",errors.New("invalid UTF-8")};return raw,nil}`
			tests := `package text
import("testing";reference "text-arrows.test/reference")
func TestDecode(t *testing.T){for _,raw:=range []string{"湯", "", "\xff"}{got,err:=execute(raw);want,wantErr:=reference.Decode(raw);if got!=want||(err==nil)!=(wantErr==nil){t.Fatalf("%q: %q,%v",raw,got,err)}}}`
			runGeneratedGoDifferentialTest(t, temp, "text-arrows.test", generated, reference, tests)
		})
	}
}
