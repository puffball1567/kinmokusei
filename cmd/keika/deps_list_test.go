package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/product"
	"github.com/puffball1567/kinmokusei/internal/project"
)

func TestDepsListFlags(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{"long help", []string{"--help"}, "Usage: keika deps list [project-directory]"},
		{"short help", []string{"-h"}, "Usage: keika deps list [project-directory]"},
		{"single dash help", []string{"-help"}, "Usage: keika deps list [project-directory]"},
		{"help before missing root", []string{"--help", filepath.Join(t.TempDir(), "missing")}, "Usage: keika deps list [project-directory]"},
		{"unknown flag", []string{"--unknown"}, "flag provided but not defined"},
		{"too many roots", []string{".", "."}, "at most one project directory"},
		{"missing root", []string{filepath.Join(t.TempDir(), "missing")}, "not accessible"},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, stdout, stderr := captureRun(t, append([]string{"deps", "list"}, test.args...)...)
			if status != 2 || stdout != "" || !strings.Contains(stderr, test.want) {
				t.Fatalf("status=%d stdout=%q stderr=%q, want status 2 and %q", status, stdout, stderr, test.want)
			}
			if strings.Contains(test.name, "help") && strings.Contains(stderr, "not accessible") {
				t.Fatalf("help must not resolve a project: %s", stderr)
			}
		})
	}
}

func TestDepsListPreservesLockedProject(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "-project")
	library := filepath.Join(root, "library")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	for path, contents := range map[string]string{
		filepath.Join(root, product.ProjectFileName): `[project]
name = "application"
version = "0.1.0"
go-module = "example.com/application"
go-version = "1.23"
`,
		filepath.Join(library, "go.mod"):     "module example.com/library\n\ngo 1.23\n",
		filepath.Join(library, "library.go"): "package library\nfunc Value() int { return 42 }\n",
	} {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := project.AddDependency(root, "example.com/library", "v0.0.0", "./library", true); err != nil {
		t.Fatal(err)
	}
	snapshots := map[string][]byte{}
	for _, name := range []string{product.ProjectFileName, product.LockFileName} {
		contents, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		snapshots[name] = contents
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Error(err)
		}
	})
	for _, test := range []struct {
		name, cwd string
		args      []string
	}{
		{"explicit root", parent, []string{root}},
		{"flag-like root after terminator", parent, []string{"--", "-project"}},
		{"current directory", root, nil},
		{"current directory after terminator", root, []string{"--"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := os.Chdir(test.cwd); err != nil {
				t.Fatal(err)
			}
			status, stdout, stderr := captureRun(t, append([]string{"deps", "list"}, test.args...)...)
			if status != 0 || stdout != "go\texample.com/library\tv0.0.0\n" || stderr != "" {
				t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout, stderr)
			}
			for name, before := range snapshots {
				after, err := os.ReadFile(filepath.Join(root, name))
				if err != nil || !bytes.Equal(before, after) {
					t.Fatalf("list changed %s: %v", name, err)
				}
			}
		})
	}
}
