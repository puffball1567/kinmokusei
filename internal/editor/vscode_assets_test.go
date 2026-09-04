package editor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/product"
)

func vscodeRoot(t *testing.T) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate VS Code asset test")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(current), "..", "..", "editors", "vscode"))
}

func readJSON(t *testing.T, name string, target any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(vscodeRoot(t), name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
}

func TestVSCodeManifestContract(t *testing.T) {
	var manifest struct {
		Name             string   `json:"name"`
		DisplayName      string   `json:"displayName"`
		Publisher        string   `json:"publisher"`
		Main             string   `json:"main"`
		ActivationEvents []string `json:"activationEvents"`
		Engines          struct {
			VSCode string `json:"vscode"`
		} `json:"engines"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		Scripts         map[string]string `json:"scripts"`
		Contributes     struct {
			Languages []struct {
				ID            string   `json:"id"`
				Extensions    []string `json:"extensions"`
				Configuration string   `json:"configuration"`
			} `json:"languages"`
			Grammars []struct {
				Language  string `json:"language"`
				ScopeName string `json:"scopeName"`
				Path      string `json:"path"`
			} `json:"grammars"`
			Commands []struct {
				Command string `json:"command"`
			} `json:"commands"`
			Configuration struct {
				Properties map[string]struct {
					Type    string `json:"type"`
					Default string `json:"default"`
					Scope   string `json:"scope"`
				} `json:"properties"`
			} `json:"configuration"`
		} `json:"contributes"`
	}
	readJSON(t, "package.json", &manifest)

	if manifest.Name != "kinmokusei" || manifest.DisplayName != product.DisplayName {
		t.Fatalf("manifest identity = %q/%q", manifest.Name, manifest.DisplayName)
	}
	if manifest.Publisher != "kinmokusei" {
		t.Fatalf("manifest publisher = %q, want kinmokusei", manifest.Publisher)
	}
	if manifest.Main != "./extension.js" || manifest.Engines.VSCode == "" {
		t.Fatalf("invalid entry point or VS Code engine: %#v", manifest)
	}
	if got := manifest.Dependencies["vscode-languageclient"]; got != "10.1.0" {
		t.Fatalf("vscode-languageclient = %q, want 10.1.0", got)
	}
	for name, version := range map[string]string{
		"@vscode/test-electron": "3.1.0",
		"@vscode/vsce":          "3.9.2",
	} {
		if got := manifest.DevDependencies[name]; got != version {
			t.Errorf("dev dependency %s = %q, want %s", name, got, version)
		}
	}
	for name, command := range map[string]string{
		"test:e2e":     "node test/e2e/run.js",
		"package:vsix": "node scripts/package-vsix.js",
	} {
		if got := manifest.Scripts[name]; got != command {
			t.Errorf("script %s = %q, want %q", name, got, command)
		}
	}
	if len(manifest.Contributes.Languages) != 1 {
		t.Fatalf("languages = %d, want 1", len(manifest.Contributes.Languages))
	}
	language := manifest.Contributes.Languages[0]
	if language.ID != "kinmokusei" ||
		len(language.Extensions) != 1 ||
		language.Extensions[0] != product.SourceExtension ||
		language.Configuration != "./language-configuration.json" {
		t.Fatalf("language contribution = %#v", language)
	}
	if len(manifest.Contributes.Grammars) != 1 ||
		manifest.Contributes.Grammars[0].Language != language.ID ||
		manifest.Contributes.Grammars[0].ScopeName != "source.kinmokusei" ||
		manifest.Contributes.Grammars[0].Path != "./syntaxes/kinmokusei.tmLanguage.json" {
		t.Fatalf("grammar contribution = %#v", manifest.Contributes.Grammars)
	}
	if len(manifest.Contributes.Commands) != 1 ||
		manifest.Contributes.Commands[0].Command != "kinmokusei.restartLanguageServer" {
		t.Fatalf("commands = %#v", manifest.Contributes.Commands)
	}
	server := manifest.Contributes.Configuration.Properties["kinmokusei.server.path"]
	if server.Type != "string" || server.Default != product.CommandName ||
		server.Scope != "machine-overridable" {
		t.Fatalf("server path configuration = %#v", server)
	}
	for _, event := range []string{
		"onLanguage:kinmokusei",
		"onCommand:kinmokusei.restartLanguageServer",
	} {
		if !contains(manifest.ActivationEvents, event) {
			t.Errorf("activation event %q is missing", event)
		}
	}
	for _, name := range []string{
		"extension.js",
		"client.js",
		"package-lock.json",
		"language-configuration.json",
		"syntaxes/kinmokusei.tmLanguage.json",
		"README.md",
		"scripts/package-vsix.js",
		"test/e2e/run.js",
		"test/e2e/suite/index.js",
		"test/e2e/fixture/main.km",
	} {
		if _, err := os.Stat(filepath.Join(vscodeRoot(t), name)); err != nil {
			t.Errorf("required asset %s: %v", name, err)
		}
	}
}

func TestVSCodeLanguageConfigurationContract(t *testing.T) {
	var configuration struct {
		Comments struct {
			Line  string   `json:"lineComment"`
			Block []string `json:"blockComment"`
		} `json:"comments"`
		Brackets         [][]string        `json:"brackets"`
		AutoClosingPairs []map[string]any  `json:"autoClosingPairs"`
		WordPattern      string            `json:"wordPattern"`
		IndentationRules map[string]string `json:"indentationRules"`
	}
	readJSON(t, "language-configuration.json", &configuration)
	if configuration.Comments.Line != "//" ||
		len(configuration.Comments.Block) != 2 ||
		configuration.Comments.Block[0] != "/*" ||
		configuration.Comments.Block[1] != "*/" {
		t.Fatalf("comment configuration = %#v", configuration.Comments)
	}
	if len(configuration.Brackets) != 3 || len(configuration.AutoClosingPairs) < 4 {
		t.Fatalf("incomplete bracket configuration")
	}
	if configuration.WordPattern == "" ||
		configuration.IndentationRules["increaseIndentPattern"] == "" ||
		configuration.IndentationRules["decreaseIndentPattern"] == "" {
		t.Fatalf("word or indentation configuration is incomplete")
	}
}

func TestVSCodeGrammarCoversLanguageVocabulary(t *testing.T) {
	var grammar map[string]any
	readJSON(t, "syntaxes/kinmokusei.tmLanguage.json", &grammar)
	if grammar["scopeName"] != "source.kinmokusei" {
		t.Fatalf("scopeName = %v", grammar["scopeName"])
	}
	fileTypes, ok := grammar["fileTypes"].([]any)
	if !ok || len(fileTypes) != 1 || fileTypes[0] != "yn" {
		t.Fatalf("fileTypes = %#v, want [yn]", grammar["fileTypes"])
	}
	encoded, err := json.Marshal(grammar)
	if err != nil {
		t.Fatal(err)
	}
	source := string(encoded)
	keywords := []string{
		"function", "const", "let", "return", "if", "else", "true", "false",
		"import", "from", "while", "for", "of", "select", "switch", "case",
		"default", "break", "continue", "goto", "fallthrough", "new", "class", "struct", "constructor", "public",
		"private", "static", "pointer", "this", "interface", "implements", "extends", "virtual", "override", "super", "go", "defer",
		"nil", "null", "as", "export", "try", "catch", "finally", "throw", "enum", "type", "alias", "distinct",
	}
	builtins := []string{
		"void", "boolean", "string", "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "float32",
		"float", "number", "float64", "byte", "error", "Map", "Result", "GoChannel",
		"GoSendChannel", "GoReceiveChannel", "len", "cap", "append", "copy",
		"delete", "clear", "min", "max", "makeSlice", "makeMap", "copyArray", "viewArray", "goChannel",
		"closeGoChannel", "ok", "fail",
	}
	for _, word := range append(keywords, builtins...) {
		if !strings.Contains(source, word) {
			t.Errorf("grammar does not contain %q", word)
		}
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
