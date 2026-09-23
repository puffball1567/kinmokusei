package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecoratorContextHoverAndCompletion(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "decorators.km")
	uri := fileURI(path)
	input := `function Register(): (context:DecoratorContext)=>void {
  return (context) => {
    const kind = context.kind;
  };
}
@Register() class Service{}
`
	typePosition := positionOf(input, "DecoratorContext", 0)
	memberPosition := positionOf(input, "kind;", 0)
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, typePosition, ""),
		requestAt("textDocument/hover", 3, uri, memberPosition, ""),
		requestAt("textDocument/definition", 4, uri, typePosition, ""),
	)
	typeHover := messages[2]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(typeHover, "type DecoratorContext") || !strings.Contains(typeHover, "parameterIndex: int") || !strings.Contains(typeHover, "overrideChain: string[]") {
		t.Fatalf("type hover=%q", typeHover)
	}
	memberHover := messages[3]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(memberHover, "kind: string") {
		t.Fatalf("member hover=%q", memberHover)
	}
	if messages[4]["result"] != nil {
		t.Fatalf("built-in definition=%v", messages[4])
	}

	completion := strings.Replace(input, "context.kind", "context.", 1)
	position := positionOf(completion, "context.", 0)
	position.Character += len("context.")
	items := completionLabels(completionItemsAt(t, path, completion, position.Line, position.Character))
	for _, field := range []string{"kind", "identity", "classIdentity", "baseIdentity", "overrideChain", "className", "memberName", "parameterName", "parameterIndex", "static", "visibility", "valueType", "valueIdentity", "constructible", "constructUnavailableReason", "construct"} {
		if items[field] == nil {
			t.Errorf("missing %s completion: %#v", field, items)
		}
	}
	lexical := completionLabels(completionItemsAt(t, path, input, 0, 0))
	for _, name := range []string{"DecoratorContext", "ClassDecoratorContext", "FieldDecoratorContext", "ConstructorDecoratorContext", "MethodDecoratorContext", "GetterDecoratorContext", "SetterDecoratorContext", "ParameterDecoratorContext", "DecoratorValue"} {
		if lexical[name] == nil {
			t.Fatalf("missing %s built-in type completion: %#v", name, lexical)
		}
	}

	targetInput := strings.Replace(input, "DecoratorContext", "ClassDecoratorContext", 1)
	targetPosition := positionOf(targetInput, "ClassDecoratorContext", 0)
	targetMessages := serveMessages(t, openDocument(uri, targetInput),
		requestAt("textDocument/hover", 2, uri, targetPosition, ""),
	)
	targetHover := targetMessages[2]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(targetHover, "type ClassDecoratorContext") || !strings.Contains(targetHover, "classIdentity: string") {
		t.Fatalf("target context hover=%q", targetHover)
	}
}

func TestImportedDecoratorFactoryNavigationAndRefactor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	library := filepath.Join(root, "library.km")
	entry := filepath.Join(root, "entry.km")
	libraryText := `export function Route(path:string):(context:DecoratorContext)=>void{return (context)=>{};}`
	entryText := `import {Route} from "./library";
@Route("/users") export class Users{}`
	if err := os.WriteFile(library, []byte(libraryText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(entryText), 0o644); err != nil {
		t.Fatal(err)
	}
	application := positionOf(entryText, "Route", 1)
	messages := serveMessages(t, openDocument(fileURI(entry), entryText), openDocument(fileURI(library), libraryText),
		requestAt("textDocument/definition", 2, fileURI(entry), application, ""),
		requestAt("textDocument/hover", 3, fileURI(entry), application, ""),
		requestAt("textDocument/references", 4, fileURI(entry), application, `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 5, fileURI(entry), application, `"newName":"Endpoint"`),
	)
	definition, ok := messages[2]["result"].(map[string]any)
	if !ok || definition["uri"] != fileURI(library) {
		t.Fatalf("definition=%v", messages[2])
	}
	hover, ok := messages[3]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "Route(path: string)") {
		t.Fatalf("hover=%v", messages[3])
	}
	if references, ok := messages[4]["result"].([]any); !ok || len(references) != 3 {
		t.Fatalf("references=%v", messages[4])
	}
	rename, ok := messages[5]["result"].(map[string]any)
	if !ok {
		t.Fatalf("rename=%v", messages[5])
	}
	changes := rename["changes"].(map[string]any)
	if len(changes) != 2 || len(changes[fileURI(entry)].([]any)) != 2 || len(changes[fileURI(library)].([]any)) != 1 {
		t.Fatalf("changes=%v", changes)
	}

	signaturePosition := positionOf(entryText, `"/users"`, 0)
	label, _, _ := signatureResult(t, signatureHelpAt(t, entry, entryText, signaturePosition))
	if !strings.Contains(label, "Route(path: string)") {
		t.Fatalf("signature=%q", label)
	}
}
