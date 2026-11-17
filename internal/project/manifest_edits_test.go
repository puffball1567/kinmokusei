package project

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDependencyCommandsPreserveManifestComments(t *testing.T) {
	t.Setenv("GOPROXY", "off")
	for _, ending := range []string{"\n", "\r\n"} {
		name := "LF"
		if ending == "\r\n" {
			name = "CRLF"
		}
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			root, library := filepath.Join(base, "app"), filepath.Join(base, "library")
			original := strings.ReplaceAll(`# Owner: platform team
  [project] # application settings
name="application" # display name
version = "0.1.0"
go-module = "app.test/main"
go-version = "1.23"

[target]
tags = "zeta, alpha" # keep user ordering

# Source packages
[dependencies] # source pins
# Keep this explanation

[imports] # source aliases

[replace] # development overrides
# End of manifest`, "\n", ending)
			packageFiles(t, root, map[string]string{"kinmokusei.toml": original, "index.km": ""})
			packageFiles(t, library, map[string]string{
				"kinmokusei.toml": packageManifest("pkg.test/library"), "index.km": "", "LICENSE": "MIT fixture",
			})
			assertComments := func() []byte {
				t.Helper()
				contents, err := os.ReadFile(filepath.Join(root, "kinmokusei.toml"))
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range strings.Split(original, ending) {
					if !strings.Contains(string(contents), line) {
						t.Fatalf("dependency edit lost source line %q:\n%s", line, contents)
					}
				}
				if ending == "\r\n" && bytes.Contains(bytes.ReplaceAll(contents, []byte("\r\n"), nil), []byte("\n")) {
					t.Fatal("dependency edit introduced LF into a CRLF manifest")
				}
				if err := CheckDependencies(root); err != nil {
					t.Fatal(err)
				}
				return contents
			}
			if err := AddAutoDependency(root, "pkg.test/library", "v0.1.0", "../library", true); err != nil {
				t.Fatal(err)
			}
			assertComments()
			packageFiles(t, library, map[string]string{"kinmokusei.toml": strings.Replace(packageManifest("pkg.test/library"), `version = "0.1.0"`, `version = "0.2.0"`, 1)})
			if err := UpdateSourcePackages(root, "pkg.test/library", true); err != nil {
				t.Fatal(err)
			}
			assertComments()
			beforeNoop := assertComments()
			if err := UpdateSourcePackages(root, "pkg.test/library", true); err != nil {
				t.Fatal(err)
			}
			if afterNoop := assertComments(); !bytes.Equal(beforeNoop, afterNoop) {
				t.Fatal("unchanged version selection rewrote the manifest")
			}
			if err := RemoveDependency(root, "pkg.test/library", true); err != nil {
				t.Fatal(err)
			}
			before := assertComments()
			beforeLock, err := os.ReadFile(filepath.Join(root, "kinmokusei.lock"))
			if err != nil {
				t.Fatal(err)
			}
			if err := AddDependency(root, "missing.invalid/library", "v0.1.0", "", true); err == nil {
				t.Fatal("offline cache miss unexpectedly succeeded")
			}
			after := assertComments()
			afterLock, err := os.ReadFile(filepath.Join(root, "kinmokusei.lock"))
			if err != nil || !bytes.Equal(before, after) || !bytes.Equal(beforeLock, afterLock) {
				t.Fatalf("failed resolution did not restore exact dependency state: %v", err)
			}
		})
	}
}
