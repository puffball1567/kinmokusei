// Package product contains the public identity of the language and toolchain.
// User-visible names stay centralized here so compiler behavior does not depend
// on duplicated branding strings.
package product

import (
	"path/filepath"
	"runtime/debug"
)

const (
	// FullName is the formal project and language name.
	FullName = "Kinmokusei Programming Language"
	// DisplayName is the human-readable language name.
	DisplayName = "Kinmokusei"
	// LanguageID is the stable lowercase identifier used by editors, generated
	// modules, and language-owned namespaces.
	LanguageID = "kinmokusei"
	// CommandName is the executable name.
	CommandName = "keika"

	SourceExtension     = ".km"
	ProjectFileName     = LanguageID + ".toml"
	LockFileName        = LanguageID + ".lock"
	StateDirectoryName  = "." + LanguageID
	GeneratedModulePath = LanguageID + ".generated"
	DefaultExecutable   = CommandName + ".out"
)

// LegacySourceExtensions lists source suffixes accepted for migration from
// released versions. New files and generated examples always use .km.
var LegacySourceExtensions = []string{".otm"}

// IsSourceExtension reports whether extension is the current source suffix or
// a supported migration suffix.
func IsSourceExtension(extension string) bool {
	if extension == SourceExtension {
		return true
	}
	for _, legacy := range LegacySourceExtensions {
		if extension == legacy {
			return true
		}
	}
	return false
}

// Version is replaced by release builds through -ldflags. Source builds and
// untagged development binaries report "devel" unless Go module build
// information contains a released module version.
var Version = "devel"

// VersionString returns the release version embedded in the binary or, for a
// binary installed from a tagged Go module, the version recorded by the Go
// toolchain.
func VersionString() string {
	moduleVersion := ""
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		moduleVersion = info.Main.Version
	}
	return resolvedVersion(Version, moduleVersion)
}

func resolvedVersion(explicit, module string) string {
	if explicit != "" && explicit != "devel" {
		return explicit
	}
	if module != "" && module != "(devel)" {
		return module
	}
	return "devel"
}

func GeneratedDirectory(projectRoot string) string {
	return filepath.Join(projectRoot, StateDirectoryName, "gen")
}

func DependencyDirectory(projectRoot string) string {
	return filepath.Join(projectRoot, StateDirectoryName, "deps")
}
