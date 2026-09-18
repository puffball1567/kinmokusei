package main

import (
	"path/filepath"

	"github.com/puffball1567/kinmokusei/internal/project"
)

func projectSources(arguments []string) ([]string, error) {
	if len(arguments) != 0 {
		return arguments, nil
	}
	root, found, err := project.FindRoot(".")
	if err != nil || !found {
		return nil, err
	}
	manifest, err := project.ReadManifest(root)
	if err != nil {
		return nil, err
	}
	entry := "main.km"
	if manifest.Package.Entry != "" {
		entry = manifest.Package.Entry
	}
	return []string{filepath.Join(root, filepath.FromSlash(entry))}, nil
}
