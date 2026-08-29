// Package product contains the public identity of the language and toolchain.
// User-visible names stay centralized here so compiler behavior does not depend
// on duplicated branding strings.
package product

import "path/filepath"

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

func GeneratedDirectory(projectRoot string) string {
	return filepath.Join(projectRoot, StateDirectoryName, "gen")
}

func DependencyDirectory(projectRoot string) string {
	return filepath.Join(projectRoot, StateDirectoryName, "deps")
}
