package attribution

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLicenseDirectoriesAndSelectedRootAliases(t *testing.T) {
	toolchain := fixtureToolchain(t)
	writeFixture(t, toolchain.GOROOT, "src/runtime/runtime.go", "package runtime\n")
	alias := filepath.Join(t.TempDir(), "toolchain")
	if err := os.Symlink(toolchain.GOROOT, alias); err != nil {
		t.Skip(err)
	}
	toolchain.GOROOT = alias
	module := t.TempDir()
	writeFixture(t, module, "LICENSES/BSD-3-Clause.txt", "Full BSD fixture\n")
	writeFixture(t, module, "LICENSES/MIT.txt", "Full MIT fixture\n")
	writeFixture(t, module, "pkg/value.go", "package pkg\n")
	bundle, err := Collect(toolchain, []Package{
		{Standard: true, ImportPath: "runtime", Dir: filepath.Join(alias, "src/runtime"), GoFiles: []string{"runtime.go"}},
		{ImportPath: "module.test/pkg", Dir: filepath.Join(module, "pkg"), GoFiles: []string{"value.go"}, Module: &Module{Path: "module.test", Version: "v1.0.0", Dir: module}},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	prefix := componentDirectory("modules", "go-module:module.test@v1.0.0")
	for _, name := range []string{"BSD-3-Clause.txt", "MIT.txt"} {
		if len(bundle.Files[prefix+"/LICENSES/"+name]) == 0 {
			t.Fatal("SPDX-style license directory not collected")
		}
	}
}

func TestSourcePackageMetadataIsNotLicenseText(t *testing.T) {
	toolchain := fixtureToolchain(t)
	root := t.TempDir()
	writeFixture(t, root, "kinmokusei.toml", "[package]\nlicense = \"MIT\"\n")
	_, err := Collect(toolchain, nil, []SourcePackage{{Path: "package.test/one", Version: "v1.0.0", Dir: root}}, nil)
	if err == nil || !strings.Contains(err.Error(), "manifest license metadata is not a license text") {
		t.Fatal("source-package license omitted", err)
	}
}

func TestEmbeddedRuntimeLicenseMatchesRepository(t *testing.T) {
	original, err := os.ReadFile(filepath.Join("..", "..", "LICENSE"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(original)) != strings.TrimSpace(string(kinmokuseiLicense)) {
		t.Fatal("embedded runtime license is stale")
	}
}
