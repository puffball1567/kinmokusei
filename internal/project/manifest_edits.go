package project

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"
)

type manifestTextLine struct {
	body, ending        string
	key, keyText, value string
	valueStart          int
	comment             string
}

type manifestTextBlock struct {
	name     string
	lines    []manifestTextLine
	insertAt int
}

type manifestValueEdit struct {
	keyText, value string
	remove         bool
}

// RenderManifest is the canonical writer for new manifests. Dependency edits
// instead patch only semantic changes into the original source. Comparing the
// canonical models avoids normalizing unchanged defaults, escaping or tags.
func renderManifestChanges(manifest Manifest) ([]byte, error) {
	desired, err := RenderManifest(manifest)
	if err != nil || len(manifest.Contents) == 0 {
		return desired, err
	}
	original, err := ParseManifest(manifest.Path, manifest.Contents)
	if err != nil {
		return nil, err
	}
	before, err := RenderManifest(original)
	if err != nil {
		return nil, err
	}
	oldBlocks, err := readManifestText(before)
	if err != nil {
		return nil, err
	}
	newBlocks, err := readManifestText(desired)
	if err != nil {
		return nil, err
	}
	changes := changedManifestValues(oldBlocks, newBlocks)
	if len(changes) == 0 {
		return append([]byte(nil), manifest.Contents...), nil
	}
	blocks, err := readManifestText(manifest.Contents)
	if err != nil {
		return nil, err
	}
	ending := "\n"
	if index := bytes.IndexByte(manifest.Contents, '\n'); index > 0 && manifest.Contents[index-1] == '\r' {
		ending = "\r\n"
	}
	var out strings.Builder
	for _, block := range blocks {
		pending := changes[block.name]
		for index := 0; index <= len(block.lines); index++ {
			if index == block.insertAt && block.name != "" {
				writeManifestAdditions(&out, pending, ending)
			}
			if index == len(block.lines) {
				break
			}
			line := block.lines[index]
			edit, changed := pending[line.key]
			if line.key == "" || !changed {
				out.WriteString(line.body + line.ending)
				continue
			}
			delete(pending, line.key)
			if edit.remove {
				// A deleted setting may still carry useful user documentation.
				// Retain its inline comment as a standalone comment.
				if line.comment != "" {
					indent := len(line.body) - len(strings.TrimLeftFunc(line.body, unicode.IsSpace))
					out.WriteString(line.body[:indent] + line.comment + line.ending)
				}
				continue
			}
			out.WriteString(line.body[:line.valueStart] + edit.value + line.body[line.valueStart+len(line.value):] + line.ending)
		}
		delete(changes, block.name)
	}
	// Append new tables in the canonical table order without moving old ones.
	for _, block := range newBlocks {
		if len(changes[block.name]) == 0 {
			continue
		}
		ensureManifestLineBreak(&out, ending)
		if out.Len() != 0 && !strings.HasSuffix(out.String(), ending+ending) {
			out.WriteString(ending)
		}
		out.WriteString("[" + block.name + "]" + ending)
		writeManifestAdditions(&out, changes[block.name], ending)
	}
	contents := []byte(out.String())
	// Prove that preserving source trivia did not change the requested model
	// before the transaction writes any manifest or lock state.
	parsed, err := ParseManifest(manifest.Path, contents)
	if err != nil {
		return nil, fmt.Errorf("cannot apply manifest changes: %w", err)
	}
	actual, err := RenderManifest(parsed)
	if err != nil || !bytes.Equal(actual, desired) {
		return nil, fmt.Errorf("manifest edits did not preserve the requested settings")
	}
	return contents, nil
}

func changedManifestValues(before, after []manifestTextBlock) map[string]map[string]manifestValueEdit {
	entries := func(blocks []manifestTextBlock) map[string]map[string]manifestTextLine {
		result := map[string]map[string]manifestTextLine{}
		for _, block := range blocks {
			result[block.name] = map[string]manifestTextLine{}
			for _, line := range block.lines {
				if line.key != "" {
					result[block.name][line.key] = line
				}
			}
		}
		return result
	}
	old, updated := entries(before), entries(after)
	changes := map[string]map[string]manifestValueEdit{}
	put := func(section, key string, edit manifestValueEdit) {
		if changes[section] == nil {
			changes[section] = map[string]manifestValueEdit{}
		}
		changes[section][key] = edit
	}
	for section, values := range old {
		for key := range values {
			if _, exists := updated[section][key]; !exists {
				put(section, key, manifestValueEdit{remove: true})
			}
		}
	}
	for section, values := range updated {
		for key, line := range values {
			if previous, exists := old[section][key]; !exists || previous.value != line.value {
				put(section, key, manifestValueEdit{keyText: line.keyText, value: line.value})
			}
		}
	}
	return changes
}

// The strict manifest parser has already validated this line-oriented syntax.
// Reuse its string/comment rules so '#' and '=' inside escaped quoted strings
// never become trivia or assignment delimiters here.
func readManifestText(contents []byte) ([]manifestTextBlock, error) {
	blocks := []manifestTextBlock{{}}
	for _, text := range strings.SplitAfter(string(contents), "\n") {
		if text == "" {
			continue
		}
		line := manifestTextLine{body: text}
		if strings.HasSuffix(text, "\n") {
			line.ending = "\n"
			line.body = strings.TrimSuffix(text, "\n")
			if strings.HasSuffix(line.body, "\r") {
				line.body = strings.TrimSuffix(line.body, "\r")
				line.ending = "\r\n"
			}
		}
		code, err := stripComment(line.body)
		if err != nil {
			return nil, err
		}
		line.comment = line.body[len(code):]
		trimmed := strings.TrimSpace(code)
		if strings.HasPrefix(trimmed, "[") {
			blocks = append(blocks, manifestTextBlock{name: strings.TrimSpace(trimmed[1 : len(trimmed)-1]), insertAt: 1})
		} else if trimmed != "" {
			line.keyText, line.value, err = splitAssignment(code)
			if err != nil {
				return nil, err
			}
			line.key = line.keyText
			if strings.HasPrefix(line.key, "\"") {
				line.key, err = quotedString(line.key)
				if err != nil {
					return nil, err
				}
			}
			line.valueStart = len(strings.TrimRightFunc(code, unicode.IsSpace)) - len(line.value)
		}
		block := &blocks[len(blocks)-1]
		block.lines = append(block.lines, line)
		if line.key != "" {
			block.insertAt = len(block.lines)
		}
	}
	return blocks, nil
}

func ensureManifestLineBreak(out *strings.Builder, ending string) {
	if out.Len() != 0 && !strings.HasSuffix(out.String(), "\n") {
		out.WriteString(ending)
	}
}

func writeManifestAdditions(out *strings.Builder, changes map[string]manifestValueEdit, ending string) {
	for _, key := range sortedKeys(changes) {
		edit := changes[key]
		if edit.remove {
			continue
		}
		ensureManifestLineBreak(out, ending)
		out.WriteString(edit.keyText + " = " + edit.value + ending)
	}
}
