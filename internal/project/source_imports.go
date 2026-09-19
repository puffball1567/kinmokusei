package project

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

var sourceImportAliasPattern = regexp.MustCompile(`^[A-Za-z0-9_@][A-Za-z0-9_@./-]*$`)

func defaultSourceImportAlias(module string) string {
	name := path.Base(module)
	// A Go semantic-import-version suffix is not the library's short name.
	if strings.HasPrefix(name, "v") && len(name) > 1 && name != "v1" && name[1] != '0' && strings.Trim(name[1:], "0123456789") == "" {
		return path.Base(path.Dir(module))
	}
	return name
}

func (m *Manifest) addSourceImportAlias(module, alias string) error {
	if alias == "" {
		alias = defaultSourceImportAlias(module)
	}
	for _, existing := range sortedKeys(m.Imports) {
		if importPrefix(alias, existing) || importPrefix(existing, alias) {
			return fmt.Errorf("source import alias %q conflicts with existing alias %q; use keika deps add --alias <name>", alias, existing)
		}
	}
	if m.Imports == nil {
		m.Imports = map[string]string{}
	}
	m.Imports[alias] = module
	if err := m.validateSourceImports(); err != nil {
		delete(m.Imports, alias)
		return fmt.Errorf("%w; use keika deps add --alias <name>", err)
	}
	return nil
}

func importPrefix(imported, prefix string) bool {
	return imported == prefix || strings.HasPrefix(imported, prefix+"/")
}

func (m Manifest) validateSourceImports() error {
	for _, alias := range sortedKeys(m.Imports) {
		module := m.Imports[alias]
		if !canonicalPackageSubpath(alias) || !sourceImportAliasPattern.MatchString(alias) {
			return fmt.Errorf("invalid source import alias %q: expected a canonical non-relative import prefix", alias)
		}
		if importPrefix(alias, "kinmokusei") {
			return fmt.Errorf("source import alias %q conflicts with the reserved kinmokusei namespace", alias)
		}
		if _, exists := m.Packages[module]; !exists {
			return fmt.Errorf("source import alias %q target %q must name a direct [dependencies] module", alias, module)
		}
		for _, canonical := range append(sortedKeys(m.Packages), m.Project.GoModule) {
			if importPrefix(alias, canonical) || importPrefix(canonical, alias) {
				return fmt.Errorf("source import alias %q conflicts with canonical module path %q", alias, canonical)
			}
		}
	}
	return nil
}

// Expand once, using the longest slash-delimited prefix. Targets are canonical
// direct dependencies, never another alias. No settings cross package boundaries.
func (m Manifest) expandSourceImport(imported string) string {
	matched := ""
	for alias := range m.Imports {
		if importPrefix(imported, alias) && len(alias) > len(matched) {
			matched = alias
		}
	}
	if matched == "" {
		return imported
	}
	return m.Imports[matched] + strings.TrimPrefix(imported, matched)
}
