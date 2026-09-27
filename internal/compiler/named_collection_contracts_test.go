package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNamedCollectionContractsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"types.km": `export alias Maybe=*int|null;
export type Seq<T>=distinct T[];
export type Fixed<T>=distinct [2]T;
export type Queue<T>=distinct GoChannel<T>;
export type Keys<T>=distinct Map<T,int>;
export type Iter<T>=distinct (emit:(item:T)=>boolean)=>void;`,
		"bridge.km": `export {Maybe,Seq as List,Fixed,Queue,Keys,Iter} from "./types";`,
		"entry.km": `import {Maybe,List,Fixed,Queue,Keys,Iter} from "./bridge";
export function Run(seed:int):int[]{
 let n=seed;const raw:Maybe[]=[null,&n];const values:List<Maybe>=raw;
 let count=0;let sum=0;
 for(const item of values){if(item===null){count++;}else{sum+=*item;}}
 const array:[2]Maybe=[null,&n];const fixed=Fixed<Maybe>(array);
 for(const item of fixed){if(item===null){count++;}else{sum+=*item;}}
 for(const item of &fixed){if(item===null){count++;}else{sum+=*item;}}
 const queue=Queue<Maybe>(goChannel<Maybe>(2));queue<-null;
 select{case queue<-&n{}default{}}
 closeGoChannel(queue);
 const [first,open]=<-queue;if(first===null&&open){count++;}
 select{case const [item,ok]=<-queue{if(item!==null&&ok){sum+=*item;}}default{}}
 const ranged=Queue<Maybe>(goChannel<Maybe>(2));ranged<-null;ranged<-&n;closeGoChannel(ranged);
 for(const item of ranged){if(item===null){count++;}else{sum+=*item;}}
 const keys=Keys<Maybe>(makeMap<Maybe,int>());keys[null]=1;keys[&n]=2;
 for(const [key,value] of keys){if(key===null){count+=value;}else{sum+=*key;}}
 const pair=():(Keys<Maybe>,Maybe)=>{return keys,null;};delete(pair());
 const visit:Iter<Maybe>=(emit:(item:Maybe)=>boolean):void=>{for(const item of values){if(!emit(item)){return;}}};
 for(const item of visit){if(item===null){count++;}else{sum+=*item;}}
 return [count,sum,len(keys)];
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
type list[T any] []T
type fixed[T any] [2]T
type queue[T any] chan T
type keys[T comparable] map[T]int
type iter[T any] func(func(T)bool)
func Run(seed int)[]int{
 n:=seed;raw:=[]*int{nil,&n};var values list[*int]=raw;count,sum:=0,0
 for _,p:=range values{if p==nil{count++}else{sum+=*p}}
 a:=fixed[*int]([2]*int{nil,&n});for _,p:=range a{if p==nil{count++}else{sum+=*p}};for _,p:=range &a{if p==nil{count++}else{sum+=*p}}
 ch:=queue[*int](make(chan *int,2));ch<-nil;select{case ch<-&n:default:};close(ch);first,open:=<-ch;if first==nil&&open{count++};select{case p,ok:=<-ch:if p!=nil&&ok{sum+=*p};default:}
 ranged:=queue[*int](make(chan *int,2));ranged<-nil;ranged<-&n;close(ranged);for p:=range ranged{if p==nil{count++}else{sum+=*p}}
 ks:=keys[*int](make(map[*int]int));ks[nil]=1;ks[&n]=2;for k,v:=range ks{if k==nil{count+=v}else{sum+=*k}};pair:=func()(keys[*int],*int){return ks,nil};m,k:=pair();delete(m,k)
 var visit iter[*int]=func(emit func(*int)bool){for _,p:=range values{if !emit(p){return}}};for p:=range visit{if p==nil{count++}else{sum+=*p}}
 return []int{count,sum,len(ks)}
}`
	tests := `package contracts_test
import("reflect";"testing";g "named-collection-contracts.test";r "named-collection-contracts.test/reference")
func TestRun(t *testing.T){for _,n:=range []int{-5,0,17}{if got,want:=g.Run(n),r.Run(n);!reflect.DeepEqual(got,want){t.Fatal(got,want)}}}`
	runGeneratedGoDifferentialTest(t, root, "named-collection-contracts.test", generated, reference, tests)
}
