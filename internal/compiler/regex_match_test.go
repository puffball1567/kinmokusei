package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegexAndMatchMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"records.km": `export struct Entry<T> { public text: string; public values: T[] }
struct Regex { public code: int }
function legacy(v: Regex): int { return v.code }
export function Legacy(): int { return legacy(Regex{code: 42}) }
export const expression = re"^\p{L}+(\d+)$"
`,
		"main.km": `import { Entry, Legacy, expression } from "./records"
import go regexp from "regexp"

const duplicated: Regex = re"^\p{L}+(\d+)$"
let count = 0
function read(text: string, values: int[]): Entry<int> {
  count += 1
  return Entry<int>{text: text, values: values}
}
function guard(v: int): boolean {
  count += 10
  return v > 0
}
function match(v: int): int { return v + 1 }
function id<T>(v: T): T { return v }

export function Run(text: string, values: int[]): (int, int) {
  count = 0
  const result = match(read(text, values)) {
    case { text: re"^\p{L}+(\d+)$", values: [const v, 0] } if (guard(v)) => match(v)
    case { values: [] } => -1
    case { values: [const v] } => v
    default => -2
  }
  return result, count
}
export function Text(text: string): (boolean, boolean, string, string[]) {
  const other = regexp.MustCompile("^[[:alpha:]]+$")
  const copy = id(duplicated)
  const test = copy.test
  return test(text), other.MatchString(text), expression.replaceAll(text, "<${1}>"), expression.captures(text)
}
export function Quoted(text: string): boolean { return re"^\".*\"$".test(text) }
export function Empty(): boolean { return makeSlice<Regex>(1)[0].test("text") }
export function Bool(v: boolean): string {
  return match(v) { case false => "off", case true => "on" }
}
export function Nullable(v: *int | null): int {
  return match(v) { case null => 0, case const value! => *value }
}
export function Fixed(v: [2]int): int {
  return match(v) { case [const lhs, const rhs] => lhs + rhs }
}
export function Context(v: boolean): int[] {
  return match(v) { case false => [], case true => [1, 2] }
}
export function Shadow(v: int): int {
  return match(v) { case const match => match }
}
export function OrdinaryCall(): int {
  match(1)
  { const v = 2 }
  switch (3) {
    case match(2) { return Legacy() }
    default { return 0 }
  }
}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "main.km")}, "patterns")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	if count := strings.Count(string(generated), `MustCompile("^\\p{L}+(\\d+)$")`); count != 1 {
		t.Fatalf("expected one compiled literal, got %d\n%s", count, generated)
	}
	reference := `package reference
import "regexp"
var expression = regexp.MustCompile(` + "`" + `^\p{L}+(\d+)$` + "`" + `)
var ascii = regexp.MustCompile("^[[:alpha:]]+$")
var quoted = regexp.MustCompile(` + "`" + `^".*"$` + "`" + `)
func Run(text string, values []int)(int,int) {
 calls:=1
 if expression.MatchString(text)&&len(values)==2&&values[1]==0 {
  calls+=10
  if values[0]>0 {return values[0]+1,calls}
 }
 switch len(values) {case 0:return -1,calls;case 1:return values[0],calls}
 return -2,calls
}
func Text(text string)(bool,bool,string,[]string) {
 return expression.MatchString(text),ascii.MatchString(text),expression.ReplaceAllString(text,"<${1}>"),expression.FindStringSubmatch(text)
}
func Quoted(text string)bool{return quoted.MatchString(text)}
`
	comparison := `package patterns_test
import("reflect";"testing";g "regex-match.test";r "regex-match.test/reference")
func TestPatterns(t *testing.T){
 for _,text:=range []string{"abc","月42","月","abc99","42","", "\"sample\""} {
  for _,values:=range [][]int{nil,{3},{3,0},{-3,0},{2,3},{1,2,3}} {
   got,steps:=g.Run(text,values);want,wsteps:=r.Run(text,values)
   if got!=want||steps!=wsteps{t.Fatalf("%q %v: got %d/%d want %d/%d",text,values,got,steps,want,wsteps)}
  }
  a,b,c,d:=g.Text(text);w,x,y,z:=r.Text(text)
  if a!=w||b!=x||c!=y||!reflect.DeepEqual(d,z)||g.Quoted(text)!=r.Quoted(text){t.Fatal(text,a,b,c,d,w,x,y,z)}
 }
 n:=12
 if !g.Empty()||g.Bool(false)!="off"||g.Bool(true)!="on"||g.Nullable(nil)!=0||g.Nullable(&n)!=12||g.Fixed([2]int{3,4})!=7||g.Shadow(5)!=5||g.OrdinaryCall()!=42{t.Fatal("value patterns")}
 if len(g.Context(false))!=0||!reflect.DeepEqual(g.Context(true),[]int{1,2}){t.Fatal("context")}
}
`
	runGeneratedGoDifferentialTest(t, root, "regex-match.test", generated, reference, comparison)
}
