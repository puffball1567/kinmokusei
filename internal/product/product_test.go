package product

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestIdentityDerivations(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"project file", ProjectFileName, LanguageID + ".toml"},
		{"lock file", LockFileName, LanguageID + ".lock"},
		{"state directory", StateDirectoryName, "." + LanguageID},
		{"generated module", GeneratedModulePath, LanguageID + ".generated"},
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

func TestIdentityIsSafeForPathsAndModules(t *testing.T) {
	if FullName != "Kinmokusei Programming Language" || DisplayName != "Kinmokusei" || LanguageID != "kinmokusei" || CommandName != "keika" || SourceExtension != ".km" {
		t.Fatalf("unexpected public identity: full=%q display=%q language=%q command=%q extension=%q", FullName, DisplayName, LanguageID, CommandName, SourceExtension)
	}
	if CommandName == "" || CommandName != strings.ToLower(CommandName) {
		t.Fatalf("CommandName must be non-empty lowercase: %q", CommandName)
	}
	if strings.ContainsAny(CommandName, `/\\`) {
		t.Fatalf("CommandName must be one path element: %q", CommandName)
	}
	if !strings.HasPrefix(SourceExtension, ".") || len(SourceExtension) == 1 {
		t.Fatalf("SourceExtension must include a non-empty dot suffix: %q", SourceExtension)
	}
	for _, extension := range []string{SourceExtension, ".otm"} {
		if !IsSourceExtension(extension) {
			t.Errorf("IsSourceExtension(%q) = false", extension)
		}
	}
	for _, extension := range []string{"", ".yn", ".go", ".KM"} {
		if IsSourceExtension(extension) {
			t.Errorf("IsSourceExtension(%q) = true", extension)
		}
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

func TestExplicitVersionTakesPriority(t *testing.T) {
	previous := Version
	Version = "v0.2.0"
	t.Cleanup(func() { Version = previous })
	if got := VersionString(); got != "v0.2.0" {
		t.Fatalf("VersionString() = %q, want v0.2.0", got)
	}
}

func TestResolvedVersionSources(t *testing.T) {
	tests := []struct {
		name     string
		explicit string
		module   string
		want     string
	}{
		{"release build", "v0.2.0", "v0.1.0", "v0.2.0"},
		{"Go module install", "devel", "v0.2.0", "v0.2.0"},
		{"empty explicit module install", "", "v0.2.0", "v0.2.0"},
		{"source build", "devel", "(devel)", "devel"},
		{"missing build information", "", "", "devel"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolvedVersion(test.explicit, test.module); got != test.want {
				t.Fatalf("resolvedVersion(%q, %q) = %q, want %q", test.explicit, test.module, got, test.want)
			}
		})
	}
}

func TestDevelopmentVersionString(t *testing.T) {
	previous := Version
	Version = "devel"
	t.Cleanup(func() { Version = previous })
	if got := VersionString(); got != "devel" {
		t.Fatalf("VersionString() = %q, want devel", got)
	}
}
