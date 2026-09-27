package sema

import (
	"strings"
	"testing"
)

func TestConversionStatementBoundaries(t *testing.T) {
	for _, expression := range []string{"int(1)", "string(65)", "Score(1)", "clock.Duration(1)", "T(1)"} {
		for _, prefix := range []string{"", "defer ", "go "} {
			t.Run(prefix+expression, func(t *testing.T) {
				source := `import go clock from "time"; type Score=distinct int; constraint Number=~int; function use<T extends Number>():void{` + prefix + expression + `;}`
				got := strings.Join(checkSource(t, source), "\n")
				want := "conversion result must be used"
				if prefix != "" {
					want = "type conversions are not calls"
				}
				if !strings.Contains(got, want) {
					t.Fatalf("want %q, got %s", want, got)
				}
			})
		}
	}
}

func TestExplicitConversionDiscardAndOrdinaryCalls(t *testing.T) {
	source := `import go clock from "time";
type Score=distinct int; constraint Number=~int;
function value():int{return 1;}
function shadow():void{const Score=(value:int):int=>value;Score(1);defer Score(2);go Score(3);}
function use<T extends Number>():void{
 _=int(value());_=string(65);_=Score(1);_=clock.Duration(1);_=T(1);
 value();defer value();go value();
}`
	if got := checkSource(t, source); len(got) != 0 {
		t.Fatal(got)
	}
}

func TestValueOnlyForClauses(t *testing.T) {
	for _, source := range []string{
		`function use():void{for(int(1);true;){break;}}`,
		`function use():void{for(;true;int(1)){break;}}`,
		`function use():void{for(min(1,2);true;){break;}}`,
		`function use():void{for(;true;min(1,2)){break;}}`,
	} {
		if got := strings.Join(checkSource(t, source), "\n"); !strings.Contains(got, "result must be used") {
			t.Fatalf("source %s: %s", source, got)
		}
	}
}
