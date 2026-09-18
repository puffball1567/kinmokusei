package lsp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
)

type workspaceFolder struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

func (s *Server) initializeWorkspace(raw json.RawMessage) error {
	var params struct {
		RootURI  string            `json:"rootUri"`
		RootPath string            `json:"rootPath"`
		Folders  []workspaceFolder `json:"workspaceFolders"`
	}
	if len(raw) != 0 {
		if err := json.Unmarshal(raw, &params); err != nil {
			return fmt.Errorf("invalid initialize parameters: %w", err)
		}
	}
	roots := map[string]bool{}
	if params.Folders != nil {
		for _, folder := range params.Folders {
			if path := workspaceFolderPath(folder.URI); path != "" {
				roots[path] = true
			}
		}
	} else if params.RootURI != "" {
		if path := workspaceFolderPath(params.RootURI); path != "" {
			roots[path] = true
		}
	} else if filepath.IsAbs(params.RootPath) {
		roots[canonicalWorkspacePath(params.RootPath)] = true
	}
	s.workspaceRoots = sortedWorkspaceRoots(roots)
	return nil
}

func workspaceFolderPath(uri string) string {
	path, err := filePath(uri)
	if err != nil || !filepath.IsAbs(path) {
		return ""
	}
	return canonicalWorkspacePath(path)
}

func canonicalWorkspacePath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return filepath.Clean(path)
}

func sortedWorkspaceRoots(roots map[string]bool) []string {
	result := make([]string, 0, len(roots))
	for root := range roots {
		result = append(result, root)
	}
	sort.Strings(result)
	return result
}

func (s *Server) didChangeWorkspaceFolders(raw json.RawMessage) error {
	var params struct {
		Event *struct {
			Added   []workspaceFolder `json:"added"`
			Removed []workspaceFolder `json:"removed"`
		} `json:"event"`
	}
	if err := json.Unmarshal(raw, &params); err != nil || params.Event == nil {
		return nil
	}
	roots := map[string]bool{}
	for _, root := range s.workspaceRoots {
		roots[root] = true
	}
	for _, folder := range params.Event.Removed {
		delete(roots, workspaceFolderPath(folder.URI))
	}
	for _, folder := range params.Event.Added {
		if path := workspaceFolderPath(folder.URI); path != "" {
			roots[path] = true
		}
	}
	next := sortedWorkspaceRoots(roots)
	if slices.Equal(next, s.workspaceRoots) {
		return nil
	}
	s.workspaceRoots = next
	s.advanceGeneration()
	// Drop stale context diagnostics immediately, even without a source edit.
	var uris []string
	for uri := range s.documents {
		uris = append(uris, uri)
	}
	sort.Strings(uris)
	for _, uri := range uris {
		if err := s.publishFor(uri); err != nil {
			return err
		}
	}
	return nil
}
