package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenericCollectionMutationMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module generic-collection-mutation.test\n\ngo 1.23\n",
		"contracts/contracts.go": `package contracts
type Maps[K comparable] interface{~map[K]int|~map[K]string}
type Both interface{~[]int|~map[string]int}
type Narrow interface{Both;~[]int}
type Empty interface{~[]int;~map[string]int}
`,
		"bounds.km": `
export constraint Resettable=~int[]|~string[]|~Map<string,int>|~Map<float,int>;
export constraint Lookup=~Map<string,int>|~Map<string,string>;
export function reset<T extends Resettable>(value:T):void{clear(value);}
export function remove<T extends Lookup>(value:T,key:string):void{delete(value,key);}
`,
		"entry.km": `
import {Resettable,Lookup,reset,remove} from "./bounds";
import go math from "math";
import go contracts from "generic-collection-mutation.test/contracts";
type Numbers=distinct int[];
type Table=distinct Map<string,int>;
constraint KeyMaps<K extends comparable>=contracts.Maps<K>;
function removeKey<K extends comparable,M extends KeyMaps<K>>(value:M,key:K):void{delete(value,key);}
function imported<T extends contracts.Narrow>(value:T):void{clear(value);}
class Cleaner<T extends Resettable>{constructor(private value:T){}public function reset():void{clear(this.value);}}
class Remover{public function remove<T extends Lookup>(value:T):void{delete(value,"drop");}}
class Key{constructor(public id:int){}}
constraint NullableKeys=~Map<Key|null,int>|~Map<Key|null,string>;
function removeNull<T extends NullableKeys>(value:T):void{delete(value,null);}
export function Slice():int[]{const values=Numbers([1,2,3,4]);const alias=values;const part=values[1:3];reset(part);new Cleaner<Numbers>(values[:1]).reset();const nilSlice:Numbers=nil;imported(nilSlice);return [alias[0],alias[1],alias[2],alias[3],len(part),cap(part),len(nilSlice)];}
export function Maps():int[]{const values=Table(makeMap<string,int>());values["drop"]=1;values["keep"]=2;const alias=values;remove(values,"absent");new Remover().remove(values);const kept=len(alias);const text=makeMap<string,string>();text["drop"]="x";removeKey(text,"drop");reset(values);const nilMap:Table=nil;remove(nilMap,"x");reset(nilMap);const nan=makeMap<float,int>();nan[math.NaN()]=1;nan[math.NaN()]=2;const before=len(nan);reset(nan);return [kept,len(alias),len(text),before,len(nan),len(nilMap)];}
export function Keys():int[]{const values=makeMap<Key|null,int>();const key=new Key(7);values[key]=1;values[null]=2;removeNull(values);return [len(values),values[key]];}
function inside<T extends Lookup>(value:T,trace:*int):void{const first=():T=>{*trace=*trace*10+4;return value;};const second=():string=>{*trace=*trace*10+5;return "x";};delete(first(),second());clear(first());}
export function Order():int[]{let trace=0;const values=makeMap<string,int>();values["x"]=1;const map=():Map<string,int>=>{trace=trace*10+1;return values;};const key=():string=>{trace=trace*10+2;return "x";};remove(map(),key());const resettable=():Map<string,int>=>{trace=trace*10+3;return values;};reset(resettable());inside(values,&trace);return [trace,len(values)];}
function removeByte<M extends KeyMaps<byte>>(value:M,n:int):void{const key=min(255,256);delete(value,key);delete(value,2.0);delete(value,1<<n);}
export function Constants(n:int):int{const values=makeMap<byte,int>();values[2]=1;values[4]=2;values[255]=3;removeByte(values,n);return len(values);}
export function Strings():string[]{const values:string[]=["a","b"];reset(values);return values;}
export function Panic():void{const values=makeMap<string,int>();const key=():string=>{const missing:string[]=[];return missing[0];};remove(values,key());}
export function Shadow():int{let calls=0;const clear=(value:int):void=>{calls+=value;};const delete=(value:int,key:int):void=>{calls+=value+key;};clear(2);delete(3,4);return calls;}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "genericmutation")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import "math"
type resettable interface{~[]int|~[]string|~map[string]int|~map[float64]int}
type lookup interface{~map[string]int|~map[string]string}
type maps[K comparable] interface{~map[K]int|~map[K]string}
type numbers []int
type table map[string]int
func reset[T resettable](value T){clear(value)}
func remove[T lookup](value T,key string){delete(value,key)}
func removeKey[K comparable,M maps[K]](value M,key K){delete(value,key)}
func Slice()[]int{values:=numbers{1,2,3,4};alias:=values;part:=values[1:3];reset(part);reset(values[:1]);var nilSlice numbers;reset(nilSlice);return []int{alias[0],alias[1],alias[2],alias[3],len(part),cap(part),len(nilSlice)}}
func Maps()[]int{values:=table{"drop":1,"keep":2};alias:=values;remove(values,"absent");remove(values,"drop");kept:=len(alias);text:=map[string]string{"drop":"x"};removeKey(text,"drop");reset(values);var nilMap table;remove(nilMap,"x");reset(nilMap);nan:=map[float64]int{};nan[math.NaN()]=1;nan[math.NaN()]=2;before:=len(nan);reset(nan);return []int{kept,len(alias),len(text),before,len(nan),len(nilMap)}}
type key struct{id int}
func removeNull[T interface{~map[*key]int|~map[*key]string}](value T){delete(value,nil)}
func Keys()[]int{values:=map[*key]int{};k:=&key{7};values[k]=1;values[nil]=2;removeNull(values);return []int{len(values),values[k]}}
func inside[T lookup](value T,trace *int){first:=func()T{*trace=*trace*10+4;return value};second:=func()string{*trace=*trace*10+5;return "x"};delete(first(),second());clear(first())}
func Order()[]int{trace:=0;values:=map[string]int{"x":1};m:=func()map[string]int{trace=trace*10+1;return values};key:=func()string{trace=trace*10+2;return "x"};remove(m(),key());resettable:=func()map[string]int{trace=trace*10+3;return values};reset(resettable());inside(values,&trace);return []int{trace,len(values)}}
func removeByte[M maps[byte]](value M,n int){const key=min(255,256);delete(value,key);delete(value,2.0);delete(value,1<<n)}
func Constants(n int)int{values:=map[byte]int{2:1,4:2,255:3};removeByte(values,n);return len(values)}
func Strings()[]string{values:=[]string{"a","b"};reset(values);return values}
func Panic(){values:=map[string]int{};key:=func()string{missing:=[]string{};return missing[0]};remove(values,key())}
func Shadow()int{calls:=0;clear:=func(value int){calls+=value};delete:=func(value,key int){calls+=value+key};clear(2);delete(3,4);return calls}
`
	comparison := `package genericmutation_test
import("testing";"reflect";g "generic-collection-mutation.test";r "generic-collection-mutation.test/reference")
func panics(f func())(yes bool){defer func(){yes=recover()!=nil}();f();return}
func TestMutation(t *testing.T){for _,pair:=range [][2][]int{{g.Slice(),r.Slice()},{g.Maps(),r.Maps()},{g.Keys(),r.Keys()},{g.Order(),r.Order()}}{if !reflect.DeepEqual(pair[0],pair[1]){t.Fatalf("got %v want %v",pair[0],pair[1])}};if !reflect.DeepEqual(g.Strings(),r.Strings())||g.Shadow()!=r.Shadow(){t.Fatal("strings/shadow")};for _,n:=range []int{0,1,2,8,99}{if g.Constants(n)!=r.Constants(n){t.Fatal("keys")}};if !panics(g.Panic)||!panics(r.Panic)||!panics(func(){g.Constants(-1)})||!panics(func(){r.Constants(-1)}){t.Fatal("panic")}}
`
	runGeneratedGoDifferentialTest(t, root, "generic-collection-mutation.test", generated, reference, comparison)
}
