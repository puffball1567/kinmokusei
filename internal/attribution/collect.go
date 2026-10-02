// Package attribution collects original redistribution notices, not legal
// conclusions about whether a dependency's terms permit a particular use.
package attribution

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/puffball1567/kinmokusei/internal/product"
)

const maxFileSize = 16 << 20
const maxBundleSize = 128 << 20

type Module struct {
	Path, Version, Dir string
	Main               bool
	Replace            *Module
}

type Package struct {
	ImportPath, Dir                                                     string
	Standard                                                            bool
	Module                                                              *Module
	GoFiles, CgoFiles, CFiles, CXXFiles, MFiles, HFiles, FFiles, SFiles []string
	SwigFiles, SwigCXXFiles, EmbedFiles, SysoFiles                      []string
}

type SourcePackage struct {
	Path, Version, Dir string
}

type Toolchain struct {
	GOROOT, GOVERSION, GOOS, GOARCH, CGO_ENABLED string
}

type File struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type Component struct {
	Kind        string   `json:"kind"`
	Path        string   `json:"path"`
	Version     string   `json:"version,omitempty"`
	Replacement string   `json:"replacement,omitempty"`
	Directory   string   `json:"directory"`
	Packages    []string `json:"packages"`
}

type Index struct {
	Format           string      `json:"format"`
	Toolchain        string      `json:"toolchain"`
	GOOS             string      `json:"goos"`
	GOARCH           string      `json:"goarch"`
	CGOEnabled       bool        `json:"cgoEnabled"`
	BuildTags        []string    `json:"buildTags"`
	ExecutableSHA256 string      `json:"executableSHA256"`
	Components       []Component `json:"components"`
	Files            []File      `json:"files"`
}

type Bundle struct {
	Index Index
	Files map[string][]byte
}

var noticeName = regexp.MustCompile(`(?i)^(LICENSE|LICENCE|COPYING|NOTICE|PATENTS)([._-].*)?$`)
var licenseName = regexp.MustCompile(`(?i)^(LICENSE|LICENCE|COPYING)([._-].*)?$`)
var licenseDirectory = regexp.MustCompile(`(?i)^(LICENSES|LICENCES)$`)
var additionalNotice = regexp.MustCompile(`(?i)copyright|licen[cs]e|permission|redistribution|public domain`)
var goHeader = regexp.MustCompile(`// Copyright[^\n]*The Go Authors[^\n]*\r?\n// Use of this source code is governed by a BSD-style\r?\n// license that can be found in the LICENSE file\.[^\n]*`)

//go:embed KINMOKUSEI_LICENSE.txt
var kinmokuseiLicense []byte

type collector struct {
	bundle Bundle
	total  int
}

// Collect keeps complete originals. Package-level collection deliberately
// includes notices for code the linker may discard. No source path is published.
func Collect(toolchain Toolchain, packages []Package, sources []SourcePackage, tags []string) (Bundle, error) {
	c := collector{bundle: Bundle{Files: map[string][]byte{}, Index: Index{
		Format: "kinmokusei-attribution-v1", Toolchain: toolchain.GOVERSION,
		GOOS: toolchain.GOOS, GOARCH: toolchain.GOARCH, CGOEnabled: toolchain.CGO_ENABLED == "1",
		BuildTags: append([]string{}, tags...), Components: []Component{}, Files: []File{},
	}}}
	root, err := filepath.EvalSymlinks(toolchain.GOROOT)
	if err != nil {
		return Bundle{}, fmt.Errorf("cannot locate Go attribution: %w", err)
	}
	for _, name := range []string{"LICENSE", "PATENTS"} {
		if err := c.add(root, "go", filepath.Join(root, name), ""); err != nil {
			return Bundle{}, err
		}
	}
	components := map[string]*Component{}
	c.bundle.Files["kinmokusei/LICENSE"] = kinmokuseiLicense
	components["kinmokusei-runtime"] = &Component{Kind: "kinmokusei-runtime", Path: "github.com/puffball1567/kinmokusei", Version: product.VersionString(), Directory: "kinmokusei", Packages: []string{}}
	moduleRoots := map[string]string{}
	for _, pkg := range packages {
		if pkg.Dir == "" { // unsafe and other pseudo packages
			continue
		}
		componentRoot, prefix := root, "go"
		logicalRoot := toolchain.GOROOT
		key := "go"
		if !pkg.Standard {
			if pkg.Module == nil {
				return Bundle{}, fmt.Errorf("cannot determine license owner for Go package %s", pkg.ImportPath)
			}
			if pkg.Module.Main {
				continue // the application's own module, including generated code
			}
			key = "go-module:" + pkg.Module.Path + "@" + pkg.Module.Version
			module := pkg.Module
			if module.Replace != nil {
				module = module.Replace
			}
			logicalRoot = module.Dir
			componentRoot, err = filepath.EvalSymlinks(module.Dir)
			if err != nil {
				return Bundle{}, fmt.Errorf("cannot read notices for %s: %w", pkg.Module.Path, err)
			}
			prefix = componentDirectory("modules", key)
			if components[key] == nil {
				replacement := ""
				if pkg.Module.Replace != nil && pkg.Module.Replace.Version != "" {
					replacement = pkg.Module.Replace.Path + "@" + pkg.Module.Replace.Version
				}
				components[key] = &Component{Kind: "go-module", Path: pkg.Module.Path, Version: pkg.Module.Version, Replacement: replacement, Directory: prefix, Packages: []string{}}
				moduleRoots[key] = componentRoot
				found, scanErr := c.scanNotices(componentRoot, prefix)
				if scanErr != nil {
					return Bundle{}, scanErr
				}
				if !found {
					return Bundle{}, fmt.Errorf("license text not found for Go module %s@%s; provide original LICENSE/LICENCE/COPYING files before distributing", pkg.Module.Path, pkg.Module.Version)
				}
			} else if moduleRoots[key] != componentRoot {
				return Bundle{}, fmt.Errorf("inconsistent attribution roots for %s", pkg.Module.Path)
			}
		} else if components[key] == nil {
			components[key] = &Component{Kind: "go", Path: "Go", Version: toolchain.GOVERSION, Directory: "go", Packages: []string{}}
		}
		components[key].Packages = append(components[key].Packages, pkg.ImportPath)
		// Resolve only the explicitly selected root alias. Interior source/notice
		// symlinks are still rejected by safeRelative rather than followed.
		if logicalRoot != componentRoot {
			if relative, err := filepath.Rel(logicalRoot, pkg.Dir); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative) {
				pkg.Dir = filepath.Join(componentRoot, relative)
			}
		}
		if err := c.packageFiles(componentRoot, prefix, pkg); err != nil {
			return Bundle{}, err
		}
	}
	for _, source := range sources {
		key := "kinmokusei-package:" + source.Path + "@" + source.Version
		prefix := componentDirectory("packages", key)
		root, err := filepath.EvalSymlinks(source.Dir)
		if err != nil {
			return Bundle{}, err
		}
		found, err := c.scanNotices(root, prefix)
		if err != nil {
			return Bundle{}, err
		}
		if !found {
			return Bundle{}, fmt.Errorf("license text not found for Kinmokusei package %s@%s; manifest license metadata is not a license text", source.Path, source.Version)
		}
		// Source-package code is flattened into the application by the linker.
		err = filepath.WalkDir(root, func(file string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if file != root && excludedDirectory(entry.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(file) == ".km" {
				return c.supplement(root, prefix, file)
			}
			return nil
		})
		if err != nil {
			return Bundle{}, err
		}
		components[key] = &Component{Kind: "kinmokusei-package", Path: source.Path, Version: source.Version, Directory: prefix, Packages: []string{}}
	}
	keys := make([]string, 0, len(components))
	for key := range components {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		component := components[key]
		sort.Strings(component.Packages)
		c.bundle.Index.Components = append(c.bundle.Index.Components, *component)
	}
	return c.bundle, nil
}

func componentDirectory(kind, identity string) string {
	sum := sha256.Sum256([]byte(identity))
	return kind + "/" + hex.EncodeToString(sum[:])
}

func excludedDirectory(name string) bool {
	return name == ".git" || name == ".kinmokusei" || name == "node_modules"
}

func (c *collector) scanNotices(root, prefix string) (bool, error) {
	found := false
	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if file != root && excludedDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			if file != root && (licenseDirectory.MatchString(entry.Name()) || licenseName.MatchString(entry.Name())) {
				filesFound := false
				err := filepath.WalkDir(file, func(license string, item fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if item.IsDir() {
						return nil
					}
					if err := c.add(root, prefix, license, ""); err != nil {
						return err
					}
					filesFound = true
					return nil
				})
				if err != nil {
					return err
				}
				if !filesFound {
					return fmt.Errorf("empty license directory")
				}
				found = true
				return filepath.SkipDir
			}
			return nil
		}
		if noticeName.MatchString(entry.Name()) {
			if err := c.add(root, prefix, file, ""); err != nil {
				return err
			}
			found = found || licenseName.MatchString(entry.Name())
		}
		return nil
	})
	return found, err
}

func (c *collector) packageFiles(root, prefix string, pkg Package) error {
	if _, err := safeRelative(root, pkg.Dir); err != nil {
		return err
	}
	for directory := pkg.Dir; ; directory = filepath.Dir(directory) {
		entries, err := os.ReadDir(directory)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if noticeName.MatchString(entry.Name()) && !entry.IsDir() {
				if err := c.add(root, prefix, filepath.Join(directory, entry.Name()), ""); err != nil {
					return err
				}
			}
		}
		if directory == root {
			break
		}
	}
	for _, files := range [][]string{pkg.GoFiles, pkg.CgoFiles, pkg.CFiles, pkg.CXXFiles, pkg.MFiles, pkg.HFiles, pkg.FFiles, pkg.SFiles, pkg.SwigFiles, pkg.SwigCXXFiles, pkg.EmbedFiles} {
		for _, name := range files {
			if err := c.supplement(root, prefix, filepath.Join(pkg.Dir, name)); err != nil {
				return err
			}
		}
	}
	if len(pkg.SysoFiles) != 0 {
		return fmt.Errorf("binary objects in %s require an explicit licensing review", pkg.ImportPath)
	}
	return nil
}

func safeRelative(root, file string) (string, error) {
	relative, err := filepath.Rel(root, file)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("attribution path escapes its component root")
	}
	// Check every component; an intermediate symlink is just as unsafe as a
	// symlinked LICENSE. The selected toolchain/module root is already resolved.
	current := root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("symlinked attribution source %s", filepath.ToSlash(relative))
		}
	}
	return filepath.ToSlash(relative), nil
}

func readOriginal(root, file string) ([]byte, string, error) {
	relative, err := safeRelative(root, file)
	if err != nil {
		return nil, "", err
	}
	info, err := os.Lstat(file)
	if err != nil {
		return nil, "", err
	}
	if !info.Mode().IsRegular() || info.Size() > maxFileSize {
		return nil, "", fmt.Errorf("invalid or oversized attribution source %s", relative)
	}
	input, err := os.Open(file)
	if err != nil {
		return nil, "", err
	}
	defer input.Close()
	data, err := io.ReadAll(io.LimitReader(input, maxFileSize+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > maxFileSize {
		return nil, "", fmt.Errorf("oversized attribution source %s", relative)
	}
	return data, relative, nil
}

func (c *collector) supplement(root, prefix, file string) error {
	data, _, err := readOriginal(root, file)
	if err != nil {
		return err
	}
	if additionalNotice.Match(goHeader.ReplaceAll(data, nil)) {
		return c.add(root, prefix, file, ".txt")
	}
	return nil
}

func (c *collector) add(root, prefix, file, suffix string) error {
	data, relative, err := readOriginal(root, file)
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return fmt.Errorf("empty attribution source %s", relative)
	}
	name := prefix + "/" + relative + suffix
	if prior, ok := c.bundle.Files[name]; ok {
		if string(prior) != string(data) {
			return fmt.Errorf("attribution source changed during collection: %s", relative)
		}
		return nil
	}
	c.total += len(data)
	if c.total > maxBundleSize {
		return fmt.Errorf("attribution bundle exceeds size limit")
	}
	c.bundle.Files[name] = data
	return nil
}

func (b Bundle) Write(directory string) error {
	b.Files["THIRD_PARTY_NOTICES.md"] = []byte(applicationNotices)
	names := make([]string, 0, len(b.Files))
	for name := range b.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	b.Index.Files = []File{}
	for _, name := range names {
		data := b.Files[name]
		file := filepath.Join(directory, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(file, data, 0o644); err != nil {
			return err
		}
		digest := sha256.Sum256(data)
		b.Index.Files = append(b.Index.Files, File{Path: name, SHA256: hex.EncodeToString(digest[:])})
	}
	index, err := json.MarshalIndent(b.Index, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(directory, "INDEX.json"), append(index, '\n'), 0o644); err != nil {
		return err
	}
	return nil
}

const applicationNotices = `# Application third-party notices

Keep this entire directory with the executable when redistributing it.
INDEX.json identifies the Go toolchain, target, build tags, dependency identities
and SHA-256 digests. Original licenses, notices and patent grants are copied
verbatim. Source supplements retain complete files containing additional notices;
they can include code removed by the linker. Source-package notices are collected
conservatively for the locked package graph, including unused files/dependencies.

These materials do not grant additional rights or certify legal compliance.
Review the terms for your use and distribution, including required source offers,
modification notices and dynamically linked/native system libraries. Libraries
outside the Go/source package graph are not inventoried automatically. The
application's own license is the distributor's responsibility.

Kinmokusei-generated runtime helpers and embedded standard-library code are
covered by kinmokusei/LICENSE (Apache-2.0). This is not a license declaration
for your application source.

Upstream Go license: https://go.dev/LICENSE
`
