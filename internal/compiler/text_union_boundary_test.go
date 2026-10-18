package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUTF8UnionBoundaryMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, source := range map[string]string{
		"scalar.km": `import go reflect from "reflect";
export constraint Scalar=~int|~string|~boolean;
export function same<T extends Scalar>(left:T,right:T):boolean{return reflect.DeepEqual(left,right);}
export function forward<T extends Scalar>(left:T,right:T):boolean{return same(left,right);}
export constraint Copies=~[2]string|~int;
export function copied<T extends Copies>(left:T,right:T):boolean{return reflect.DeepEqual(left,right);}
`,
		"entry.km": `import {same,forward,copied} from "./scalar";
import go reflect from "reflect";
type Text=distinct string;
constraint RawOrText=~bstring[]|~string;
function raw<T extends RawOrText>(left:T,right:T):boolean{return reflect.DeepEqual(left,right);}
export function Values():boolean[]{
 const pair:[2]string=["a","b"];
 return [same<int>(1,1),same<string>("金木犀","金木犀"),same<boolean>(true,false),
 same<Text>(Text("text"),Text("other")),forward("\u0000雪","\u0000雪"),
 copied(pair,pair),copied(2,3),
 raw<bstring[]>([b"\xff"],[b"\xff"]),raw("ok","no")];
}
function main():void{same<int>(1,1);}
`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "textunions")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	reference := `package reference
import "reflect"
type scalar interface{~int|~string|~bool}
func same[T scalar](left,right T)bool{return reflect.DeepEqual(left,right)}
func forward[T scalar](left,right T)bool{return same(left,right)}
type text string
func copied[T interface{~[2]string|~int}](left,right T)bool{return reflect.DeepEqual(left,right)}
func raw[T interface{~[]string|~string}](left,right T)bool{return reflect.DeepEqual(left,right)}
func Values()[]bool{return []bool{same[int](1,1),same[string]("金木犀","金木犀"),same[bool](true,false),
same[text](text("text"),text("other")),forward("\x00雪","\x00雪"),
copied([2]string{"a","b"},[2]string{"a","b"}),copied(2,3),
raw[[]string]([]string{"\xff"},[]string{"\xff"}),raw("ok","no")}}
`
	tests := `package textunions
import("reflect";"testing";reference "text-unions.test/reference")
func TestValues(t *testing.T){if got,want:=Values(),reference.Values();!reflect.DeepEqual(got,want){t.Fatalf("got %v; want %v",got,want)}}
`
	runGeneratedGoDifferentialTest(t, root, "text-unions.test", generated, reference, tests)
}
