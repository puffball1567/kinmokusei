package compiler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/puffball1567/kinmokusei/internal/codegen"
	"github.com/puffball1567/kinmokusei/internal/diagnostic"
	"github.com/puffball1567/kinmokusei/internal/source"
)

type MapPosition struct {
	Offset int `json:"offset"`
	Line   int `json:"line"`
	Column int `json:"column"`
}

type MapRange struct {
	Start MapPosition `json:"start"`
	End   MapPosition `json:"end"`
}

type MapSource struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	ByteLength int    `json:"byte_length"`
	Embedded   bool   `json:"embedded,omitempty"`
}

type sourceMapInput struct {
	contents string
	embedded bool
}

type MapGenerated struct {
	File       string `json:"file"`
	SHA256     string `json:"sha256"`
	ByteLength int    `json:"byte_length"`
}

type GoSourceMapping struct {
	OriginID  string   `json:"origin_id"`
	SourceID  string   `json:"source_id"`
	Source    MapRange `json:"source"`
	Generated MapRange `json:"generated"`
}

// GoSourceMap is versioned independently of the language. Positions use bytes,
// one-based lines/columns, and half-open ranges; generated coordinates refer to
// the physical Go file, not //line-adjusted positions.
type GoSourceMap struct {
	Version    int               `json:"version"`
	SourceRoot string            `json:"source_root"`
	Generated  MapGenerated      `json:"generated"`
	Sources    []MapSource       `json:"sources"`
	Mappings   []GoSourceMapping `json:"mappings"`
}

type GoArtifacts struct {
	GoSource  []byte
	SourceMap GoSourceMap
	root      string
}

// WriteFiles publishes a matched pair. Hash verification detects interruption
// between the two renames. Neither output may overwrite a compilation input.
func (a GoArtifacts) WriteFiles(goPath, mapPath string) error {
	outputs := []string{goPath, mapPath}
	inputs := []string{}
	for _, entry := range a.SourceMap.Sources {
		if !entry.Embedded {
			inputs = append(inputs, filepath.Join(a.root, filepath.FromSlash(entry.Path)))
		}
	}
	for i, output := range outputs {
		for _, other := range append(inputs, outputs[:i]...) {
			same, err := sameArtifactPath(output, other)
			if err != nil {
				return err
			}
			if same {
				return fmt.Errorf("source-map outputs must be distinct from each other and source inputs: %q", output)
			}
		}
	}
	metadata, err := a.SourceMapJSON(goPath, mapPath)
	if err != nil {
		return err
	}
	var temporary []string
	defer func() {
		for _, path := range temporary {
			_ = os.Remove(path)
		}
	}()
	for i, contents := range [][]byte{a.GoSource, metadata} {
		file, err := os.CreateTemp(filepath.Dir(outputs[i]), ".kinmokusei-artifact-*")
		if err != nil {
			return err
		}
		temporary = append(temporary, file.Name())
		if err = file.Chmod(0o644); err == nil {
			_, err = file.Write(contents)
		}
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	for i, output := range outputs {
		if i > 0 {
			// On case-insensitive filesystems, two previously absent spellings
			// can become the same inode after publishing the first output.
			same, err := sameArtifactPath(outputs[0], output)
			if err != nil {
				return err
			}
			if same {
				return fmt.Errorf("source-map outputs resolve to the same file")
			}
		}
		if err := os.Rename(temporary[i], output); err != nil {
			return err
		}
	}
	return nil
}

func sameArtifactPath(left, right string) (bool, error) {
	l, err := canonicalArtifactPath(left)
	if err != nil {
		return false, err
	}
	r, err := canonicalArtifactPath(right)
	if err != nil {
		return false, err
	}
	if l == r {
		return true, nil
	}
	li, le := os.Stat(l)
	ri, re := os.Stat(r)
	if le != nil && !os.IsNotExist(le) {
		return false, le
	}
	if re != nil && !os.IsNotExist(re) {
		return false, re
	}
	return le == nil && re == nil && os.SameFile(li, ri), nil
}

func canonicalArtifactPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		return resolved, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	// New outputs have no inode yet, but their existing parent may be a
	// symlink. Resolve it before comparing planned filenames, not only after
	// publication when the second rename could replace the first artifact.
	parent, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err == nil {
		return filepath.Join(parent, filepath.Base(absolute)), nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	return absolute, nil // Missing parents fail before publishing either file.
}

// SourceMapJSON rebases portable paths to the sidecar location. Source paths
// remain project-root-relative, so origins survive changes to the output path.
func (a GoArtifacts) SourceMapJSON(goPath, mapPath string) ([]byte, error) {
	mapAbsolute, err := filepath.Abs(mapPath)
	if err != nil {
		return nil, err
	}
	goAbsolute, err := filepath.Abs(goPath)
	if err != nil {
		return nil, err
	}
	root, err := filepath.Rel(filepath.Dir(mapAbsolute), a.root)
	if err != nil {
		return nil, err
	}
	generated, err := filepath.Rel(filepath.Dir(mapAbsolute), goAbsolute)
	if err != nil {
		return nil, err
	}
	metadata := a.SourceMap
	metadata.SourceRoot = filepath.ToSlash(root)
	metadata.Generated.File = filepath.ToSlash(generated)
	data, err := json.MarshalIndent(metadata, "", "  ")
	return append(data, '\n'), err
}

// EmitGoWithSourceMap captures the exact inputs used by checking and emits
// executable origins for failure-location and source-coverage consumers.
func EmitGoWithSourceMap(paths []string, packageName string) (GoArtifacts, []diagnostic.Diagnostic, error) {
	if len(paths) == 0 {
		return GoArtifacts{}, nil, fmt.Errorf("at least one source path is required")
	}
	result, err := CheckFiles(paths)
	if err != nil {
		return GoArtifacts{}, nil, err
	}
	if len(result.Diagnostics) != 0 {
		return GoArtifacts{}, result.Diagnostics, nil
	}
	root, err := findProjectRoot(paths[0])
	if err != nil {
		return GoArtifacts{}, nil, err
	}
	return emitCheckedGoWithSourceMap(result, packageName, root)
}

func emitCheckedGoWithSourceMap(result Result, packageName, root string) (GoArtifacts, []diagnostic.Diagnostic, error) {
	generated, ranges, err := codegen.GenerateMappedWithTarget(result.Program, packageName, result.goImporter, result.goSizes)
	if err != nil {
		return GoArtifacts{}, nil, err
	}
	metadata := GoSourceMap{Version: 1, SourceRoot: ".", Generated: MapGenerated{File: "generated.go", SHA256: mapHash(generated), ByteLength: len(generated)}, Sources: []MapSource{}, Mappings: []GoSourceMapping{}}
	sources := map[string]MapSource{}
	lineStarts := map[string][]int{}
	for path, snapshot := range result.sourceInputs {
		input := snapshot.contents
		starts := []int{0}
		for offset := range input {
			if input[offset] == '\n' {
				starts = append(starts, offset+1)
			}
		}
		lineStarts[path] = starts
		portable := path
		if !snapshot.embedded {
			absolute, absErr := filepath.Abs(path)
			if absErr != nil {
				return GoArtifacts{}, nil, absErr
			}
			portable, err = filepath.Rel(root, absolute)
			if err != nil {
				return GoArtifacts{}, nil, err
			}
		}
		portable = filepath.ToSlash(portable)
		digest := mapHash([]byte(input))
		kind := "file"
		if snapshot.embedded {
			kind = "embedded"
		}
		identity, _ := json.Marshal([]string{kind, portable, digest})
		entry := MapSource{ID: mapHash(identity), Path: portable, SHA256: digest, ByteLength: len(input), Embedded: snapshot.embedded}
		sources[path] = entry
		metadata.Sources = append(metadata.Sources, entry)
	}
	sort.Slice(metadata.Sources, func(i, j int) bool { return metadata.Sources[i].Path < metadata.Sources[j].Path })
	for _, mapping := range ranges {
		entry, ok := sources[mapping.Source.Path]
		if !ok {
			return GoArtifacts{}, nil, fmt.Errorf("source mapping input missing for %q", mapping.Source.Path)
		}
		if mapping.Source.Start.Offset < 0 || mapping.Source.Start.Offset >= mapping.Source.End.Offset || mapping.Source.End.Offset > entry.ByteLength {
			return GoArtifacts{}, nil, fmt.Errorf("invalid source mapping span for %q", mapping.Source.Path)
		}
		// Lexer diagnostic columns count Unicode code points. This interchange
		// format uses byte columns consistently with Go's coverage coordinates.
		original := MapRange{Start: mapBytePosition(mapping.Source.Start.Offset, lineStarts[mapping.Source.Path]), End: mapBytePosition(mapping.Source.End.Offset, lineStarts[mapping.Source.Path])}
		identity, _ := json.Marshal(struct {
			ID    string
			Range MapRange
		}{entry.ID, original})
		metadata.Mappings = append(metadata.Mappings, GoSourceMapping{OriginID: mapHash(identity), SourceID: entry.ID, Source: original, Generated: mapRange(mapping.Generated)})
	}
	return GoArtifacts{GoSource: generated, SourceMap: metadata, root: root}, nil, nil
}

func mapBytePosition(offset int, lineStarts []int) MapPosition {
	line := sort.Search(len(lineStarts), func(i int) bool { return lineStarts[i] > offset })
	return MapPosition{Offset: offset, Line: line, Column: offset - lineStarts[line-1] + 1}
}

func mapRange(span source.Span) MapRange {
	return MapRange{Start: MapPosition{Offset: span.Start.Offset, Line: span.Start.Line, Column: span.Start.Column}, End: MapPosition{Offset: span.End.Offset, Line: span.End.Line, Column: span.End.Column}}
}

func mapHash(input []byte) string {
	digest := sha256.Sum256(input)
	return hex.EncodeToString(digest[:])
}
