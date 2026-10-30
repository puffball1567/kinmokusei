package project

import (
	"fmt"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// Project/compiler metadata uses full SemVer without v, including optional
// prerelease and build identifiers. IsValid also accepts short Go spellings,
// so enforce all three core components before delegating the actual grammar.
func validProjectVersion(version string) bool {
	core, _, _ := strings.Cut(version, "-")
	core, _, _ = strings.Cut(core, "+")
	return strings.Count(core, ".") == 2 && semver.IsValid("v"+version)
}

// Dependencies are immutable Go module identities, not queries that Go may
// rewrite. Preserve +incompatible, but reject other build metadata and short
// versions rather than silently changing the requested version/cache key.
func validateDependencyVersion(path, version string) error {
	if version == "" || module.CanonicalVersion(version) != version {
		return fmt.Errorf("dependency version %q is not a complete canonical Go module version; use vMAJOR.MINOR.PATCH with an optional prerelease (only +incompatible build metadata is preserved)", version)
	}
	if err := module.Check(path, version); err != nil {
		return fmt.Errorf("invalid dependency %s@%s: %w", path, version, err)
	}
	return nil
}

// Compare complete versions (and Go's major.minor spelling), with prereleases
// ordered before the corresponding release. Numeric components never overflow.
func comparePackageVersions(a, b string) int {
	return semver.Compare("v"+strings.TrimPrefix(a, "v"), "v"+strings.TrimPrefix(b, "v"))
}
