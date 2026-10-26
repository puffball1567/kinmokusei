package sema

import (
	"strings"
	"testing"
)

func TestNullableTypeParameterRequiresNilRepresentation(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"guarded fallback":     `function orElse<T>(x:T|null,d:T):T{if(x===null){return d;}return x;}`,
		"unused generic":       `function wrap<T>(x:T|null):int{return 1;}`,
		"scalar instantiation": `function wrap<T>(x:T|null):int{return 1;}const value=wrap<int>(null);`,
		"field initializer":    `class Box<T>{public value:T|null=null;}`,
		"alias":                `alias Maybe<T>=T|null;`,
		"callback":             `function use<T>(read:()=>T|null):void{}`,
		"comparable":           `function use<T extends comparable>(value:T|null):void{}`,
		"mixed bound":          `constraint Mixed=~*int|~int;function use<T extends Mixed>(value:T|null):void{}`,
		"method bound":         `interface Readable{function read():int;}function use<T extends Readable>(value:T|null):void{}`,
	} {
		t.Run(name, func(t *testing.T) {
			diagnostics := checkSource(t, input)
			if !strings.Contains(strings.Join(diagnostics, "\n"), "type T cannot be nullable because it has no nil representation") {
				t.Fatalf("missing declaration diagnostic: %v", diagnostics)
			}
		})
	}
}

func TestNullableTypeParameterWithNilCapableBound(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"pointer":         `constraint Pointer=~*int;function orElse<T extends Pointer>(x:T|null,d:T):T{if(x===null){return d;}return x;}`,
		"slice":           `constraint Slice=~int[];function orElse<T extends Slice>(x:T|null,d:T):T{if(x===null){return d;}return x;}`,
		"map":             `constraint Dict=~Map<int,int>;function use<T extends Dict>(value:T|null):boolean{return value===null;}`,
		"channel":         `constraint Channel=~GoChannel<int>;function use<T extends Channel>(value:T|null):boolean{return value===null;}`,
		"function":        `constraint Callback=()=>int;function use<T extends Callback>(value:T|null):boolean{return value===null;}`,
		"mixed nil bound": `constraint Reference=~*int|~int[];function use<T extends Reference>(value:T|null):boolean{return value===null;}`,
		"generic pointee": `function use<T>(value:*T|null):boolean{return value===null;}`,
		"generic slice":   `function use<T>(value:T[]|null):boolean{return value===null;}`,
	} {
		t.Run(name, func(t *testing.T) {
			if diagnostics := checkSource(t, input); len(diagnostics) != 0 {
				t.Fatalf("valid nullable type rejected: %v", diagnostics)
			}
		})
	}
}
