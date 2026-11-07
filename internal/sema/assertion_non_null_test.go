package sema

import (
	"strings"
	"testing"
)

func TestSourceFunctionAssertionContracts(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct{ input, want string }{
		"checked function":                 {`alias F=()=>int;function f(v:interface{}):int{const [callback,ok]=v as? F;if(!ok){return 0;}return callback();}`, ""},
		"forced function":                  {`alias F=()=>int;function f(v:interface{}):F{return v as! F;}`, ""},
		"defined function":                 {`type F=distinct ()=>int;function f(v:interface{}):F{return v as! F;}`, ""},
		"generic function":                 {`constraint F=()=>int;function f<T extends F>(v:interface{}):T{return v as! T;}`, ""},
		"raw Go constraint":                {`import go http from "net/http";constraint Mixed=http.HandlerFunc|int;function f<T extends Mixed>(v:interface{}):T{return v as! T;}`, ""},
		"nullable function":                {`alias F=()=>int;function f(v:interface{}):boolean{const [callback,ok]=v as? F|null;if(!ok||callback===null){return false;}return callback()>0;}`, ""},
		"unchecked payload":                {`alias F=()=>int;function f(v:interface{}):int{const [callback,ok]=v as? F;return callback();}`, "may be absent"},
		"blank success":                    {`alias F=()=>int;function f(v:interface{}):int{const [callback,_]=v as? F;return callback();}`, "may be absent"},
		"replaced success":                 {`alias F=()=>int;function f(v:interface{}):int{let [callback,ok]=v as? F;ok=true;if(ok){return callback();}return 0;}`, "may be absent"},
		"mixed target constraint":          {`type F=distinct ()=>int;constraint Mixed=F|int;function f<T extends Mixed>(v:interface{}):T{return v as! T;}`, "constraint mixing nil-capable and value types"},
		"mixed target checked constraint":  {`type F=distinct ()=>int;constraint Mixed=F|int;function f<T extends Mixed>(v:interface{}):boolean{const [callback,ok]=v as? T;return ok;}`, "constraint mixing nil-capable and value types"},
		"mixed nullable constraint":        {`alias F=()=>int;alias M=F|null;constraint Mixed=M|int;function f<T extends Mixed>(v:interface{}):T{return v as! T;}`, ""},
		"verified result remains rejected": {`alias F=()=>string;function f(v:interface{}):F{return v as! F;}`, "cannot establish a UTF-8"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := strings.Join(checkSource(t, test.input), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
