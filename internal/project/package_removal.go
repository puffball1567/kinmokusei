package project

// Use the last matching graph to distinguish now-unreachable overrides from
// a removed direct dependency that is still required transitively. This needs
// no source/cache reads for packages that the user is trying to remove.
// If no matching graph exists, keep the historical direct-only removal path.
func pruneRemovedPackageReplacements(manifest *Manifest) bool {
	lock, err := ReadLock(manifest.Root)
	if err != nil || lock.ManifestHash != Hash(manifest.Contents) {
		return false
	}
	locked := map[string]LockedPackage{}
	for _, pkg := range lock.Packages {
		locked[pkg.Path] = pkg
	}
	reachable := map[string]bool{}
	var visit func(string, string) bool
	visit = func(module, version string) bool {
		pkg, exists := locked[module]
		if !exists || pkg.Version != version {
			return false
		}
		if reachable[module] {
			return true
		}
		reachable[module] = true
		for child, version := range pkg.Dependencies {
			if !visit(child, version) {
				return false
			}
		}
		return true
	}
	for module, version := range manifest.Packages {
		if !visit(module, version) {
			return false
		}
	}
	for module := range manifest.PackageReplacements {
		if _, known := locked[module]; known && !reachable[module] {
			delete(manifest.PackageReplacements, module)
		}
	}
	return true
}
