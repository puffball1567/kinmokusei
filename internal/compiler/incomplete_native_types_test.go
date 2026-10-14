package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIncompleteNativeTypesDiagnosedBeforeEmission(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`type A=distinct Map<A,int>;`,
		`type A=distinct Map<[2]A,int>;`,
		`type K=distinct [1]S;struct S{public m:Map<K,int>;}`,
		`type A<T>=distinct Map<A<T>,T>;function f(v:A<int>):void{}`,
		`struct Box<T extends comparable>{public v:int;}type A=distinct Map<int,*Box<A>>;`,
		`import go reflect from "reflect";struct Box<T extends comparable>{public text:string;public v:T;}type A=distinct *Box<A>;function f(v:A):void{reflect.ValueOf(v);}`,
	} {
		t.Run(input, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "invalid.km")
			if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
				t.Fatal(err)
			}
			checked, err := CheckFiles([]string{path})
			if err != nil || len(checked.Diagnostics) == 0 {
				t.Fatalf("check err=%v diagnostics=%v", err, checked.Diagnostics)
			}
			generated, diagnostics, err := EmitGo([]string{path}, "invalid")
			if err != nil || len(diagnostics) == 0 || len(generated) != 0 {
				t.Fatalf("emit err=%v diagnostics=%v bytes=%d", err, diagnostics, len(generated))
			}
			for _, diagnostic := range diagnostics {
				if diagnostic.Span.Path != path || diagnostic.Span.Start.Line < 1 || strings.Contains(diagnostic.Message, "generated Go") {
					t.Fatalf("expected source diagnostic: %v", diagnostic)
				}
			}
		})
	}
}

func TestRecursiveNativeStorageMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, input := range map[string]string{
		"types.km": `export type PointerKey=distinct *Map<PointerKey,int>;
export type ChannelKey=distinct GoChannel<Map<ChannelKey,int>>;
export type Box<T extends comparable>=distinct GoChannel<T>;
export type Ref=distinct *Box<Ref>;
export struct Record<T extends comparable>{public value:int;}
export type RecordRef=distinct *Record<RecordRef>;
export type Generic<E>=distinct Map<*Generic<E>,E>;
export struct Table{public m:Map<Key,int>;}
export struct Key{public id:int;}
`,
		"entry.km": `import {PointerKey,ChannelKey,Box,Ref,Record,RecordRef,Generic,Table,Key} from "./types";
export function Pointer():int{let values=makeMap<PointerKey,int>();const key=PointerKey(&values);values[nil]=7;values[key]=11;return values[nil]+values[key]+len(*key);}
export function Channel():int{const values=makeMap<ChannelKey,int>();const key=ChannelKey(goChannel<Map<ChannelKey,int>>(1));values[key]=19;key<-values;const received=<-key;return received[key]+len(key);}
export function Constrained():int{let channel=Box<Ref>(goChannel<Ref>(2));const value=Ref(&channel);channel<-value;return len(*value);}
export function Structure():int{let row=Record<RecordRef>{value:23};const value=RecordRef(&row);const values=makeMap<RecordRef,int>();values[value]=row.value;return values[RecordRef(&row)];}
export function GenericValue():int{let values=Generic<int>(makeMap<*Generic<int>,int>());values[&values]=29;return values[&values];}
export function Forward():int{const table=Table{m:makeMap<Key,int>()};table.m[Key{id:3}]=31;return table.m[Key{id:3}];}
`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entry := []string{filepath.Join(root, "entry.km")}
	generated, diagnostics, err := EmitGo(entry, "recursive")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap(entry, "recursive")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(mapped.GoSource, generated) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("source map err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type pointerKey *map[pointerKey]int
type channelKey chan map[channelKey]int
type box[T comparable]chan T
type ref *box[ref]
type record[T comparable]struct{Value int}
type recordRef *record[recordRef]
type generic[E any]map[*generic[E]]E
type table struct{M map[key]int};type key struct{Id int}
func Pointer()int{values:=make(map[pointerKey]int);k:=pointerKey(&values);values[nil]=7;values[k]=11;return values[nil]+values[k]+len(*k)}
func Channel()int{values:=make(map[channelKey]int);k:=channelKey(make(chan map[channelKey]int,1));values[k]=19;k<-values;received:=<-k;return received[k]+len(k)}
func Constrained()int{channel:=box[ref](make(chan ref,2));v:=ref(&channel);channel<-v;return len(*v)}
func Structure()int{row:=record[recordRef]{23};v:=recordRef(&row);values:=make(map[recordRef]int);values[v]=row.Value;return values[recordRef(&row)]}
func GenericValue()int{values:=generic[int](make(map[*generic[int]]int));values[&values]=29;return values[&values]}
func Forward()int{v:=table{make(map[key]int)};v.M[key{3}]=31;return v.M[key{3}]}
`
	comparison := `package recursive_test
import("testing";g "recursive-storage.test";r "recursive-storage.test/reference")
func TestStorage(t *testing.T){
 if got,want:=g.Pointer(),r.Pointer();got!=want{t.Fatal("pointer key",got,want)}
 if got,want:=g.Channel(),r.Channel();got!=want{t.Fatal("channel key",got,want)}
 if got,want:=g.Constrained(),r.Constrained();got!=want{t.Fatal("constraint",got,want)}
 if got,want:=g.Structure(),r.Structure();got!=want{t.Fatal("struct",got,want)}
 if got,want:=g.GenericValue(),r.GenericValue();got!=want{t.Fatal("generic",got,want)}
 if got,want:=g.Forward(),r.Forward();got!=want{t.Fatal("forward key",got,want)}
}
`
	runGeneratedGoDifferentialTest(t, root, "recursive-storage.test", generated, reference, comparison)
}
