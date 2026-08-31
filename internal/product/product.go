// Package product contains the public identity of the language and toolchain.
// User-visible names stay centralized here so compiler behavior does not depend
// on duplicated branding strings.
package product

import (
	"path/filepath"
	"runtime/debug"
)

const (
	// DisplayName is the human-readable language name.
	DisplayName = "OnsenTamago"
	// CommandName is the lowercase identifier used by the CLI and project files.
	CommandName = "ontama"

	SourceExtension     = ".otm"
	ProjectFileName     = CommandName + ".toml"
	LockFileName        = CommandName + ".lock"
	StateDirectoryName  = "." + CommandName
	GeneratedModulePath = CommandName + ".generated"
	DefaultExecutable   = CommandName + ".out"
)

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
