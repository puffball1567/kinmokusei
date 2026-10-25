package selfupdate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type InstalledVersion struct {
	Version   string
	Bytes     int64  // -1 when a safe, complete size measurement is unavailable
	Status    string // managed, unmanaged, or invalid
	Active    bool
	Protected bool
}

type protectedPath struct {
	path, reason string
	info         os.FileInfo
}

type versionInstallation struct {
	launcher, root string
	protected      []protectedPath
}

func versionInstallationPaths(options Options, goos string) (versionInstallation, error) {
	var installation versionInstallation
	if goos != "linux" && goos != "darwin" {
		return installation, fmt.Errorf("automatic version management is not supported on %s", goos)
	}
	if options.CommandPath == "" {
		options.CommandPath = os.Args[0]
	}
	if options.ExecutablePath == "" {
		var err error
		options.ExecutablePath, err = os.Executable()
		if err != nil {
			return installation, err
		}
	}
	launcher, binary, root, original, err := useInstallationPaths(options)
	if err != nil {
		return installation, err
	}
	installation.launcher, installation.root = launcher, root
	installation.protected = append(installation.protected, protectedPath{launcher, "compiler launcher", original})
	for _, item := range []struct{ path, reason string }{
		{binary, "active compiler"}, {options.ExecutablePath, "running compiler or manager"},
		{filepath.Join(filepath.Dir(launcher), "keika-use"), "switch manager"},
	} {
		if _, err := os.Lstat(item.path); os.IsNotExist(err) && item.reason == "switch manager" {
			continue
		} else if err != nil {
			return installation, err
		}
		physical, err := filepath.EvalSymlinks(item.path)
		if err != nil {
			return installation, err
		}
		physical, err = filepath.Abs(physical)
		if err != nil {
			return installation, err
		}
		info, err := os.Stat(physical)
		if err != nil {
			return installation, err
		}
		installation.protected = append(installation.protected, protectedPath{physical, item.reason, info})
	}
	return installation, nil
}

func (installation versionInstallation) checkRoot() error {
	info, err := os.Lstat(installation.root)
	if err != nil {
		return err
	}
	physical, resolveErr := filepath.EvalSymlinks(installation.root)
	if !info.IsDir() || resolveErr != nil || physical != installation.root || filepath.Dir(physical) == physical {
		return fmt.Errorf("release root must be a canonical, non-root directory")
	}
	return nil
}

func pathWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (installation versionInstallation) protection(directory string, entries []versionFile) string {
	for _, protected := range installation.protected {
		if pathWithin(directory, protected.path) {
			return protected.reason
		}
		for _, entry := range entries {
			if !entry.info.IsDir() && os.SameFile(entry.info, protected.info) {
				return protected.reason
			}
		}
	}
	return ""
}

// ListVersions is local and read-only: it never downloads, adopts a bundle,
// executes a cached binary, creates a helper or writes an update lock.
func ListVersions(ctx context.Context, options Options) ([]InstalledVersion, error) {
	u := updater{goos: runtime.GOOS, goarch: runtime.GOARCH}
	return u.listVersions(ctx, options)
}

func (u updater) listVersions(ctx context.Context, options Options) ([]InstalledVersion, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	installation, err := versionInstallationPaths(options, u.goos)
	if err != nil {
		return nil, err
	}
	if err = installation.checkRoot(); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	children, err := os.ReadDir(installation.root)
	if err != nil {
		return nil, err
	}
	var result []InstalledVersion
	for _, child := range children {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		version := "v" + child.Name()
		if _, err := versionParts(version); err != nil {
			continue
		}
		entry := InstalledVersion{Version: version, Bytes: -1, Status: "invalid"}
		directory := filepath.Join(installation.root, child.Name())
		if child.IsDir() {
			inventory, size, scanErr := versionInventory(ctx, directory)
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if scanErr == nil {
				entry.Bytes = size
				if _, err := os.Lstat(filepath.Join(directory, receiptName)); os.IsNotExist(err) {
					entry.Status = "unmanaged"
				} else if err == nil {
					if _, _, ownedErr := ownedVersionInventory(ctx, directory, version, u.goos, u.goarch); ownedErr == nil {
						entry.Status = "managed"
					}
				}
			}
			entry.Protected = installation.protection(directory, inventory) != ""
			for _, protected := range installation.protected {
				if protected.reason == "active compiler" {
					active := versionInstallation{protected: []protectedPath{protected}}
					entry.Active = active.protection(directory, inventory) != ""
				}
			}
		}
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool {
		return versionNewer(result[i].Version, result[j].Version)
	})
	return result, nil
}
