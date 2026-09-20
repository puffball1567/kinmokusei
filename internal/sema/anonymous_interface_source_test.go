package sema

import (
	"strings"
	"testing"
)

func TestAnonymousInterfaceSourceType(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"method", `function read(value: interface { read(offset:int):string; }):string{return value.read(0);}`, ""},
		{"multiple methods", `function use(value: interface { read():string; close():void; }):string{value.close();return value.read();}`, ""},
		{"generic method parameter", `function use(value: interface { read(value:int):string; }):string{return value.read(1);}`, ""},
		{"terminator optional", `function bad(value: interface { read():string }):string{return "";}`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.source), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
