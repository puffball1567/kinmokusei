package project

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifestSourceEdits(t *testing.T) {
	t.Parallel()
	dependency := "\t" + `"example.com/\u0061lpha"` + "\t=\t" + `"v0.0.\x30"` + " \t# pinned = reviewed"
	replacement := "  " + `"example.com/alpha"  = "./library#=\"quoted\"" # development path`
	input := "# manifest notes\n" + strings.ReplaceAll(validManifest(), `"example.com/alpha" = "v0.0.0"`, dependency)
	input = strings.ReplaceAll(input, `"example.com/alpha" = "./library"`, replacement)
	for _, ending := range []string{"\n", "\r\n"} {
		for _, terminalNewline := range []bool{false, true} {
			base := strings.TrimSuffix(strings.ReplaceAll(input, "\n", ending), ending)
			if terminalNewline {
				base += ending
			}
			for _, test := range []struct {
				name string
				edit func(*Manifest)
				want string
			}{
				{"unchanged", func(*Manifest) {}, base},
				{"version", func(m *Manifest) { m.Dependencies["example.com/alpha"] = "v0.2.0" }, strings.Replace(base, `"v0.0.\x30"`, `"v0.2.0"`, 1)},
				{"remove with comments", func(m *Manifest) {
					delete(m.Dependencies, "example.com/alpha")
					delete(m.Replacements, "example.com/alpha")
				}, strings.Replace(strings.Replace(base, dependency, "\t# pinned = reviewed", 1), replacement, "  # development path", 1)},
				{"insert sorted new keys", func(m *Manifest) {
					m.Dependencies["example.com/beta"] = "v0.1.0"
					m.Dependencies["example.com/added"] = "v0.1.0"
				}, strings.Replace(base, dependency, dependency+ending+`"example.com/added" = "v0.1.0"`+ending+`"example.com/beta" = "v0.1.0"`, 1)},
			} {
				t.Run(fmt.Sprintf("%s/%q/final=%t", test.name, ending, terminalNewline), func(t *testing.T) {
					manifest, err := ParseManifest(filepath.Join(t.TempDir(), "kinmokusei.toml"), []byte(base))
					if err != nil {
						t.Fatal(err)
					}
					test.edit(&manifest)
					got, err := renderManifestChanges(manifest)
					if err != nil || string(got) != test.want {
						t.Fatalf("err=%v\ngot:\n%s\nwant:\n%s", err, got, test.want)
					}
					if string(manifest.Contents) != base {
						t.Fatal("renderer changed the caller's original contents")
					}
				})
			}
		}
	}
}

func TestManifestSourceEditsCreateTables(t *testing.T) {
	t.Parallel()
	for _, suffix := range []string{"", "\n", "\r\n", "\r\n\r\n"} {
		input := "# keep table order\r\n" + strings.ReplaceAll(strings.Split(validManifest(), "\n[go.dependencies]")[0], "\n", "\r\n")
		input = strings.TrimRight(input, "\r\n") + suffix
		manifest, err := ParseManifest(filepath.Join(t.TempDir(), "kinmokusei.toml"), []byte(input))
		if err != nil {
			t.Fatal(err)
		}
		manifest.Packages["pkg.test/library"] = "v0.1.0"
		manifest.Imports["lib"] = "pkg.test/library"
		manifest.PackageReplacements["pkg.test/library"] = "../library#path"
		got, err := renderManifestChanges(manifest)
		if err != nil {
			t.Fatal(err)
		}
		prefix := input
		if !strings.HasSuffix(prefix, "\n") {
			prefix += "\r\n"
		}
		if !strings.HasSuffix(prefix, "\r\n\r\n") {
			prefix += "\r\n"
		}
		want := prefix + "[dependencies]\r\n\"pkg.test/library\" = \"v0.1.0\"\r\n\r\n[imports]\r\n\"lib\" = \"pkg.test/library\"\r\n\r\n[replace]\r\n\"pkg.test/library\" = \"../library#path\"\r\n"
		if string(got) != want {
			t.Fatalf("suffix=%q\ngot=%q\nwant=%q", suffix, got, want)
		}
	}
}

func TestManifestSourceEditsPreserveUnchangedSections(t *testing.T) {
	t.Parallel()
	input := "# package notes\n" + packageManifest("app.test/main") + `[exports] # public source modules
"sub" = "sub.km" # keep export

[target] # target notes
tags = "zeta, alpha"
cgo = "auto"

[go.interop]
unsafe = "deny" # intentionally explicit

[go.dependencies]
"example.com/alpha" = "v0.1.0" # direct pin
`
	manifest, err := ParseManifest(filepath.Join(t.TempDir(), "kinmokusei.toml"), []byte(input))
	if err != nil {
		t.Fatal(err)
	}
	manifest.Dependencies["example.com/alpha"] = "v0.2.0"
	got, err := renderManifestChanges(manifest)
	want := strings.Replace(input, `"example.com/alpha" = "v0.1.0"`, `"example.com/alpha" = "v0.2.0"`, 1)
	if err != nil || string(got) != want {
		t.Fatalf("err=%v\ngot=%s\nwant=%s", err, got, want)
	}
}
