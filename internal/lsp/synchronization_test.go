package lsp

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"ontama.local/ontama/internal/source"
)

func intPointer(value int) *int { return &value }

func changeRange(startLine, startCharacter, endLine, endCharacter int) *protocolRange {
	return &protocolRange{
		Start: position{Line: startLine, Character: startCharacter},
		End:   position{Line: endLine, Character: endCharacter},
	}
}

func TestApplyIncrementalContentChangesMatrix(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		changes []contentChange
		want    string
	}{
		{
			name: "ASCII replacement with range length", text: "abc",
			changes: []contentChange{{Range: changeRange(0, 1, 0, 2), RangeLength: intPointer(1), Text: "X"}}, want: "aXc",
		},
		{
			name: "zero width insertion", text: "ac",
			changes: []contentChange{{Range: changeRange(0, 1, 0, 1), RangeLength: intPointer(0), Text: "b"}}, want: "abc",
		},
		{
			name: "deletion", text: "abc",
			changes: []contentChange{{Range: changeRange(0, 1, 0, 2), Text: ""}}, want: "ac",
		},
		{
			name: "astral character uses two UTF-16 units", text: "a😀b",
			changes: []contentChange{{Range: changeRange(0, 1, 0, 3), RangeLength: intPointer(2), Text: "温泉"}}, want: "a温泉b",
		},
		{
			name: "multiline LF replacement", text: "one\ntwo\nthree",
			changes: []contentChange{{Range: changeRange(0, 2, 2, 2), Text: "X"}}, want: "onXree",
		},
		{
			name: "multiline CRLF replacement", text: "one\r\ntwo\r\nthree",
			changes: []contentChange{{Range: changeRange(0, 3, 1, 0), RangeLength: intPointer(2), Text: " "}}, want: "one two\r\nthree",
		},
		{
			name: "bare CR line boundary", text: "one\rtwo",
			changes: []contentChange{{Range: changeRange(1, 0, 1, 3), Text: "next"}}, want: "one\rnext",
		},
		{
			name: "sequential ranges use updated text", text: "abc",
			changes: []contentChange{
				{Range: changeRange(0, 1, 0, 2), Text: "XY"},
				{Range: changeRange(0, 3, 0, 4), Text: "z"},
			}, want: "aXYz",
		},
		{
			name: "full replacement followed by incremental change", text: "old",
			changes: []contentChange{
				{Text: "温泉"},
				{Range: changeRange(0, 2, 0, 2), Text: "卵"},
			}, want: "温泉卵",
		},
		{
			name: "later full replacement wins", text: "old",
			changes: []contentChange{
				{Range: changeRange(0, 0, 0, 3), Text: "intermediate"},
				{Text: "final"},
			}, want: "final",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := applyContentChanges(test.text, test.changes)
			if err != nil {
				t.Fatalf("applyContentChanges() error: %v", err)
			}
			if got != test.want {
				t.Fatalf("applyContentChanges() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestIncrementalContentChangeFailureIsTransactional(t *testing.T) {
	tests := []struct {
		name   string
		change contentChange
	}{
		{"negative start", contentChange{Range: changeRange(0, -1, 0, 0), Text: "x"}},
		{"line outside document", contentChange{Range: changeRange(2, 0, 2, 0), Text: "x"}},
		{"character outside line", contentChange{Range: changeRange(0, 99, 0, 99), Text: "x"}},
		{"reversed range", contentChange{Range: changeRange(0, 2, 0, 1), Text: "x"}},
		{"split surrogate pair start", contentChange{Range: changeRange(0, 2, 0, 3), Text: "x"}},
		{"split surrogate pair end", contentChange{Range: changeRange(0, 1, 0, 2), Text: "x"}},
		{"range length mismatch", contentChange{Range: changeRange(0, 1, 0, 3), RangeLength: intPointer(1), Text: "x"}},
		{"range length without range", contentChange{RangeLength: intPointer(0), Text: "x"}},
		{"CRLF is not an addressable character", contentChange{Range: changeRange(1, 0, 0, 4), Text: "x"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			original := "a😀b\r\nnext"
			changes := []contentChange{
				{Range: changeRange(1, 0, 1, 4), Text: "changed"},
				test.change,
			}
			if got, err := applyContentChanges(original, changes); err == nil || got != "" {
				t.Fatalf("applyContentChanges() = %q, %v; want transactional failure", got, err)
			}
		})
	}
}

func TestDidChangeVersionAndRollbackMatrix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "version.otm")
	uri := fileURI(path)
	initial := `function value(): int { return 1; }`
	var output bytes.Buffer
	server := &Server{
		writer: &output,
		documents: map[string]document{
			uri: {URI: uri, Path: path, Text: initial, Version: 3},
		},
		diagnostics: map[string]map[string][]protocolDiagnostic{},
	}
	change := func(version int, changes []contentChange) {
		raw, err := json.Marshal(map[string]any{
			"textDocument":   map[string]any{"uri": uri, "version": version},
			"contentChanges": changes,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err = server.didChange(raw); err != nil {
			t.Fatal(err)
		}
	}

	change(2, []contentChange{{Text: "stale older"}})
	change(3, []contentChange{{Text: "stale equal"}})
	change(4, nil)
	if doc := server.documents[uri]; doc.Version != 3 || doc.Text != initial || output.Len() != 0 {
		t.Fatalf("stale/empty changes mutated state: %#v output=%q", doc, output.String())
	}
	missingText, marshalErr := json.Marshal(map[string]any{
		"textDocument": map[string]any{"uri": uri, "version": 4},
		"contentChanges": []any{map[string]any{
			"range": changeRange(0, 0, 0, 0),
		}},
	})
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	if err := server.didChange(missingText); err != nil {
		t.Fatal(err)
	}
	if doc := server.documents[uri]; doc.Version != 3 || doc.Text != initial || output.Len() != 0 {
		t.Fatalf("malformed change mutated state: %#v output=%q", doc, output.String())
	}

	change(4, []contentChange{{Range: changeRange(0, 31, 0, 32), RangeLength: intPointer(2), Text: "2"}})
	if doc := server.documents[uri]; doc.Version != 3 || doc.Text != initial || output.Len() != 0 {
		t.Fatalf("invalid change mutated state: %#v output=%q", doc, output.String())
	}

	one := len(initial) - 4
	change(4, []contentChange{{Range: changeRange(0, one, 0, one+1), RangeLength: intPointer(1), Text: "2"}})
	doc := server.documents[uri]
	if doc.Version != 4 || doc.Text != `function value(): int { return 2; }` {
		t.Fatalf("valid change state = %#v", doc)
	}
	if output.Len() == 0 {
		t.Fatal("valid change did not publish diagnostics")
	}
}

func TestUTF16AndLineEndingPositionRoundTripMatrix(t *testing.T) {
	text := "a😀\r\n温b\rx\n"
	tests := []struct {
		offset   int
		position position
	}{
		{0, position{Line: 0, Character: 0}},
		{1, position{Line: 0, Character: 1}},
		{5, position{Line: 0, Character: 3}},
		{7, position{Line: 1, Character: 0}},
		{10, position{Line: 1, Character: 1}},
		{11, position{Line: 1, Character: 2}},
		{12, position{Line: 2, Character: 0}},
		{13, position{Line: 2, Character: 1}},
		{14, position{Line: 3, Character: 0}},
	}
	for _, test := range tests {
		got := protocolPosition(text, source.Position{Offset: test.offset})
		if got != test.position {
			t.Errorf("protocolPosition(offset %d) = %#v, want %#v", test.offset, got, test.position)
		}
		offset, err := byteOffsetAtPosition(text, test.position)
		if err != nil || offset != test.offset {
			t.Errorf("byteOffsetAtPosition(%#v) = %d, %v; want %d", test.position, offset, err, test.offset)
		}
	}
	if _, err := byteOffsetAtPosition(text, position{Line: 0, Character: 2}); err == nil {
		t.Fatal("position splitting surrogate pair was accepted")
	}
}

func FuzzApplyContentChangesNeverPanics(f *testing.F) {
	seeds := []struct {
		text, replacement                                string
		startLine, startCharacter, endLine, endCharacter int
		rangeLength                                      int
	}{
		{"", "x", 0, 0, 0, 0, 0},
		{"a😀b", "温泉", 0, 1, 0, 3, 2},
		{"one\r\ntwo", " ", 0, 3, 1, 0, 2},
		{"one\rtwo\n", "next", 1, 0, 1, 3, 3},
		{"abc", "x", 0, 2, 0, 1, 1},
	}
	for _, seed := range seeds {
		f.Add(seed.text, seed.replacement, seed.startLine, seed.startCharacter, seed.endLine, seed.endCharacter, seed.rangeLength)
	}
	f.Fuzz(func(t *testing.T, text, replacement string, startLine, startCharacter, endLine, endCharacter, rangeLength int) {
		_, _ = applyContentChanges(text, []contentChange{{
			Range:       changeRange(startLine, startCharacter, endLine, endCharacter),
			RangeLength: intPointer(rangeLength),
			Text:        replacement,
		}})
	})
}
