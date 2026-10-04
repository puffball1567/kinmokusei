package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUTF8RawInterfacesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, contents := range map[string]string{
		"library.km": `
export interface Label<T> { function label(value:T):bstring; }
export class RawRow<T> implements Label<T> {
  public text:bstring="";
  public function label(value:T):bstring { return this.text; }
}`,
		"main.km": `
import {Label,RawRow} from "./library";
import go reflect from "reflect";
alias View=interface { label(value:int):bstring; };
function overwrite(value:View):void {
  reflect.ValueOf(value).Elem().FieldByName("Text").SetString(b"\xff\x00");
}
function overwriteNamed(value:Label<int>):void {
  reflect.ValueOf(value).Elem().FieldByName("Text").SetString(b"\xff\x00");
}
export function Structural():bstring {
  const row=new RawRow<int>(); overwrite(row); return row.text;
}
export function Named():bstring {
  const row=new RawRow<int>(); overwriteNamed(row); return row.text;
}
export function Decode():Result<string> { return string(Structural()); }
`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "main.km")}, "textinterfaces")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import "reflect"
type row struct { Text string }
func (*row) Label(int)string{return "row"}
func Raw()string {
  var value interface{Label(int)string}=&row{}
  reflect.ValueOf(value).Elem().FieldByName("Text").SetString("\xff\x00")
  return value.(*row).Text
}`
	tests := `package textinterfaces
import("testing";"unicode/utf8";reference "text-interfaces.test/reference")
func TestBoundary(t *testing.T){
  for name,call:=range map[string]func()string{"structural":Structural,"named":Named}{
    got:=call();if got!=reference.Raw()||utf8.ValidString(got){t.Fatalf("%s: %q",name,got)}
  }
  if got,err:=Decode();err==nil||got!=""{t.Fatalf("invalid bytes became verified text: %q,%v",got,err)}
}`
	runGeneratedGoDifferentialTest(t, root, "text-interfaces.test", generated, reference, tests)
}

func TestUTF8ImportedInterfaceHiddenFieldsRejected(t *testing.T) {
	for _, contract := range []string{"Label<int>", "interface{label(value:int):bstring;}"} {
		t.Run(contract, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			entry := filepath.Join(root, "main.km")
			for name, contents := range map[string]string{
				"library.km": `export interface Label<T>{function label(value:T):bstring;}
export class Row<T> implements Label<T>{public text:string="";public function label(value:T):bstring{return "row";}}`,
				"main.km": `import {Label,Row} from "./library";import go reflect from "reflect";
function inspect(value:` + contract + `):void{reflect.ValueOf(value);}
function use():void{inspect(new Row<int>());}`,
			} {
				if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			generated, diagnostics, err := EmitGo([]string{entry}, "textinterfaces")
			if err != nil {
				t.Fatal(err)
			}
			for _, item := range diagnostics {
				if item.Span.Path == entry && strings.Contains(item.Message, "cannot use") && strings.HasSuffix(item.Message, " as any") {
					if len(generated) != 0 {
						t.Fatal("invalid source emitted Go")
					}
					return
				}
			}
			t.Fatalf("missing boundary diagnostic: %v", diagnostics)
		})
	}
}
