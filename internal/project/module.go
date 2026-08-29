package project

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

func RenderGoMod(manifest Manifest, moduleDirectory string) ([]byte, error) {
	return renderGoMod(manifest, nil, moduleDirectory)
}

func RenderResolvedGoMod(manifest Manifest, requirements map[string]string, moduleDirectory string) ([]byte, error) {
	return renderGoMod(manifest, requirements, moduleDirectory)
}

func renderGoMod(manifest Manifest, requirements map[string]string, moduleDirectory string) ([]byte, error) {
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	var result bytes.Buffer
	fmt.Fprintf(&result, "module %s\n\ngo %s\n", manifest.Project.GoModule, manifest.Project.GoVersion)
	paths := make([]string, 0, len(manifest.Dependencies))
	for path := range manifest.Dependencies {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	if len(paths) != 0 {
		result.WriteString("\nrequire (\n")
		for _, path := range paths {
			fmt.Fprintf(&result, "\t%s %s\n", path, manifest.Dependencies[path])
		}
		result.WriteString(")\n")
	}
	indirectPaths := make([]string, 0, len(requirements))
	for path := range requirements {
		if _, direct := manifest.Dependencies[path]; !direct {
			indirectPaths = append(indirectPaths, path)
		}
	}
	sort.Strings(indirectPaths)
	if len(indirectPaths) != 0 {
		result.WriteString("\nrequire (\n")
		for _, path := range indirectPaths {
			fmt.Fprintf(&result, "\t%s %s // indirect\n", path, requirements[path])
		}
		result.WriteString(")\n")
	}
	for _, path := range paths {
		replacement, exists := manifest.Replacements[path]
		if !exists {
			continue
		}
		target := filepath.Join(manifest.Root, filepath.FromSlash(replacement))
		relative, err := filepath.Rel(moduleDirectory, target)
		if err != nil {
			return nil, fmt.Errorf("cannot make replacement %q relative to generated module: %w", path, err)
		}
		relative = filepath.ToSlash(filepath.Clean(relative))
		if relative != "." && !strings.HasPrefix(relative, ".") {
			relative = "./" + relative
		}
		fmt.Fprintf(&result, "\nreplace %s => %s\n", path, relative)
	}
	return result.Bytes(), nil
}
