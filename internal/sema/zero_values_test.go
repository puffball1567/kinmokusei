package sema

import (
	"fmt"
	"strings"
	"testing"
)

func TestZeroSafetySharedStructGraph(t *testing.T) {
	c := &Checker{structs: map[string]*structSymbol{}}
	value := builtins["int"]
	for i := 0; i < 48; i++ {
		name := fmt.Sprintf("Layer%d", i)
		c.structs[name] = &structSymbol{fields: map[string]fieldSymbol{"left": {typeInfo: value}, "right": {typeInfo: value}}}
		value = Type{Kind: Struct, Name: name}
	}
	if !c.zeroValueSafe(value) {
		t.Fatal("shared finite struct graph must have a safe zero value")
	}
}

func TestRejectsUnsafeSliceZeroValues(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"mixed nullable alternatives": `alias P=*int;alias M=*int|null;type A=distinct P[];type B=distinct M[];constraint C=B|A;function f<T extends C>():T{return make<T>(1);}`,
		"forward generic field":       `class User{}struct Outer{public inner:Inner<int>;}struct Inner<T>{public user:User;}function f():void{const xs=makeSlice<Outer>(1);}`,
		"class":                       `class User{}function f():void{const xs=makeSlice<User>(1);}`,
		"pointer":                     `function f():void{const xs=makeSlice<*int>(1);}`,
		"interface":                   `interface I{function read():int;}function f():void{const xs=makeSlice<I>(1);}`,
		"callback":                    `alias F=()=>int;function f():void{const xs=makeSlice<F>(1);}`,
		"nested struct":               `class User{}struct Row{private user:User;}function f():void{const xs=makeSlice<Row>(1);}`,
		"array":                       `class User{}function f():void{const xs=makeSlice<[2]User>(1);}`,
		"object":                      `class User{}alias Row={user:User};function f():void{const xs=makeSlice<Row>(1);}`,
		"generic struct":              `class User{}struct Row<T>{public value:T;}function f():void{const xs=makeSlice<Row<User>>(1);}`,
		"unknown parameter":           `function f<T>():T[]{return makeSlice<T>(1);}`,
		"mixed parameter":             `class User{}constraint C=int|User;function f<T extends C>():T[]{return makeSlice<T>(1);}`,
		"dynamic length":              `class User{}function f(n:int):User[]{return makeSlice<User>(n);}`,
		"named slice":                 `alias Pointer=*int;type Pointers=distinct Pointer[];function f():Pointers{return make<Pointers>(1);}`,
		"generic make":                `constraint S<E>=~E[];function f<E,T extends S<E>>():T{return make<T>(1);}`,
		"clear class":                 `class User{}function f(xs:User[]):void{clear(xs);}`,
		"clear nullable container":    `class User{}function f(xs:User[]|null):void{clear(xs);}`,
		"clear generic struct":        `class User{}struct Row<T>{public value:T;}function f(xs:Row<User>[]):void{clear(xs);}`,
		"clear mixed bound":           `class User{}constraint C=~User[]|~Map<int,User>;function f<T extends C>(xs:T):void{clear(xs);}`,
		"clear unknown element":       `constraint C<E>=~E[];function f<E,T extends C<E>>(xs:T):void{clear(xs);}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if diagnostics := strings.Join(checkSource(t, input), "\n"); !strings.Contains(diagnostics, "zero value") {
				t.Fatalf("unsafe zero construction accepted: %s", diagnostics)
			}
		})
	}
}

func TestSliceZeroSafetyRetainsInitializedAndNullableValues(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"finite nullable bound":   `class User{}alias Maybe=User|null;constraint C=int|Maybe;function f<T extends C>(n:int):T[]{return makeSlice<T>(n);}`,
		"empty reserved class":    `class User{}function f(n:int):User[]{let xs=makeSlice<User>(0,n);return append(xs,new User());}`,
		"empty unknown":           `function f<T>(n:int):T[]{return makeSlice<T>(0,n);}`,
		"empty generic make":      `constraint S<E>=~E[];function f<E,T extends S<E>>(n:int):T{return make<T>(0,n);}`,
		"nullable class":          `class User{}alias Maybe=User|null;function f():Maybe[]{const xs=makeSlice<Maybe>(2);clear(xs);return xs;}`,
		"nullable pointer":        `alias P=*int|null;function f():P[]{return makeSlice<P>(2);}`,
		"nullable struct member":  `class User{}struct Row{public user:User|null;}function f():Row[]{const xs=makeSlice<Row>(2);clear(xs);return xs;}`,
		"nullable generic member": `class User{}struct Row<T>{public value:T;}function f():Row<User|null>[]{return makeSlice<Row<User|null>>(2);}`,
		"empty array":             `class User{}alias Empty=[0]User;function f():Empty[]{return makeSlice<Empty>(2);}`,
		"containers":              `class User{}function f():void{const a=makeSlice<User[]>(2);const b=makeSlice<Map<int,User>>(2);const ch=makeSlice<GoChannel<User>>(2);clear(a);clear(b);clear(ch);}`,
		"map removal":             `class User{}function f(xs:Map<int,User>):void{clear(xs);}`,
		"generic map removal":     `class User{}constraint C=~Map<int,User>|~Map<int,*int>;function f<T extends C>(xs:T):void{clear(xs);}`,
		"finite safe bound":       `constraint Number=~int|~int8;function f<T extends Number>(n:int):T[]{return makeSlice<T>(n);}`,
		"go zero struct":          `import go time from "time";function f():time.Time[]{return makeSlice<time.Time>(2);}`,
		"safe recursive record":   `struct Node{public children:Node[];public n:int;}function f():Node[]{return makeSlice<Node>(2);}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if diagnostics := checkSource(t, input); len(diagnostics) != 0 {
				t.Fatalf("safe zero construction rejected: %v", diagnostics)
			}
		})
	}
}
