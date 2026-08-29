package project

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"ontama.local/ontama/internal/product"
)

type Project struct {
	Name      string
	Version   string
	GoModule  string
	GoVersion string
}

type TargetConfig struct {
	GOOS   string
	GOARCH string
	CGO    string
	Tags   []string
}

type GoInteropConfig struct {
	Unsafe string
}

type Manifest struct {
	Root         string
	Path         string
	Contents     []byte
	Project      Project
	Target       TargetConfig
	GoInterop    GoInteropConfig
	Dependencies map[string]string
	Replacements map[string]string
}

var (
	projectNamePattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	versionPattern         = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)
	goModuleVersionPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.]+)?(?:\+[0-9A-Za-z.]+)?$`)
	goVersionPattern       = regexp.MustCompile(`^1\.[0-9]+(?:\.[0-9]+)?$`)
)

func ReadManifest(root string) (Manifest, error) {
	path := filepath.Join(root, product.ProjectFileName)
	contents, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("cannot read project manifest %s: %w", path, err)
	}
	return ParseManifest(path, contents)
}

func FindRoot(path string) (string, bool, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", false, err
	}
	start := absolute
	if info, statErr := os.Stat(absolute); statErr == nil && !info.IsDir() {
		start = filepath.Dir(absolute)
	} else if statErr != nil && filepath.Ext(absolute) != "" {
		start = filepath.Dir(absolute)
	}
	for directory := start; ; directory = filepath.Dir(directory) {
		if _, statErr := os.Stat(filepath.Join(directory, product.ProjectFileName)); statErr == nil {
			return directory, true, nil
		} else if !os.IsNotExist(statErr) {
			return "", false, statErr
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", false, nil
		}
	}
}

func ParseManifest(path string, contents []byte) (Manifest, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return Manifest{}, err
	}
	manifest := Manifest{
		Root: filepath.Dir(absolute), Path: absolute, Contents: append([]byte(nil), contents...),
		Dependencies: map[string]string{}, Replacements: map[string]string{},
	}
	section := ""
	seenSections := map[string]bool{}
	seenProject := map[string]bool{}
	seenTarget := map[string]bool{}
	seenGoInterop := map[string]bool{}
	scanner := bufio.NewScanner(strings.NewReader(string(contents)))
	for line := 1; scanner.Scan(); line++ {
		text, stripErr := stripComment(scanner.Text())
		if stripErr != nil {
			return Manifest{}, manifestError(path, line, stripErr.Error())
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		if strings.HasPrefix(text, "[") {
			if !strings.HasSuffix(text, "]") || strings.Count(text, "[") != 1 || strings.Count(text, "]") != 1 {
				return Manifest{}, manifestError(path, line, "malformed section header")
			}
			section = strings.TrimSpace(text[1 : len(text)-1])
			if section != "project" && section != "target" && section != "go.interop" && section != "go.dependencies" && section != "go.replacements" {
				return Manifest{}, manifestError(path, line, fmt.Sprintf("unknown section %q", section))
			}
			if seenSections[section] {
				return Manifest{}, manifestError(path, line, fmt.Sprintf("duplicate section %q", section))
			}
			seenSections[section] = true
			continue
		}
		if section == "" {
			return Manifest{}, manifestError(path, line, "key-value pair appears before a section")
		}
		keyText, valueText, splitErr := splitAssignment(text)
		if splitErr != nil {
			return Manifest{}, manifestError(path, line, splitErr.Error())
		}
		value, valueErr := quotedString(valueText)
		if valueErr != nil {
			return Manifest{}, manifestError(path, line, valueErr.Error())
		}
		switch section {
		case "project":
			key := strings.TrimSpace(keyText)
			if seenProject[key] {
				return Manifest{}, manifestError(path, line, fmt.Sprintf("duplicate project key %q", key))
			}
			seenProject[key] = true
			switch key {
			case "name":
				manifest.Project.Name = value
			case "version":
				manifest.Project.Version = value
			case "go-module":
				manifest.Project.GoModule = value
			case "go-version":
				manifest.Project.GoVersion = value
			default:
				return Manifest{}, manifestError(path, line, fmt.Sprintf("unknown project key %q", key))
			}
		case "target":
			key := strings.TrimSpace(keyText)
			if seenTarget[key] {
				return Manifest{}, manifestError(path, line, fmt.Sprintf("duplicate target key %q", key))
			}
			seenTarget[key] = true
			switch key {
			case "goos":
				manifest.Target.GOOS = value
			case "goarch":
				manifest.Target.GOARCH = value
			case "cgo":
				manifest.Target.CGO = value
			case "tags":
				manifest.Target.Tags, valueErr = parseBuildTags(value)
				if valueErr != nil {
					return Manifest{}, manifestError(path, line, valueErr.Error())
				}
			default:
				return Manifest{}, manifestError(path, line, fmt.Sprintf("unknown target key %q", key))
			}
		case "go.interop":
			key := strings.TrimSpace(keyText)
			if seenGoInterop[key] {
				return Manifest{}, manifestError(path, line, fmt.Sprintf("duplicate Go interop key %q", key))
			}
			seenGoInterop[key] = true
			switch key {
			case "unsafe":
				manifest.GoInterop.Unsafe = value
			default:
				return Manifest{}, manifestError(path, line, fmt.Sprintf("unknown Go interop key %q", key))
			}
		case "go.dependencies", "go.replacements":
			key, keyErr := quotedString(keyText)
			if keyErr != nil {
				return Manifest{}, manifestError(path, line, "Go module paths must be quoted strings")
			}
			target := manifest.Dependencies
			if section == "go.replacements" {
				target = manifest.Replacements
			}
			if _, exists := target[key]; exists {
				return Manifest{}, manifestError(path, line, fmt.Sprintf("duplicate Go module %q in [%s]", key, section))
			}
			target[key] = value
		}
	}
	if err := scanner.Err(); err != nil {
		return Manifest{}, fmt.Errorf("cannot read project manifest %s: %w", path, err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, fmt.Errorf("invalid project manifest %s: %w", path, err)
	}
	return manifest, nil
}

func (m Manifest) Validate() error {
	missing := make([]string, 0, 4)
	for key, value := range map[string]string{"name": m.Project.Name, "version": m.Project.Version, "go-module": m.Project.GoModule, "go-version": m.Project.GoVersion} {
		if value == "" {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	if len(missing) != 0 {
		return fmt.Errorf("[project] is missing required keys: %s", strings.Join(missing, ", "))
	}
	if !projectNamePattern.MatchString(m.Project.Name) {
		return fmt.Errorf("project name %q must contain only letters, digits, '.', '_' or '-'", m.Project.Name)
	}
	if !versionPattern.MatchString(m.Project.Version) {
		return fmt.Errorf("project version %q must be a complete semantic version such as 0.1.0", m.Project.Version)
	}
	if err := validateModulePath(m.Project.GoModule); err != nil {
		return fmt.Errorf("invalid project go-module: %w", err)
	}
	if !goVersionPattern.MatchString(m.Project.GoVersion) {
		return fmt.Errorf("go-version %q must have the form 1.N or 1.N.P", m.Project.GoVersion)
	}
	if err := m.Target.Validate(); err != nil {
		return err
	}
	if m.GoInterop.Unsafe != "" && m.GoInterop.Unsafe != "deny" && m.GoInterop.Unsafe != "allow" {
		return fmt.Errorf("go.interop unsafe %q must be deny or allow", m.GoInterop.Unsafe)
	}
	for path, version := range m.Dependencies {
		if err := validateModulePath(path); err != nil {
			return fmt.Errorf("invalid Go dependency %q: %w", path, err)
		}
		if path == m.Project.GoModule {
			return fmt.Errorf("project module %q cannot depend on itself", path)
		}
		if !goModuleVersionPattern.MatchString(version) {
			return fmt.Errorf("Go dependency %q has invalid complete version %q", path, version)
		}
	}
	for path, replacement := range m.Replacements {
		if _, exists := m.Dependencies[path]; !exists {
			return fmt.Errorf("replacement %q has no matching Go dependency", path)
		}
		if replacement == "" || filepath.IsAbs(replacement) {
			return fmt.Errorf("replacement %q must be a non-empty project-relative path", path)
		}
		clean := filepath.Clean(filepath.FromSlash(replacement))
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("replacement %q escapes the project root", path)
		}
	}
	return nil
}

func (m Manifest) AllowsUnsafeGoInterop() bool {
	return m.GoInterop.Unsafe == "allow"
}

func stripComment(line string) (string, error) {
	quoted, escaped := false, false
	for i, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if quoted && r == '\\' {
			escaped = true
			continue
		}
		if r == '"' {
			quoted = !quoted
			continue
		}
		if r == '#' && !quoted {
			return line[:i], nil
		}
	}
	if quoted {
		return "", fmt.Errorf("unterminated quoted string")
	}
	return line, nil
}

func splitAssignment(text string) (string, string, error) {
	quoted, escaped := false, false
	for i, r := range text {
		if escaped {
			escaped = false
			continue
		}
		if quoted && r == '\\' {
			escaped = true
			continue
		}
		if r == '"' {
			quoted = !quoted
			continue
		}
		if r == '=' && !quoted {
			key, value := strings.TrimSpace(text[:i]), strings.TrimSpace(text[i+1:])
			if key == "" || value == "" {
				return "", "", fmt.Errorf("assignment requires a key and value")
			}
			return key, value, nil
		}
	}
	return "", "", fmt.Errorf("expected '=' in assignment")
}

func quotedString(text string) (string, error) {
	text = strings.TrimSpace(text)
	if len(text) < 2 || text[0] != '"' || text[len(text)-1] != '"' {
		return "", fmt.Errorf("value must be a quoted string")
	}
	value, err := strconv.Unquote(text)
	if err != nil {
		return "", fmt.Errorf("invalid quoted string: %w", err)
	}
	return value, nil
}

func validateModulePath(path string) error {
	if path == "" || strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") || strings.Contains(path, "//") || strings.ContainsAny(path, "\\ \t\r\n") {
		return fmt.Errorf("module path %q is not canonical", path)
	}
	if !strings.Contains(path, ".") {
		return fmt.Errorf("module path %q must contain a domain-like first component", path)
	}
	return nil
}

func manifestError(path string, line int, message string) error {
	return fmt.Errorf("invalid project manifest %s:%d: %s", path, line, message)
}
