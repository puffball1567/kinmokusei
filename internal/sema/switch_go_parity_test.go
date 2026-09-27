package sema

import (
	goast "go/ast"
	goparser "go/parser"
	gotoken "go/token"
	gotypes "go/types"
	"strings"
	"testing"
)

func TestSwitchDuplicateCasesMatchGoChecker(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, kmType, goType string
		cases                []string
	}{
		{"narrow rounding", "float32", "float32", []string{"16777216.0", "16777217.0"}},
		{"wide rounding", "float", "float64", []string{"9007199254740992.0", "9007199254740993.0"}},
		{"interface rounding", "reflect.Value", "any", []string{"9007199254740992.0", "9007199254740993.0"}},
		{"interface fractional duplicate", "reflect.Value", "any", []string{"0.1", "0.1"}},
		{"interface numeric kinds", "reflect.Value", "any", []string{"1", "1.0", "int64(1)", "N(1)"}},
		{"interface identical aliases", "reflect.Value", "any", []string{"byte(1)", "uint8(1)"}},
		{"interface named identity", "reflect.Value", "any", []string{"N(1)", "N(1)"}},
		{"strings", "string", "string", []string{`"a"+"b"`, `"ab"`}},
		{"boolean", "boolean", "bool", []string{"true", "true"}},
		{"complex", "complex128", "complex128", []string{"1i", "1i"}},
		{"generic same literal", "T", "T", []string{"1", "1"}},
		{"generic literal kinds", "T", "T", []string{"1", "1.0"}},
		{"generic runtime conversion", "T", "T", []string{"T(1)", "T(1)"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			kmTag := "v"
			if test.goType == "any" {
				kmTag = "v.Interface()"
			}
			km := `import go reflect from "reflect";type N=distinct int;function f(v:` + test.kmType + `):void{switch(` + kmTag + `){`
			goSource := `package sample;type N int;func f(v ` + test.goType + `){switch v{`
			if test.goType == "T" {
				km = `constraint C=~int|~int64;function f<T extends C>(v:T):void{switch(v){`
				goSource = `package sample;func f[T interface{~int|~int64}](v T){switch v{`
			}
			for _, expression := range test.cases {
				km += "case " + expression + "{}"
				goSource += "case " + expression + ":;"
			}
			km += "}}"
			goSource += "}}"
			set := gotoken.NewFileSet()
			file, err := goparser.ParseFile(set, "sample.go", goSource, 0)
			if err != nil {
				t.Fatal(err)
			}
			_, goErr := (&gotypes.Config{GoVersion: "go1.23"}).Check("switch.test", set, []*goast.File{file}, nil)
			if goErr != nil && !strings.Contains(goErr.Error(), "duplicate case") {
				t.Fatalf("unexpected Go error: %v", goErr)
			}
			diagnostics := strings.Join(checkSource(t, km), "\n")
			if (goErr == nil) != (diagnostics == "") || goErr != nil && !strings.Contains(diagnostics, "duplicate value switch case") {
				t.Fatalf("Go=%v Kinmokusei=%s", goErr, diagnostics)
			}
		})
	}
}
