package sema

import (
	"strings"
	"testing"
)

func TestNamedCollectionArgumentInference(t *testing.T) {
	for _, input := range []string{
		`type Values=distinct int[];function first<T>(v:T[]):T{return v[0];}function run(v:Values):int{return first(v);}`,
		`type Values<T>=distinct T[];function first<T>(v:Values<T>):T{return v[0];}function run(v:int[]):int{return first(v);}`,
		`type Lookup=distinct Map<string,int>;function read<K extends comparable,V>(v:Map<K,V>,k:K):V{return v[k];}function run(v:Lookup):int{return read(v,"x");}`,
		`type Pair=distinct [2]int;function first<T>(v:[2]T):T{return v[0];}function run(v:Pair):int{return first(v);}`,
		`type Lookup<K extends comparable,V>=distinct Map<K,V>;function read<K extends comparable,V>(v:Lookup<K,V>,k:K):V{return v[k];}function run(v:Map<string,int>):int{return read(v,"x");}`,
		`type Pair<T>=distinct [2]T;function first<T>(v:Pair<T>):T{return v[0];}function run(v:[2]int):int{return first(v);}`,
		`type Ref<T>=distinct *T;function read<T>(v:Ref<T>):T{return *v;}function run(v:*int):int{return read(v);}`,
		`type Channel<T>=distinct GoChannel<T>;function read<T>(v:Channel<T>):T{return <-v;}function run(v:GoChannel<int>):int{return read(v);}`,
	} {
		t.Run(input, func(t *testing.T) {
			if got := checkSource(t, input); len(got) != 0 {
				t.Fatal(got)
			}
		})
	}
}

func TestNamedCollectionInferenceBoundaries(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"nominal identity", `type A<T>=distinct T[];type B<T>=distinct T[];function use<T>(v:A<T>):void{}function run(v:B<int>):void{use(v);}`, "cannot use"},
		{"array length", `type A=distinct [3]int;function use<T>(v:[2]T):void{}function run(v:A):void{use(v);}`, "cannot use"},
		{"element conflict", `type A=distinct int[];function use<T>(v:T[],x:T):void{}function run(v:A):void{use(v,"wrong");}`, "cannot use"},
		{"channel direction", `type A=distinct GoSendChannel<int>;function use<T>(v:GoReceiveChannel<T>):void{}function run(v:A):void{use(v);}`, "cannot use"},
		{"nullable elements", `alias Maybe=int[]|null;type A=distinct Maybe[];function use<T>(v:T[]):T{return v[0];}function run(v:A):int[]{return use(v);}`, "cannot use"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, tc.input), "\n")
			if !strings.Contains(got, tc.want) {
				t.Fatalf("want %q, got %s", tc.want, got)
			}
		})
	}
}
