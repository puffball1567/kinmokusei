package sema

import (
	"strings"
	"testing"
)

func TestImportedGoStructEmbedding(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, input, want string }{
		{"promoted selector", `struct Output{bytes.Buffer;} function use(v:Output):int{return v.Len();}`, ""},
		{"promoted pointer method expression", `struct Output{bytes.Buffer;} function use(v:*Output):int{const f=(*Output).Len;return f(v);}`, ""},
		{"value embedding pointer method set", `struct Output{bytes.Buffer;} function use(v:Output):io.Writer{return v;}`, "cannot use Output"},
		{"pointer implements writer", `struct Output{bytes.Buffer;} function use(v:*Output):io.Writer{return v;}`, ""},
		{"pointer embedding value method set", `struct Output{*bytes.Buffer;} function use(v:Output):io.Writer{return v;}`, ""},
		{"temporary with embedded pointer", `struct Output{*bytes.Buffer;} function use(v:*bytes.Buffer):int{return Output{Buffer:v}.Len();}`, ""},
		{"temporary with embedded value", `struct Output{bytes.Buffer;} function use():int{return Output{Buffer:bytes.Buffer{}}.Len();}`, "requires an addressable"},
		{"source overload uses emitted name", `struct Output{bytes.Buffer;public function len():int{return 1;}} function use(v:Output):int{return v.Len();}`, "shadowed or ambiguous in Go"},
		{"conflicting selectors", `struct Left{bytes.Buffer;} struct Right{bytes.Buffer;} struct Output{Left;Right;} function use(v:Output):int{return v.Len();}`, "ambiguous promoted"},
		{"native and Go method names collide", `struct Left{bytes.Buffer;} struct Right{public function len():int{return 1;}} struct Output{Left;Right;} function use(v:Output):int{return v.Len();}`, "shadowed or ambiguous in Go"},
		{"private imported field", `struct Output{bytes.Buffer;} function use(v:Output):byte[]{return v.buf;}`, "has no field"},
		{"pointer to pointer", `struct Output{**bytes.Buffer;}`, "embedded field must name"},
		{"named import alias", `import go {Buffer as B} from "bytes"; struct Output{B;} function use():int{let x=Output{B:bytes.Buffer{}};return x.B.Len()+x.Len();}`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := `import go bytes from "bytes";import go io from "io";` + test.input
			got := strings.Join(checkSource(t, input), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%q want=%q", got, test.want)
			}
		})
	}
}

func TestGoInterfaceEmbedding(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, input, want string }{
		{"promoted call", `struct Output{io.Writer;} function use(v:Output,p:byte[]):(int,error){return v.Write(p);}`, ""},
		{"value implements delegated interface", `struct Output{io.Writer;} function use(v:Output):io.Writer{return v;}`, ""},
		{"value method expression", `struct Output{io.Writer;} function use(v:Output,p:byte[]):(int,error){const f=Output.Write;return f(v,p);}`, ""},
		{"explicit delegation", `struct Output{io.Writer; public function write(p:byte[]):(int,error){return this.Writer.Write(p);}} function use(v:Output):io.Writer{return v;}`, ""},
		{"pointer method shadows value promotion", `struct Output{io.Writer; public pointer function write(p:byte[]):(int,error){return this.Writer.Write(p);}} function use(v:Output):io.Writer{return v;}`, "cannot use Output"},
		{"pointer wrapper overrides delegated method", `struct Output{io.Writer; public pointer function write(p:byte[]):(int,error){return this.Writer.Write(p);}} function use(v:*Output):io.Writer{return v;}`, ""},
		{"nil interface field", `struct Output{io.Writer;} function use():boolean{return Output{Writer:nil}.Writer==nil;}`, ""},
		{"built-in Go error", `struct Failure{error;} function use(e:error):error{return Failure{error:e};}`, ""},
		{"interface alias", `alias Writer=io.Writer; struct Output{Writer;} function use(v:Output):io.Writer{return v;}`, ""},
		{"qualified collision", `struct Output{io.Writer;io.WriteCloser;} function use(v:Output,p:byte[]):(int,error){return v.Writer.Write(p);}`, ""},
		{"ambiguous promoted method", `struct Output{io.Writer;io.WriteCloser;} function use(v:Output,p:byte[]):(int,error){return v.Write(p);}`, "ambiguous promoted"},
		{"pointer to interface", `struct Output{*io.Writer;}`, "cannot embed a pointer to a Go interface"},
		{"constraint interface", `import go cmp from "cmp";struct Output{cmp.Ordered;}`, "must be usable as a runtime value"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, `import go io from "io";`+test.input), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%q want=%q", got, test.want)
			}
		})
	}
}
