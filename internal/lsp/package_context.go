package lsp

import (
	"github.com/puffball1567/kinmokusei/internal/compiler"
	"github.com/puffball1567/kinmokusei/internal/project"
)

// Navigating into an external source must not demand a second lock in that
// library/cache checkout. Use a workspace or open consumer's locked graph.
func (s *Server) checkDocument(doc document, overlay map[string]string) (compiler.Result, error) {
	root, found, err := project.FindRoot(doc.Path)
	if err == nil && found {
		manifest, readErr := project.ReadManifest(root)
		if readErr == nil && manifest.Package.Entry != "" {
			candidates := map[string]bool{}
			addCandidate := func(path string) {
				other, exists, findErr := project.FindRoot(path)
				if findErr == nil && exists && canonicalWorkspacePath(other) != canonicalWorkspacePath(root) {
					candidates[canonicalWorkspacePath(other)] = true
				}
			}
			for _, folder := range s.workspaceRoots {
				addCandidate(folder)
			}
			for _, open := range s.documents {
				addCandidate(open.Path)
			}
			for _, candidate := range sortedWorkspaceRoots(candidates) {
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
