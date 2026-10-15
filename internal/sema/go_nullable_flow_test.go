package sema

import (
	"strings"
	"testing"
)

const goNullableFlowPrelude = `
import go flag from "flag";
import go errors from "errors";
alias MaybeFlag = *flag.Flag | null;
alias Action = () => void;
class Holder { constructor(public value: MaybeFlag) {} }
`

func TestGoReferencesRetainExplicitNullableFlow(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"pointer":               `const value: *flag.Flag | null = flag.Lookup("missing"); if (value === null) { return true; } return value.Name !== "";`,
		"alias":                 `const value: MaybeFlag = flag.Lookup("missing"); if (null === value) { return true; } return value.Name !== "";`,
		"inferred intermediate": `const raw = flag.Lookup("missing"); const value: MaybeFlag = raw; if (value === null) { return true; } return value.Name !== "";`,
		"typed intermediate":    `const raw: *flag.Flag = flag.Lookup("missing"); const value: MaybeFlag = raw; if (value === null) { return true; } return value.Name !== "";`,
		"reassignment":          `let value: MaybeFlag = null; value = flag.Lookup("missing"); if (value !== null) { return value.Name !== ""; } return true;`,
		"interface":             `const value: error | null = errors.Unwrap(errors.New("plain")); if (value === null) { return true; } return value.Error() !== "";`,
		"Go field":              `const outer = flag.NewFlagSet("test", flag.ContinueOnError); const value: Action | null = outer.Usage; if (value === null) { return true; } value(); return false;`,
		"member assignment":     `const holder = new Holder(null); holder.value = flag.Lookup("missing"); if (holder.value === null) { return true; } return holder.value.Name !== "";`,
		"branch join":           `let value: MaybeFlag = null; if (flag.Lookup("x") === nil) { value = flag.Lookup("a"); } else { value = flag.Lookup("b"); } if (value === null) { return true; } return value.Name !== "";`,
		"loop":                  `let value: MaybeFlag = flag.Lookup("x"); while (value !== null) { const name = value.Name; value = flag.Lookup("missing"); if (name === "stop") { break; } } if (value === null) { return true; } return value.Name !== "";`,
		"checked copy":          `const value: MaybeFlag = flag.Lookup("x"); if (value === null) { return true; } const copy: MaybeFlag = value; return copy.Name !== "";`,
		"checked member copy":   `const holder = new Holder(flag.Lookup("x")); if (holder.value === null) { return true; } const copy: MaybeFlag = holder.value; return copy.Name !== "";`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if diagnostics := checkSource(t, goNullableFlowPrelude+`function inspect(): boolean {`+body+`}`); len(diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", diagnostics)
			}
		})
	}
}

func TestGoReferenceAssignmentDoesNotProveNonNull(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"pointer":                     `const value: MaybeFlag = flag.Lookup("missing"); return value.Name !== "";`,
		"intermediate":                `const raw = flag.Lookup("missing"); const value: MaybeFlag = raw; return value.Name !== "";`,
		"reassignment":                `let value: MaybeFlag = null; value = flag.Lookup("missing"); return value.Name !== "";`,
		"interface":                   `const value: error | null = errors.Unwrap(errors.New("plain")); return value.Error() !== "";`,
		"Go field":                    `const outer = flag.NewFlagSet("test", flag.ContinueOnError); const value: Action | null = outer.Usage; value(); return false;`,
		"typed function intermediate": `const outer = flag.NewFlagSet("test", flag.ContinueOnError); const raw: Action = outer.Usage; const value: Action | null = raw; value(); return false;`,
		"member assignment":           `const holder = new Holder(null); holder.value = flag.Lookup("missing"); return holder.value.Name !== "";`,
		"guard then overwrite":        `let value: MaybeFlag = flag.Lookup("x"); if (value === null) { return false; } value = flag.Lookup("missing"); return value.Name !== "";`,
		"member guard then overwrite": `const holder = new Holder(flag.Lookup("x")); if (holder.value === null) { return false; } holder.value = flag.Lookup("missing"); return holder.value.Name !== "";`,
		"branch join":                 `let value: MaybeFlag = null; if (flag.Lookup("x") === nil) { value = flag.Lookup("a"); } else { value = flag.Lookup("b"); } return value.Name !== "";`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			diagnostics := checkSource(t, goNullableFlowPrelude+`function inspect(): boolean {`+body+`}`)
			if joined := strings.Join(diagnostics, "\n"); !strings.Contains(joined, "must be checked against null") {
				t.Fatalf("expected nullable access diagnostic, got %v", diagnostics)
			}
		})
	}
}

func TestFreshValuesStillProveNonNull(t *testing.T) {
	t.Parallel()
	input := `
alias Action = () => int;
function answer(): int { return 42; }
class Holder { constructor(public pointer: *int | null) {} }
function inspect(): int {
  let seed = 7;
  const pointer: *int | null = &seed;
  const literal: int[] | null = [1];
  const slice: int[] | null = makeSlice<int>(1);
  const lookup: Map<string, int> | null = makeMap<string, int>();
  const channel: GoChannel<int> | null = goChannel<int>(1);
  const arrow: Action | null = (): int => 3;
  const named: Action | null = answer;
  const holder = new Holder(null);
  holder.pointer = &seed;
  const copy: *int | null = holder.pointer;
  channel <- 4;
  lookup["x"] = 5;
  return *pointer + literal[0] + slice[0] + lookup["x"] + <-channel + arrow() + named() + *copy;
}
`
	if diagnostics := checkSource(t, input); len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
}

func TestAssignmentNonNullProofUsesStorageNotTypeSpelling(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"slice":            `const raw: int[] = nil; const value: int[] | null = raw; if (value === null) { return true; } return value[0] > 0;`,
		"map":              `const raw: Map<string, int> = nil; const value: Map<string, int> | null = raw; if (value === null) { return true; } return value["x"] > 0;`,
		"channel":          `const raw: GoChannel<int> = nil; const value: GoChannel<int> | null = raw; if (value === null) { return true; } return <-value > 0;`,
		"defined pointer":  `const raw: Link = nil; const value: Link | null = raw; if (value === null) { return true; } return *value > 0;`,
		"defined function": `const raw: Callback = nil; const value: Callback | null = raw; if (value === null) { return true; } return value() > 0;`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			input := `type Link = distinct *int; type Callback = distinct () => int; function inspect(): boolean {` + body + `}`
			if diagnostics := checkSource(t, input); len(diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", diagnostics)
			}
		})
	}
}
