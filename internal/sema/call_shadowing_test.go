package sema

import (
	"strings"
	"testing"
)

func TestLocalCallableShadowsTopLevelFunction(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ source, want string }{
		{`function read<T>(value:T):T{return value;}function use():int{const read=():int=>2;return read();}`, ""},
		{`function read(value:int):int{return value;}function use(read:()=>string):string{return read();}`, ""},
		{`function read():int{return 1;}function use():int{const read=2;return read();}`, "not callable"},
		{`function read(value:int):int{return value;}function use():int{{const read=():string=>"x";const text=read();}return read(2);}`, ""},
	} {
		got := strings.Join(checkSource(t, test.source), "\n")
		if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
			t.Fatalf("source=%s diagnostics=%s", test.source, got)
		}
	}
}
