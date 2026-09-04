package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRequestCancellationIDMatrix(t *testing.T) {
	for _, test := range []struct {
		name string
		id   string
	}{
		{"number", "2"},
		{"string", `"hover-request"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "cancel.km")
			uri := fileURI(path)
			text := `function value(input: int): int { return input; }`
			requestMessage := fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"method":"textDocument/hover","params":{"textDocument":{"uri":%q},"position":{"line":0,"character":10}}}`, test.id, uri)
			cancel := fmt.Sprintf(`{"jsonrpc":"2.0","method":"$/cancelRequest","params":{"id":%s}}`, test.id)
			messages, err := serveAsyncTest(t,
				func(ctx context.Context, _ request) { <-ctx.Done() },
				`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
				openDocument(uri, text),
				requestMessage,
				cancel,
				`{"jsonrpc":"2.0","id":90,"method":"shutdown"}`,
				`{"jsonrpc":"2.0","method":"exit"}`,
			)
			if err != nil {
				t.Fatal(err)
			}
			responses := responsesForRawID(messages, test.id)
			if len(responses) != 1 {
				t.Fatalf("responses for %s = %#v", test.id, responses)
			}
			if code := responses[0]["error"].(map[string]any)["code"]; code != float64(requestCancelledCode) {
				t.Fatalf("cancelled response = %#v", responses[0])
			}
		})
	}
}

func TestRequestIDValidationMatrix(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
		valid bool
	}{
		{`"request"`, "string:request", true},
		{`"re\u0071uest"`, "string:request", true},
		{"0", "number:0", true},
		{"-0", "number:0", true},
		{"-2147483648", "number:-2147483648", true},
		{"2147483647", "number:2147483647", true},
		{"null", "", false},
		{"{}", "", false},
		{"[]", "", false},
		{"1.5", "", false},
		{"2147483648", "", false},
	} {
		got, valid := requestIDKey([]byte(test.input))
		if got != test.want || valid != test.valid {
			t.Errorf("requestIDKey(%s) = %q, %v; want %q, %v", test.input, got, valid, test.want, test.valid)
		}
	}
}

func TestUnknownAndMalformedCancellationAreIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ignored_cancel.km")
	uri := fileURI(path)
	text := `function value(input: int): int { return input; }`
	hover := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":%q},"position":{"line":0,"character":10}}}`, uri)
	messages, err := serveAsyncTest(t, nil,
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
		openDocument(uri, text),
		`{"jsonrpc":"2.0","method":"$/cancelRequest","params":{"id":999}}`,
		`{"jsonrpc":"2.0","method":"$/cancelRequest","params":{}}`,
		`{"jsonrpc":"2.0","method":"$/cancelRequest","params":"invalid"}`,
		hover,
		`{"jsonrpc":"2.0","id":90,"method":"shutdown"}`,
		`{"jsonrpc":"2.0","method":"exit"}`,
	)
	if err != nil {
		t.Fatal(err)
	}
	responses := responsesForRawID(messages, "2")
	if len(responses) != 1 || responses[0]["error"] != nil || responses[0]["result"] == nil {
		t.Fatalf("hover response = %#v", responses)
	}
	if responses := responsesForRawID(messages, "999"); len(responses) != 0 {
		t.Fatalf("unknown cancellation produced responses: %#v", responses)
	}
}

func TestDocumentChangeSuppressesStaleRequestResult(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stale.km")
	uri := fileURI(path)
	initial := `function value(input: int): int { return input; }`
	updated := `function value(input: int): int { return input + 1; }`
	hover := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":%q},"position":{"line":0,"character":10}}}`, uri)
	updatedHover := strings.Replace(hover, `"id":2`, `"id":3`, 1)
	change := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":%q,"version":2},"contentChanges":[{"text":%q}]}}`, uri, updated)

	var server *Server
	before := func(ctx context.Context, _ request) {
		for {
			server.requestMu.Lock()
			generation := server.generation
			server.requestMu.Unlock()
			if generation >= 2 {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Millisecond):
			}
		}
	}
	var output bytes.Buffer
	server = newServer(strings.NewReader(framed(
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
		openDocument(uri, initial), hover, change, updatedHover,
		`{"jsonrpc":"2.0","id":90,"method":"shutdown"}`,
		`{"jsonrpc":"2.0","method":"exit"}`,
	)), &output)
	server.beforeRequest = before
	if err := server.serve(); err != nil {
		t.Fatal(err)
	}
	responses := responsesForRawID(decodeMessages(t, output.String()), "2")
	if len(responses) != 1 {
		t.Fatalf("stale responses = %#v", responses)
	}
	if code := responses[0]["error"].(map[string]any)["code"]; code != float64(contentModifiedCode) {
		t.Fatalf("stale response = %#v", responses[0])
	}
	updatedResponses := responsesForRawID(decodeMessages(t, output.String()), "3")
	if len(updatedResponses) != 1 || updatedResponses[0]["error"] != nil || updatedResponses[0]["result"] == nil {
		t.Fatalf("updated snapshot response = %#v", updatedResponses)
	}
}

func TestShutdownDrainsAcceptedRequestsBeforeResponding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shutdown.km")
	uri := fileURI(path)
	text := `function value(input: int): int { return input; }`
	hover := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":%q},"position":{"line":0,"character":10}}}`, uri)
	messages, err := serveAsyncTest(t,
		func(context.Context, request) { time.Sleep(5 * time.Millisecond) },
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
		openDocument(uri, text), hover,
		`{"jsonrpc":"2.0","id":90,"method":"shutdown"}`,
		`{"jsonrpc":"2.0","method":"exit"}`,
	)
	if err != nil {
		t.Fatal(err)
	}
	requestIndex, shutdownIndex := -1, -1
	for index, message := range messages {
		switch message["id"] {
		case float64(2):
			requestIndex = index
		case float64(90):
			shutdownIndex = index
		}
	}
	if requestIndex < 0 || shutdownIndex < 0 || requestIndex >= shutdownIndex {
		t.Fatalf("request/shutdown ordering = %d/%d messages=%#v", requestIndex, shutdownIndex, messages)
	}
}

func serveAsyncTest(t *testing.T, before func(context.Context, request), messages ...string) ([]map[string]any, error) {
	t.Helper()
	var output bytes.Buffer
	server := newServer(strings.NewReader(framed(messages...)), &output)
	server.beforeRequest = before
	err := server.serve()
	if err != nil {
		return nil, err
	}
	return decodeMessages(t, output.String()), nil
}

func responsesForRawID(messages []map[string]any, rawID string) []map[string]any {
	var want any
	if err := json.Unmarshal([]byte(rawID), &want); err != nil {
		return nil
	}
	var responses []map[string]any
	for _, message := range messages {
		if message["id"] == want {
			responses = append(responses, message)
		}
	}
	return responses
}
