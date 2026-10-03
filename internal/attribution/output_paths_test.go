package attribution

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNoticeOutputRejectsCollisionsAndNonportablePaths(t *testing.T) {
	for _, files := range []map[string][]byte{
		{"go/LICENSE": []byte("One"), "go/license": []byte("Two")},
		{"INDEX.json": []byte("Replacement")},
		{"../outside": []byte("Escape")},
		{"go/LICENSE:stream": []byte("Alternate stream")},
		{"go/LICENSE\\..\\outside": []byte("Windows separator")},
		{"go/CON.go.txt": []byte("Windows reserved name")},
		{"go/COM1.go.txt": []byte("Windows reserved device")},
		{"go/LICENSE-MIT.": []byte("Trailing dot")},
		{"go/LICENSE-MIT ": []byte("Trailing space")},
		{"go/LICENSE-?": []byte("Windows invalid character")},
	} {
		bundle := Bundle{Files: files}
		if err := bundle.Write(t.TempDir()); err == nil {
			t.Fatal("unsafe/colliding path accepted")
		}
	}
}

func TestNoticeWriterDoesNotOverwriteExistingFiles(t *testing.T) {
	root := t.TempDir()
	file := writeFixture(t, root, "go/LICENSE", "User material\n")
	bundle := Bundle{Files: map[string][]byte{"go/LICENSE": []byte("Replacement\n")}}
	if err := bundle.Write(root); err == nil {
		t.Fatal("existing material replaced")
	}
	contents, err := os.ReadFile(file)
	if err != nil || string(contents) != "User material\n" {
		t.Fatal("existing material lost", err)
	}
	if _, err := os.Stat(filepath.Join(root, "INDEX.json")); !os.IsNotExist(err) {
		t.Fatal("partial index published", err)
	}
}
