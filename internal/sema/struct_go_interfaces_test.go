package sema

import (
	"strings"
	"testing"
)

func TestNativeStructGoInterfaces(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"value receiver", `struct Reader{public function read(p:byte[]):(int,error){return 0,nil;}} function use(v:Reader):io.Reader{return v;}`, ""},
		{"pointer receiver", `struct Reader{public pointer function read(p:byte[]):(int,error){return 0,nil;}} function use(v:*Reader):io.Reader{return v;}`, ""},
		{"pointer not inferred on assignment", `struct Reader{public pointer function read(p:byte[]):(int,error){return 0,nil;}} function use(v:Reader):io.Reader{return v;}`, "cannot use Reader"},
		{"pointer embedding", `struct Reader{public pointer function read(p:byte[]):(int,error){return 0,nil;}} struct Wrapper{*Reader;} function use(v:Wrapper):io.Reader{return v;}`, ""},
		{"value embedding needs outer pointer", `struct Reader{public pointer function read(p:byte[]):(int,error){return 0,nil;}} struct Wrapper{Reader;} function use(v:Wrapper):io.Reader{return v;}`, "cannot use Wrapper"},
		{"outer pointer promotes pointer method", `struct Reader{public pointer function read(p:byte[]):(int,error){return 0,nil;}} struct Wrapper{Reader;} function use(v:*Wrapper):io.Reader{return v;}`, ""},
		{"shadowed method", `struct Reader{public function read(p:byte[]):(int,error){return 0,nil;}} struct Wrapper{Reader;public read:int;} function use(v:Wrapper):io.Reader{return v;}`, "cannot use Wrapper"},
		{"ambiguous promoted method", `struct Reader{public function read(p:byte[]):(int,error){return 0,nil;}} struct Other{public function read(p:byte[]):(int,error){return 0,nil;}} struct Wrapper{Reader;Other;} function use(v:Wrapper):io.Reader{return v;}`, "cannot use Wrapper"},
		{"private method", `struct Reader{private function Read(p:byte[]):(int,error){return 0,nil;}} function use(v:Reader):io.Reader{return v;}`, "cannot use Reader"},
		{"generic method is not in Go set", `struct Reader{public function read<T>(p:T):(int,error){return 0,nil;}} function use(v:Reader):io.Reader{return v;}`, "cannot use Reader"},
		{"Go conversion", `struct Reader{public function read(p:byte[]):(int,error){return 0,nil;}} function use(v:Reader):io.Reader{return io.Reader(v);}`, ""},
		{"multi-result forwarding", `struct Reader{public function read(p:byte[]):(int,error){return 0,nil;}} function make():(Reader,int){return Reader{},1;} function use():(io.Reader,int){return make();}`, ""},
		{"assertion", `struct Reader{public function read(p:byte[]):(int,error){return 0,nil;}} function use(v:io.Reader):Reader{return v as! Reader;}`, ""},
		{"impossible assertion", `struct Reader{public pointer function read(p:byte[]):(int,error){return 0,nil;}} function use(v:io.Reader):Reader{return v as! Reader;}`, "cannot contain asserted type"},
		{"Go error interface", `struct Failure{public function error():bstring{return b"bad";}} function use():error{return Failure{};}`, ""},
		{"Go error interface via alias conversion", `alias FailureType=error;struct Failure{public function error():bstring{return b"bad";}} function use():error{return FailureType(Failure{});}`, ""},
		{"preserve verified signature", `struct S{public function string():string{return "safe";}} function use(v:S):fmt.Stringer{return v;}`, "cannot use S"},
		{"preserve verified signature on conversion", `struct S{public function string():string{return "safe";}} function use(v:S):fmt.Stringer{return fmt.Stringer(v);}`, "cannot convert S"},
		{"preserve shared text storage", `struct S{public data:string[];public function string():bstring{return b"raw";}} function use(v:S):fmt.Stringer{return v;}`, "verified UTF-8"},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := `import go io from "io";import go fmt from "fmt";` + test.source
			got := strings.Join(checkSource(t, input), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%q want=%q", got, test.want)
			}
		})
	}
}
