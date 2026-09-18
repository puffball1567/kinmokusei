package lsp

import (
	"sort"

	"github.com/puffball1567/kinmokusei/internal/compiler"
	"github.com/puffball1567/kinmokusei/internal/project"
)

// Navigating into an external source must not demand a second lock in that
// library/cache checkout. Use an open consumer's graph for its dependency files.
func (s *Server) checkDocument(doc document, overlay map[string]string) (compiler.Result, error) {
	root, found, err := project.FindRoot(doc.Path)
	if err == nil && found {
		manifest, readErr := project.ReadManifest(root)
		if readErr == nil && manifest.Package.Entry != "" {
			var candidates []string
			for _, open := range s.documents {
				other, exists, findErr := project.FindRoot(open.Path)
				if findErr == nil && exists && other != root {
					candidates = append(candidates, other)
				}
			}
			sort.Strings(candidates)
			for _, candidate := range candidates {
				consumer, lock, lockErr := project.ValidateLockedFiles(candidate)
				if lockErr != nil {
					continue
				}
				graph, graphErr := project.ReadPackageGraph(consumer, lock)
				if graphErr != nil {
					continue
				}
				if graph.SourceIdentity(doc.Path) != "" {
					return compiler.CheckFilesWithOverlayInProject([]string{doc.Path}, overlay, candidate)
				}
			}
		}
	}
	return compiler.CheckFilesWithOverlay([]string{doc.Path}, overlay)
}
