package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModuleScopeRejectsUnimportedAndTransitiveNames(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		entry string
		want  string
	}{
		{
			"unimported dependency declaration",
			map[string]string{
				"dependency.otm": `function visible(): int { return 1; } function hidden(): int { return 2; }`,
				"entry.otm":      `import { visible } from "./dependency"; function value(): int { return hidden(); }`,
			},
			"entry.otm", `undefined function "hidden"`,
		},
		{
			"transitive import",
			map[string]string{
				"base.otm":   `function base(): int { return 1; }`,
				"middle.otm": `import { base } from "./base"; function middle(): int { return base(); }`,
				"entry.otm":  `import { middle } from "./middle"; function value(): int { return base(); }`,
			},
			"entry.otm", `undefined function "base"`,
		},
		{
			"root modules do not share names",
			map[string]string{
				"first.otm":  `function first(): int { return 1; }`,
				"second.otm": `function second(): int { return first(); }`,
			},
			"second.otm", `undefined function "first"`,
		},
		{
			"unimported global value",
			map[string]string{
				"dependency.otm": `const secret: int = 1; function visible(): int { return secret; }`,
				"entry.otm":      `import { visible } from "./dependency"; function value(): int { return secret; }`,
			},
			"entry.otm", `undefined name "secret"`,
		},
		{
			"unimported class type",
			map[string]string{
				"dependency.otm": `class Hidden {} function visible(): int { return 1; }`,
				"entry.otm":      `import { visible } from "./dependency"; function value(): Hidden { return new Hidden(); }`,
			},
			"entry.otm", `unknown type "Hidden"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			temp := t.TempDir()
			for name, source := range test.files {
				if err := os.WriteFile(filepath.Join(temp, name), []byte(source), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			paths := []string{filepath.Join(temp, test.entry)}
			if test.name == "root modules do not share names" {
				paths = []string{filepath.Join(temp, "first.otm"), filepath.Join(temp, "second.otm")}
			}
			result, err := CheckFiles(paths)
			if err != nil {
				t.Fatal(err)
			}
			messages := make([]string, len(result.Diagnostics))
			for i, item := range result.Diagnostics {
				messages[i] = item.Message
			}
			if !strings.Contains(strings.Join(messages, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", messages, test.want)
			}
		})
	}
}

func TestModuleScopeRejectsBindingCollisions(t *testing.T) {
	tests := []struct {
		name  string
		entry string
		want  string
	}{
		{"relative import and declaration", `import { value } from "./dependency"; function value(): int { return 2; }`, "conflicts with a declaration"},
		{"Go alias and declaration", `import go strings from "strings"; function strings(): int { return 2; }`, "Go package alias"},
		{"relative name and Go alias", `import go value from "strings"; import { value } from "./dependency";`, "duplicate import binding"},
		{"duplicate Go alias", `import go text from "strings"; import go text from "strconv";`, "duplicate import binding"},
		{"Go alias and built-in type", `import go int from "strings";`, "conflicts with a built-in type"},
		{"Go alias and Result type", `import go Result from "strings";`, "conflicts with a built-in type"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			temp := t.TempDir()
			if err := os.WriteFile(filepath.Join(temp, "dependency.otm"), []byte(`function value(): int { return 1; }`), 0o644); err != nil {
				t.Fatal(err)
			}
			entry := filepath.Join(temp, "entry.otm")
			if err := os.WriteFile(entry, []byte(test.entry), 0o644); err != nil {
				t.Fatal(err)
			}
			result, err := CheckFiles([]string{entry})
			if err != nil {
				t.Fatal(err)
			}
			messages := make([]string, len(result.Diagnostics))
			for i, item := range result.Diagnostics {
				messages[i] = item.Message
			}
			if !strings.Contains(strings.Join(messages, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", messages, test.want)
			}
		})
	}
}

func TestDuplicateDependencyNamesAreLinkedAndCompile(t *testing.T) {
	temp := t.TempDir()
	files := map[string]string{
		"left.otm":  `class Hidden { public function value(): int { return 20; } } function left(): int { return new Hidden().value(); }`,
		"right.otm": `class Hidden { public function value(): int { return 22; } } function right(): int { return new Hidden().value(); }`,
		"entry.otm": `import { left } from "./left"; import { right } from "./right"; function answer(): int { return left() + right(); }`,
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(temp, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(temp, "entry.otm")}, "linked")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	if strings.Count(string(generated), "type _ontama_") < 2 {
		t.Fatalf("duplicate dependency names were not linked:\n%s", generated)
	}
	referenceSource := `package reference
type leftHidden struct{}
func (leftHidden) Value() int { return 20 }
func Left() int { return leftHidden{}.Value() }
type rightHidden struct{}
func (rightHidden) Value() int { return 22 }
func Right() int { return rightHidden{}.Value() }
func Answer() int { return Left() + Right() }
`
	testSource := `package linked
import (
  "testing"
  reference "linked.test/reference"
)
func TestAnswer(t *testing.T) {
  if got, want := answer(), reference.Answer(); got != want { t.Errorf("answer = %d, Go = %d", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "linked.test", generated, referenceSource, testSource)
}

func TestDependencyLinkNamesAreIndependentOfProjectLocation(t *testing.T) {
	generate := func(root string) string {
		t.Helper()
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		files := map[string]string{
			"left.otm":  `function hidden(): int { return 1; } function left(): int { return hidden(); }`,
			"right.otm": `function hidden(): int { return 2; } function right(): int { return hidden(); }`,
			"entry.otm": `import { left } from "./left"; import { right } from "./right"; function value(): int { return left() + right(); }`,
		}
		for name, source := range files {
			if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.otm")}, "stable")
		if err != nil || len(diagnostics) != 0 {
			t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
		}
		return string(generated)
	}
	temp := t.TempDir()
	first := generate(filepath.Join(temp, "first"))
	second := generate(filepath.Join(temp, "second"))
	if first != second {
		t.Fatalf("link names depend on project location:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}
