package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/attribution"
	"github.com/puffball1567/kinmokusei/internal/project"
)

func TestBuildAttributionAndSafeRebuild(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "main.km")
	if err := os.WriteFile(source, []byte(`import go fmt from "fmt"; function main(): void { fmt.Println("hello"); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "program")
	if runtime.GOOS == "windows" {
		output += ".exe"
	}
	for i := 0; i < 2; i++ {
		status, stdout, stderr := captureRun(t, "build", "-o", output, source)
		if status != 0 || stdout != "" || stderr != "" {
			t.Fatalf("build %d: %d %s %s", i, status, stdout, stderr)
		}
		if err := verifyManagedNotices(output + "-licenses"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(output + ".licenses"); !os.IsNotExist(err) {
		t.Fatal("legacy notice directory generated", err)
	}
	indexData, err := os.ReadFile(filepath.Join(output+"-licenses", "INDEX.json"))
	if err != nil {
		t.Fatal(err)
	}
	var index attribution.Index
	if err := json.Unmarshal(indexData, &index); err != nil {
		t.Fatal(err)
	}
	if index.GOOS != runtime.GOOS || index.GOARCH != runtime.GOARCH || !strings.HasPrefix(index.Toolchain, "go1.") {
		t.Fatalf("wrong build identity: %+v", index)
	}
	binary, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(binary)
	if index.ExecutableSHA256 != hex.EncodeToString(digest[:]) {
		t.Fatal("index not bound to executable")
	}
	if !strings.Contains(string(indexData), `"fmt"`) || strings.Contains(string(indexData), root) {
		t.Fatal("missing dependency or local path leaked")
	}
	license, err := os.ReadFile(filepath.Join(output+"-licenses", "go", "LICENSE"))
	if err != nil || !strings.Contains(string(license), "The Go Authors") || !strings.Contains(string(license), "Redistributions in binary form") {
		t.Fatal("complete Go license not emitted", err)
	}
	// Do not discard manually added notices during an ordinary rebuild.
	custom := filepath.Join(output+"-licenses", "CUSTOM.txt")
	if err := os.WriteFile(custom, []byte("user notice\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	status, _, stderr := captureRun(t, "build", "-o", output, source)
	if status == 0 || !strings.Contains(stderr, "modified attribution") {
		t.Fatalf("modified directory replaced: %d %s", status, stderr)
	}
	after, _ := os.ReadFile(output)
	if string(after) != string(binary) {
		t.Fatal("failed build replaced executable")
	}
	if data, err := os.ReadFile(custom); err != nil || string(data) != "user notice\n" {
		t.Fatal("user notice lost", err)
	}
}

func TestUnattributedDependencyBuildPreservesPreviousOutput(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "library")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, contents := range map[string]string{
		"go.mod":           "module application.test/main\n\ngo 1.23\nrequire dependency.test/library v0.0.0\nreplace dependency.test/library => ./library\n",
		"library/go.mod":   "module dependency.test/library\n\ngo 1.23\n",
		"library/value.go": "package library\nfunc Value() int { return 42 }\n",
		"main.km":          `import go library from "dependency.test/library"; function main(): void { library.Value(); }`,
	} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	output := filepath.Join(root, "program")
	if err := os.WriteFile(output, []byte("previous executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	status, _, stderr := captureRun(t, "build", "-o", output, filepath.Join(root, "main.km"))
	if status == 0 || !strings.Contains(stderr, "license text not found for Go module dependency.test/library") {
		t.Fatalf("missing license: %d %s", status, stderr)
	}
	data, _ := os.ReadFile(output)
	if string(data) != "previous executable" {
		t.Fatal("previous output changed")
	}
	if _, err := os.Stat(output + "-licenses"); !os.IsNotExist(err) {
		t.Fatal("partial attribution published", err)
	}
}

func TestPublicationRollsBackBothArtifacts(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "program")
	old := attribution.Bundle{Index: attribution.Index{Format: "kinmokusei-attribution-v1"}, Files: map[string][]byte{"go/LICENSE": []byte("Old license\n")}}
	if err := old.Write(output + "-licenses"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("old executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(output+"-licenses", "INDEX.json"))
	stage := filepath.Join(root, "stage")
	if err := os.Mkdir(stage, 0o755); err != nil {
		t.Fatal(err)
	}
	stagedNotices := filepath.Join(stage, "licenses")
	if err := old.Write(stagedNotices); err != nil {
		t.Fatal(err)
	}
	if err := publishBuild(stage, filepath.Join(stage, "missing-executable"), stagedNotices, output); err == nil {
		t.Fatal("missing executable publication succeeded")
	}
	after, _ := os.ReadFile(filepath.Join(output+"-licenses", "INDEX.json"))
	if string(before) != string(after) {
		t.Fatal("previous notices not restored")
	}
	binary, _ := os.ReadFile(output)
	if string(binary) != "old executable" {
		t.Fatal("previous executable not restored")
	}
}

func TestPublicationUsesHyphenatedNoticesAndPreservesLegacyDirectory(t *testing.T) {
	for _, name := range []string{"app", "app.exe", "my app", "app.v2"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			output := filepath.Join(root, name)
			legacy := output + ".licenses"
			if err := os.Mkdir(legacy, 0o755); err != nil {
				t.Fatal(err)
			}
			legacyNotice := filepath.Join(legacy, "CUSTOM.txt")
			if err := os.WriteFile(legacyNotice, []byte("legacy user notice\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			stage := filepath.Join(root, "stage")
			if err := os.Mkdir(stage, 0o755); err != nil {
				t.Fatal(err)
			}
			stagedNotices := filepath.Join(stage, "licenses")
			bundle := attribution.Bundle{Index: attribution.Index{Format: "kinmokusei-attribution-v1"}, Files: map[string][]byte{"go/LICENSE": []byte("New license\n")}}
			if err := bundle.Write(stagedNotices); err != nil {
				t.Fatal(err)
			}
			stagedOutput := filepath.Join(stage, "executable")
			if err := os.WriteFile(stagedOutput, []byte("new executable"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := publishBuild(stage, stagedOutput, stagedNotices, output); err != nil {
				t.Fatal(err)
			}
			if err := verifyManagedNotices(output + "-licenses"); err != nil {
				t.Fatal(err)
			}
			if data, err := os.ReadFile(output); err != nil || string(data) != "new executable" {
				t.Fatal("executable not published", err)
			}
			if data, err := os.ReadFile(legacyNotice); err != nil || string(data) != "legacy user notice\n" {
				t.Fatal("legacy user notice changed", err)
			}
		})
	}
}

func TestManagedNoticeVerificationRejectsUserChanges(t *testing.T) {
	for _, scenario := range []string{"empty-directory", "edited-license", "edited-notices", "missing-license", "symlink-index", "index-traversal", "oversize-index"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			bundle := attribution.Bundle{Index: attribution.Index{Format: "kinmokusei-attribution-v1"}, Files: map[string][]byte{"go/LICENSE": []byte("Original license\n")}}
			if err := bundle.Write(root); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "empty-directory":
				if err := os.Mkdir(filepath.Join(root, "personal-notices"), 0o755); err != nil {
					t.Fatal(err)
				}
			case "edited-license":
				if err := os.WriteFile(filepath.Join(root, "go", "LICENSE"), []byte("User modification\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "edited-notices":
				if err := os.WriteFile(filepath.Join(root, "THIRD_PARTY_NOTICES.md"), []byte("User notice\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "missing-license":
				if err := os.Remove(filepath.Join(root, "go", "LICENSE")); err != nil {
					t.Fatal(err)
				}
			case "symlink-index":
				other := filepath.Join(t.TempDir(), "index.json")
				if err := os.Rename(filepath.Join(root, "INDEX.json"), other); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(other, filepath.Join(root, "INDEX.json")); err != nil {
					t.Skip(err)
				}
			case "index-traversal":
				if err := os.WriteFile(filepath.Join(root, "INDEX.json"), []byte(`{"format":"kinmokusei-attribution-v1","files":[{"path":"../outside","sha256":"`+strings.Repeat("0", 64)+`"}]}`), 0o644); err != nil {
					t.Fatal(err)
				}
			case "oversize-index":
				file, err := os.OpenFile(filepath.Join(root, "INDEX.json"), os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				if err := file.Truncate((16 << 20) + 1); err != nil {
					t.Fatal(err)
				}
				file.Close()
			}
			if err := verifyManagedNotices(root); err == nil {
				t.Fatal("modified notices treated as disposable")
			}
		})
	}
}

func TestBuildAttributionIncludesOfflineReplacedDependency(t *testing.T) {
	root := t.TempDir()
	for name, contents := range map[string]string{
		"kinmokusei.toml":      "[project]\nname=\"app\"\nversion=\"0.1.0\"\ngo-module=\"application.test/main\"\ngo-version=\"1.23\"\n[go.dependencies]\n\"dependency.test/library\"=\"v0.0.0\"\n[go.replacements]\n\"dependency.test/library\"=\"library\"\n",
		"main.km":              `import go library from "dependency.test/library/pkg"; function main(): void { library.Value(); }`,
		"library/go.mod":       "module dependency.test/library\n\ngo 1.23\n",
		"library/LICENSE":      "Complete fixture license, conditions and disclaimer\n",
		"library/pkg/NOTICE":   "Additional nested attribution\n",
		"library/pkg/value.go": "package pkg\nfunc Value() int { return 42 }\n",
	} {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GOPROXY", "off")
	if _, err := project.LockDependencies(root, true); err != nil {
		t.Fatal(err)
	}
	manifest, _ := os.ReadFile(filepath.Join(root, "kinmokusei.toml"))
	lock, _ := os.ReadFile(filepath.Join(root, "kinmokusei.lock"))
	output := filepath.Join(root, "program")
	status, _, stderr := captureRun(t, "build", "-o", output, filepath.Join(root, "main.km"))
	if status != 0 {
		t.Fatalf("offline build: %d %s", status, stderr)
	}
	data, err := os.ReadFile(filepath.Join(output+"-licenses", "INDEX.json"))
	if err != nil {
		t.Fatal(err)
	}
	var index attribution.Index
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, component := range index.Components {
		if component.Path != "dependency.test/library" {
			continue
		}
		found = true
		for name, expected := range map[string]string{"LICENSE": "Complete fixture license, conditions and disclaimer\n", "pkg/NOTICE": "Additional nested attribution\n"} {
			actual, err := os.ReadFile(filepath.Join(output+"-licenses", filepath.FromSlash(component.Directory), filepath.FromSlash(name)))
			if err != nil || string(actual) != expected {
				t.Fatal("original dependency notice missing", err)
			}
		}
	}
	if !found {
		t.Fatal("dependency identity not inventoried")
	}
	manifestAfter, _ := os.ReadFile(filepath.Join(root, "kinmokusei.toml"))
	lockAfter, _ := os.ReadFile(filepath.Join(root, "kinmokusei.lock"))
	if !reflect.DeepEqual(manifest, manifestAfter) || !reflect.DeepEqual(lock, lockAfter) {
		t.Fatal("build changed dependency metadata")
	}
}
