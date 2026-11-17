package project

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDependencyVersionsAcceptHyphenatedPrereleases(t *testing.T) {
	for _, version := range []string{"v1.0.0-rc-1", "v1.0.0-alpha.1-2", "v1.0.0--", "v0.0.0-20260101000000-abcdefabcdef"} {
		t.Run(version, func(t *testing.T) {
			path, got, err := ParseDependencyArgument("example.com/library@" + version)
			if err != nil || path != "example.com/library" || got != version {
				t.Fatalf("path=%s version=%s err=%v", path, got, err)
			}
			input := strings.Replace(validManifest(), `"example.com/zeta" = "v1.2.3"`, `"example.com/zeta" = "`+version+`"`, 1)
			if _, err := ParseManifest(filepath.Join(t.TempDir(), "kinmokusei.toml"), []byte(input)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestProjectVersionsUseCompleteSemver(t *testing.T) {
	for _, version := range []string{"1.0.0-rc-1", "1.0.0+build-1", "1.0.0-rc-1+build-1", "1.0.0-0.a-1+001"} {
		input := strings.Replace(validManifest(), `version = "0.1.0"`, `version = "`+version+`"`, 1)
		if _, err := ParseManifest(filepath.Join(t.TempDir(), "kinmokusei.toml"), []byte(input)); err != nil {
			t.Fatalf("version=%s err=%v", version, err)
		}
	}
	for _, version := range []string{"1.0", "v1.0.0", "01.0.0", "1.02.0", "1.0.03", "1.0.0-01", "1.0.0-alpha..1", "1.0.0-", "1.0.0+", "1.0.0+meta..1"} {
		input := strings.Replace(validManifest(), `version = "0.1.0"`, `version = "`+version+`"`, 1)
		if _, err := ParseManifest(filepath.Join(t.TempDir(), "kinmokusei.toml"), []byte(input)); err == nil {
			t.Fatalf("accepted invalid version=%s", version)
		}
	}
}

func TestPinnedVersionsRejectRewritesAndMajorMismatches(t *testing.T) {
	for _, test := range []struct{ path, version string }{
		{"example.com/lib", "v1"}, {"example.com/lib", "v1.2"}, {"example.com/lib", "v01.2.3"},
		{"example.com/lib", "v1.2.3-01"}, {"example.com/lib", "v1.2.3-rc..1"},
		{"example.com/lib", "v1.2.3+build-1"}, {"example.com/lib", "v1.2.3-rc-1+build-1"},
		{"example.com/lib", "v2.0.0"}, {"example.com/lib/v2", "v1.0.0"}, {"example.com/lib/v1", "v1.0.0"},
	} {
		if _, _, err := ParseDependencyArgument(test.path + "@" + test.version); err == nil {
			t.Fatalf("accepted %s@%s", test.path, test.version)
		}
		if err := validatePackageRequest(test.path, test.version); err == nil {
			t.Fatalf("request accepted %s@%s", test.path, test.version)
		}
		for _, section := range []string{"go.dependencies", "dependencies"} {
			input := packageManifest("app.test/main") + "[" + section + "]\n\"" + test.path + "\"=\"" + test.version + "\"\n"
			if _, err := ParseManifest(filepath.Join(t.TempDir(), "kinmokusei.toml"), []byte(input)); err == nil {
				t.Fatalf("%s accepted %s@%s", section, test.path, test.version)
			}
		}
	}
	for _, test := range []struct{ path, version string }{
		{"example.com/lib/v2", "v2.0.0-rc-1"}, {"example.com/lib", "v2.0.0+incompatible"}, {"gopkg.in/yaml.v2", "v2.1.0-rc-1"},
	} {
		if _, _, err := ParseDependencyArgument(test.path + "@" + test.version); err != nil {
			t.Fatal(err)
		}
	}
}
