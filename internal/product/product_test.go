package product

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProvisionalIdentityDerivations(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"project file", ProjectFileName, CommandName + ".toml"},
		{"lock file", LockFileName, CommandName + ".lock"},
		{"state directory", StateDirectoryName, "." + CommandName},
		{"generated module", GeneratedModulePath, CommandName + ".generated"},
		{"default executable", DefaultExecutable, CommandName + ".out"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("got %q, want %q", test.got, test.want)
			}
		})
	}
}

func TestProvisionalIdentityIsSafeForPathsAndModules(t *testing.T) {
	if CommandName == "" || CommandName != strings.ToLower(CommandName) {
		t.Fatalf("CommandName must be non-empty lowercase: %q", CommandName)
	}
	if strings.ContainsAny(CommandName, `/\\`) {
		t.Fatalf("CommandName must be one path element: %q", CommandName)
	}
	if !strings.HasPrefix(SourceExtension, ".") || len(SourceExtension) == 1 {
		t.Fatalf("SourceExtension must include a non-empty dot suffix: %q", SourceExtension)
	}
	want := filepath.Join("project", StateDirectoryName, "gen")
	if got := GeneratedDirectory("project"); got != want {
		t.Fatalf("GeneratedDirectory = %q, want %q", got, want)
	}
	wantDependencies := filepath.Join("project", StateDirectoryName, "deps")
	if got := DependencyDirectory("project"); got != wantDependencies {
		t.Fatalf("DependencyDirectory = %q, want %q", got, wantDependencies)
	}
}
