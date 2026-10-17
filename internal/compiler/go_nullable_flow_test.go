package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestGoNullableFlowMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dependency := filepath.Join(root, "types.km")
	entry := filepath.Join(root, "entry.km")
	if err := os.WriteFile(dependency, []byte(`
import go flag from "flag";
export alias MaybeFlag = *flag.Flag | null;
export class Holder { constructor(public value: MaybeFlag) {} }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(`
import go flag from "flag";
import go errors from "errors";
import { MaybeFlag, Holder } from "./types";
alias Action = () => int;

export function Run(present: boolean): int[] {
  const flags = flag.NewFlagSet("nullable-flow", flag.ContinueOnError);
  const enabled = flags.Bool("known", false, "example");
  let name = "missing";
  if (present) { name = "known"; }
  let calls = 0;
  const lookup = (): *flag.Flag => { calls++; return flags.Lookup(name); };
  const value: MaybeFlag = lookup();
  let score = 0;
  if (value === null) { score += 1; }
  else {
    const copy: MaybeFlag = value;
    if (copy.Name === "known") { score += 2; }
  }
  const holder = new Holder(null);
  holder.value = lookup();
  if (holder.value !== null && holder.value.Name === "known") { score += 4; }
  let reassigned: MaybeFlag = null;
  reassigned = lookup();
  if (null === reassigned) { score += 8; }
  else { if (reassigned.Name === "known") { score += 16; } }
  const unwrapped: error | null = errors.Unwrap(errors.New("plain"));
  if (unwrapped === null) { score += 32; }
  flags.VisitAll((item: *flag.Flag): void => {
    const optional: MaybeFlag = item;
    if (optional !== null && optional.Name === "known") { calls++; }
  });
  return [score, calls];
}

export function ErrorAbsent(value: error): boolean {
  const optional: error | null = value;
  return optional === null;
}

export function Invoke(value: Action): int {
  const optional: Action | null = value;
  if (optional === null) { return -1; }
  return optional();
}

export function Direct(value: *flag.Flag): boolean { return value === nil; }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	paths := []string{entry}
	generated, diagnostics, err := EmitGo(paths, "nullablego")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap(paths, "nullablego")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("mapped emission differs: err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import("flag";"errors")
func Run(present bool) []int {
 flags:=flag.NewFlagSet("nullable-flow",flag.ContinueOnError)
 flags.Bool("known",false,"example")
 name:="missing";if present{name="known"}
 calls:=0;lookup:=func()*flag.Flag{calls++;return flags.Lookup(name)}
 value:=lookup();score:=0
 if value==nil{score+=1}else{copy:=value;if copy.Name=="known"{score+=2}}
 holder:=struct{value *flag.Flag}{};holder.value=lookup()
 if holder.value!=nil&&holder.value.Name=="known"{score+=4}
 reassigned:=lookup();if reassigned==nil{score+=8}else{if reassigned.Name=="known"{score+=16}}
 unwrapped:=errors.Unwrap(errors.New("plain"));if unwrapped==nil{score+=32}
 flags.VisitAll(func(item *flag.Flag){optional:=item;if optional!=nil&&optional.Name=="known"{calls++}})
 return []int{score,calls}
}
func ErrorAbsent(value error)bool{return value==nil}
func Invoke(value func()int)int{if value==nil{return -1};return value()}
func Direct(value *flag.Flag)bool{return value==nil}
`
	tests := `package nullablego_test
import("errors";"flag";"reflect";"testing";g "nullable-go.test";r "nullable-go.test/reference")
type typedError struct{}
func (*typedError)Error()string{return "typed nil"}
func TestFlow(t *testing.T){
 for _,present:=range []bool{false,true}{if got,want:=g.Run(present),r.Run(present);!reflect.DeepEqual(got,want){t.Fatal(present,got,want)}}
 var typedNil *typedError
 for _,value:=range []error{nil,errors.New("failure"),typedNil}{if got,want:=g.ErrorAbsent(value),r.ErrorAbsent(value);got!=want{t.Fatalf("nil interface: %T got=%v want=%v",value,got,want)}}
 for _,value:=range []func()int{nil,func()int{return 42}}{if got,want:=g.Invoke(value),r.Invoke(value);got!=want{t.Fatal(got,want)}}
 for _,value:=range []*flag.Flag{nil,{Name:"present"}}{if got,want:=g.Direct(value),r.Direct(value);got!=want{t.Fatal(got,want)}}
}
`
	runGeneratedGoDifferentialTest(t, root, "nullable-go.test", generated, reference, tests)
}
