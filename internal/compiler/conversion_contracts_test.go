package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConversionContractsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"types.km": `export alias Maybe = *int | null;
export type Before = distinct () => Result<Maybe>;
export type After = distinct () => Result<Maybe>;
export type Raw = distinct () => (Maybe, error);
export type Values = distinct Maybe[];
export constraint Slice = ~Maybe[];
export function into<T extends Slice>(v: Maybe[]): T { return T(v); }`,
		"bridge.km": `export {Maybe, Before, After as Load, Raw, Values, into} from "./types";`,
		"entry.km": `import {Maybe, Before, Load, Raw, Values, into} from "./bridge";
import go errors from "errors";
import go {RawMessage as JSON} from "encoding/json";
export function Run(n: int): int[] {
 let value = n;
 const original: Maybe[] = [null, &value];
 const converted = into<Values>(original);
 converted[0] = &value;
 let first = 0; const p = original[0]; if(p !== null) { first = *p; }
 const before: Before = (): Result<Maybe> => { return ok(&value); };
 const [result, err] = Raw(before)();
 let second = 0; if(result !== null) { second = *result; }
 return [first, second, len(converted)];
}
export function ResultValue(failed: boolean): Result<int> {
 let value = 7;
 const before: Before = (): Result<Maybe> => { if(failed) { return fail(errors.New("expected")); } return ok(&value); };
 const result = Load(before)()?;
 if(result === null) { return ok(0); }
 return ok(*result);
}
export function JSONSize(v: byte[] | null): int {
 if(v === null) { return 0; }
 return len(JSON(v));
}`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "contracts")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import ("encoding/json"; "errors")
type before func()(*int,error)
type after func()(*int,error)
type raw func()(*int,error)
type values []*int
func into[T ~[]*int](v []*int)T{return T(v)}
func Run(n int)[]int{value:=n;original:=[]*int{nil,&value};converted:=into[values](original);converted[0]=&value;first:=0;if p:=original[0];p!=nil{first=*p};source:=before(func()(*int,error){return &value,nil});result,_:=raw(source)();second:=0;if result!=nil{second=*result};return []int{first,second,len(converted)}}
func ResultValue(failed bool)(int,error){value:=7;source:=before(func()(*int,error){if failed{return nil,errors.New("expected")};return &value,nil});result,err:=after(source)();if err!=nil{return 0,err};if result==nil{return 0,nil};return *result,nil}
func JSONSize(v []byte)int{if v==nil{return 0};return len(json.RawMessage(v))}`
	tests := `package contracts_test
import("reflect";"testing";g "conversion-contracts.test";r "conversion-contracts.test/reference")
func TestRun(t *testing.T){for _,n:=range []int{-7,0,42}{if got,want:=g.Run(n),r.Run(n);!reflect.DeepEqual(got,want){t.Fatal(got,want)}}}
func TestResult(t *testing.T){for _,failed:=range []bool{false,true}{got,ge:=g.ResultValue(failed);want,we:=r.ResultValue(failed);if got!=want||(ge==nil)!=(we==nil){t.Fatal(got,ge,want,we)};if ge!=nil&&we!=nil&&ge.Error()!=we.Error(){t.Fatal(ge,we)}}}
func TestJSON(t *testing.T){for _,v:=range [][]byte{nil,{},[]byte("hello")}{if got,want:=g.JSONSize(v),r.JSONSize(v);got!=want{t.Fatal(got,want)}}}`
	runGeneratedGoDifferentialTest(t, root, "conversion-contracts.test", generated, reference, tests)
}

func TestLinkedConversionContractDiagnostics(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	entry, types, bridge := filepath.Join(root, "entry.km"), filepath.Join(root, "types.km"), filepath.Join(root, "bridge.km")
	input := `import {Source, Target} from "./bridge";
function bad(value: Source): Target { return Target(value); }`
	result, err := CheckFilesWithOverlay([]string{entry}, map[string]string{
		entry:  input,
		types:  `export alias Maybe = *int | null; export type A = distinct () => Result<Maybe>; export type B = distinct () => Result<*int>;`,
		bridge: `export {A as Source, B as Target} from "./types";`,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range result.Diagnostics {
		if d.Span.Path == entry && strings.Contains(d.Message, "cannot convert") && input[d.Span.Start.Offset:d.Span.End.Offset] == "value" {
			return
		}
	}
	t.Fatalf("missing source conversion diagnostic: %v", result.Diagnostics)
}
