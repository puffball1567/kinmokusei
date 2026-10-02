package attribution

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, root, name, contents string) string {
	t.Helper()
	file := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return file
}

func fixtureToolchain(t *testing.T) Toolchain {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, "LICENSE", "Go license full conditions and disclaimer\n")
	writeFixture(t, root, "PATENTS", "Go patent grant\n")
	return Toolchain{GOROOT: root, GOVERSION: "go1.23.5", GOOS: "windows", GOARCH: "arm64", CGO_ENABLED: "0"}
}

func TestCollectOriginalNoticesAndSourceSupplements(t *testing.T) {
	toolchain := fixtureToolchain(t)
	writeFixture(t, toolchain.GOROOT, "src/vendor/example/LICENSE", "Nested license\n")
	writeFixture(t, toolchain.GOROOT, "src/vendor/example/pkg/NOTICE.txt", "Extra notice\n")
	ordinary := "// Copyright 2024 The Go Authors. All rights reserved.\n// Use of this source code is governed by a BSD-style\n// license that can be found in the LICENSE file.\npackage pkg\n"
	writeFixture(t, toolchain.GOROOT, "src/vendor/example/pkg/ordinary.go", ordinary)
	special := ordinary + "// Copyright another author\n// full permission text\n// full disclaimer\n"
	writeFixture(t, toolchain.GOROOT, "src/vendor/example/pkg/special.go", special)
	writeFixture(t, toolchain.GOROOT, "src/vendor/example/pkg/arch.s", "// Copyright assembly author\n// full license and disclaimer\n")
	packages := []Package{{Standard: true, ImportPath: "vendor/example/pkg", Dir: filepath.Join(toolchain.GOROOT, "src/vendor/example/pkg"), GoFiles: []string{"ordinary.go", "special.go"}, SFiles: []string{"arch.s"}}}
	bundle, err := Collect(toolchain, packages, nil, []string{"custom"})
	if err != nil {
		t.Fatal(err)
	}
	if string(bundle.Files["go/src/vendor/example/pkg/special.go.txt"]) != special {
		t.Fatal("supplement not copied in full")
	}
	if _, ok := bundle.Files["go/src/vendor/example/pkg/ordinary.go.txt"]; ok {
		t.Fatal("ordinary Go header duplicated")
	}
	for _, name := range []string{"go/LICENSE", "go/PATENTS", "go/src/vendor/example/LICENSE", "go/src/vendor/example/pkg/NOTICE.txt", "go/src/vendor/example/pkg/arch.s.txt"} {
		if len(bundle.Files[name]) == 0 {
			t.Fatalf("missing %s", name)
		}
	}
	output := t.TempDir()
	if err := bundle.Write(output); err != nil {
		t.Fatal(err)
	}
	index, err := os.ReadFile(filepath.Join(output, "INDEX.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(index), toolchain.GOROOT) {
		t.Fatal("private root disclosed")
	}
	var parsed Index
	if err := json.Unmarshal(index, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Toolchain != "go1.23.5" || parsed.GOOS != "windows" || parsed.GOARCH != "arm64" || !reflect.DeepEqual(parsed.BuildTags, []string{"custom"}) {
		t.Fatalf("wrong target: %+v", parsed)
	}
	second := t.TempDir()
	if err := bundle.Write(second); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(filepath.Join(second, "INDEX.json"))
	if string(index) != string(again) {
		t.Fatal("nondeterministic index")
	}
}

func TestDependencyIdentityReplacementsAndSourcePackages(t *testing.T) {
	toolchain := fixtureToolchain(t)
	moduleRoot, sourceRoot := t.TempDir(), t.TempDir()
	writeFixture(t, moduleRoot, "LICENSE", "Module license\n")
	writeFixture(t, moduleRoot, "nested/NOTICE", "Nested notice\n")
	writeFixture(t, moduleRoot, "pkg/foreign.c", "/* Copyright C author; full disclaimer */\n")
	writeFixture(t, moduleRoot, "pkg/header.h", "/* Copyright header author; full permission */\n")
	writeFixture(t, sourceRoot, "LICENSE", "Source license\n")
	writeFixture(t, sourceRoot, "index.km", "// Copyright source author\nexport const value: int = 42;\n")
	pkg := Package{ImportPath: "example.test/library/pkg", Dir: filepath.Join(moduleRoot, "pkg"), CFiles: []string{"foreign.c"}, HFiles: []string{"header.h"}, Module: &Module{Path: "example.test/library", Version: "v1.2.3", Replace: &Module{Path: moduleRoot, Dir: moduleRoot}}}
	source := SourcePackage{Path: "example.test/library", Version: "v1.2.3", Dir: sourceRoot}
	bundle, err := Collect(toolchain, []Package{pkg}, []SourcePackage{source}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Index.Components) != 3 {
		t.Fatalf("components: %+v", bundle.Index.Components)
	}
	if bundle.Index.Components[0].Directory == bundle.Index.Components[1].Directory {
		t.Fatal("source/Go identity collision")
	}
	for _, component := range bundle.Index.Components {
		if len(bundle.Files[component.Directory+"/LICENSE"]) == 0 {
			t.Fatal("missing component license")
		}
		if strings.Contains(component.Replacement, moduleRoot) {
			t.Fatal("local replacement path disclosed")
		}
	}
	if len(bundle.Files[componentDirectory("modules", "go-module:example.test/library@v1.2.3")+"/pkg/foreign.c.txt"]) == 0 {
		t.Fatal("C attribution missing")
	}
}

func TestCollectorRejectsMissingEmptyUnsafeAndOpaqueSources(t *testing.T) {
	for _, scenario := range []string{"missing-license", "notice-only", "empty-license", "symlink-license", "symlink-parent", "escape", "opaque", "oversize", "unknown-owner"} {
		t.Run(scenario, func(t *testing.T) {
			toolchain := fixtureToolchain(t)
			root := t.TempDir()
			writeFixture(t, root, "pkg/plain.go", "package pkg\n")
			pkg := Package{ImportPath: "example.test/pkg", Dir: filepath.Join(root, "pkg"), GoFiles: []string{"plain.go"}, Module: &Module{Path: "example.test", Version: "v1.0.0", Dir: root}}
			if scenario != "missing-license" && scenario != "notice-only" {
				writeFixture(t, root, "LICENSE", "License\n")
			}
			switch scenario {
			case "notice-only":
				writeFixture(t, root, "NOTICE", "notice is not a license\n")
			case "empty-license":
				writeFixture(t, root, "LICENSE", " \n")
			case "symlink-license":
				if err := os.Remove(filepath.Join(root, "LICENSE")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(toolchain.GOROOT, "LICENSE"), filepath.Join(root, "LICENSE")); err != nil {
					t.Skip(err)
				}
			case "symlink-parent":
				directory := t.TempDir()
				writeFixture(t, directory, "data.go", "// Copyright outsider\n")
				if err := os.Symlink(directory, filepath.Join(root, "linked")); err != nil {
					t.Skip(err)
				}
				pkg.GoFiles = []string{"../linked/data.go"}
			case "escape":
				pkg.GoFiles = []string{"../../outside.go"}
			case "opaque":
				pkg.SysoFiles = []string{"binary.syso"}
			case "oversize":
				file, err := os.OpenFile(filepath.Join(root, "LICENSE"), os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				if err := file.Truncate(maxFileSize + 1); err != nil {
					t.Fatal(err)
				}
				file.Close()
			case "unknown-owner":
				pkg.Module = nil
			}
			if _, err := Collect(toolchain, []Package{pkg}, nil, nil); err == nil {
				t.Fatal("unsafe or unattributed dependency accepted")
			}
		})
	}
}
