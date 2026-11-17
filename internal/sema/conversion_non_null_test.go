package sema

import (
	"strings"
	"testing"
)

func TestCheckedReferenceConversionsAndProofs(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"function snapshot":         `alias F=()=>int;function f(raw:F):int{const checked:F|null=F(raw);return checked();}`,
		"defined function":          `type F=distinct ()=>int;function f(raw:F):int{const checked:F|null=F(raw);return checked();}`,
		"generic function":          `constraint Fn=()=>int;function f<T extends Fn>(raw:T):T{const checked:T|null=T(raw);return checked;}`,
		"forced assertion snapshot": `alias F=()=>int;function f(raw:interface{}):int{const checked:F|null=raw as! F;return checked();}`,
		"converted interface":       `alias View=interface{Error():bstring;};function f(raw:error):bstring{const checked:View|null=View(raw);return checked.Error();}`,
		"conversion skips await":    `alias F=()=>int;function work():F{return ()=>9;}function f(raw:F):F{const task=go work();const chosen=F(raw)??await task;detach task;return chosen;}`,
		"assertion skips await":     `alias F=()=>int;function work():F{return ()=>9;}function f(raw:interface{}):F{const task=go work();const chosen=(raw as! F)??await task;detach task;return chosen;}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := checkSource(t, input); len(got) != 0 {
				t.Fatalf("rejected: %v", got)
			}
		})
	}
}

func TestReferenceConversionProofBoundaries(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct{ input, want string }{
		"raw conversion is not proof":     {`import go http from "net/http";function f(raw:http.HandlerFunc):void{const checked:http.HandlerFunc|null=http.HandlerFunc(raw);checked(nil,nil);}`, "nullable"},
		"raw interface is not proof":      {`function f(raw:error):bstring{const checked:error|null=error(raw);return checked.Error();}`, "nullable"},
		"nullable operand":                {`alias F=()=>int;function f(raw:F|null):F{return F(raw);}`, "cannot convert"},
		"nil remains rejected":            {`alias F=()=>int;function f():F{return F(nil);}`, "cannot convert nil"},
		"reassignment drops proof":        {`alias F=()=>int;function f(raw:F):int{let checked:F|null=F(raw);checked=null;return checked();}`, "nullable"},
		"raw conversion conditional task": {`import go http from "net/http";function work():http.HandlerFunc{return nil;}function f(raw:http.HandlerFunc):http.HandlerFunc{const task=go work();return http.HandlerFunc(raw)??await task;}`, "Task"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := strings.Join(checkSource(t, test.input), "\n")
			if !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%q want=%q", got, test.want)
			}
		})
	}
}
