package lsp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/puffball1567/kinmokusei/internal/compiler"
	"github.com/puffball1567/kinmokusei/internal/diagnostic"
	"github.com/puffball1567/kinmokusei/internal/product"
	"github.com/puffball1567/kinmokusei/internal/source"
)

const maxMessageSize = 16 << 20

const (
	requestCancelledCode = -32800
	contentModifiedCode  = -32801
)

var ErrExitWithoutShutdown = errors.New("LSP client exited before shutdown")

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *responseError  `json:"error,omitempty"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type protocolRange struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

type protocolDiagnostic struct {
	Range    protocolRange `json:"range"`
	Severity int           `json:"severity"`
	Source   string        `json:"source"`
	Message  string        `json:"message"`
}

type contentChange struct {
	Range       *protocolRange `json:"range,omitempty"`
	RangeLength *int           `json:"rangeLength,omitempty"`
	Text        string         `json:"text"`
}

func (c *contentChange) UnmarshalJSON(data []byte) error {
	var wire struct {
		Range       *protocolRange `json:"range"`
		RangeLength *int           `json:"rangeLength"`
		Text        *string        `json:"text"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if wire.Text == nil {
		return fmt.Errorf("content change is missing text")
	}
	c.Range, c.RangeLength, c.Text = wire.Range, wire.RangeLength, *wire.Text
	return nil
}

type document struct {
	URI     string
	Path    string
	Text    string
	Version int
}

type pendingRequest struct {
	id         json.RawMessage
	key        string
	generation uint64
	cancel     context.CancelFunc
}

type Server struct {
	reader      *bufio.Reader
	writer      io.Writer
	writeMu     sync.Mutex
	documents   map[string]document
	diagnostics map[string]map[string][]protocolDiagnostic
	initialized bool
	shutdown    bool

	requestMu       sync.Mutex
	pendingRequests map[string]*pendingRequest
	generation      uint64
	workers         sync.WaitGroup
	asyncErrMu      sync.Mutex
	asyncErr        error
	beforeRequest   func(context.Context, request)
}

func Serve(reader io.Reader, writer io.Writer) error {
	return newServer(reader, writer).serve()
}

func newServer(reader io.Reader, writer io.Writer) *Server {
	return &Server{
		reader:          bufio.NewReader(reader),
		writer:          writer,
		documents:       map[string]document{},
		diagnostics:     map[string]map[string][]protocolDiagnostic{},
		pendingRequests: map[string]*pendingRequest{},
	}
}

func (s *Server) serve() error {
	for {
		payload, err := readMessage(s.reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				s.cancelPendingRequests()
				s.workers.Wait()
				return s.asyncError()
			}
			s.cancelPendingRequests()
			s.workers.Wait()
			return err
		}
		var message request
		if err = json.Unmarshal(payload, &message); err != nil {
			if writeErr := s.writeResponse(response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &responseError{Code: -32700, Message: "parse error"}}); writeErr != nil {
				return writeErr
			}
			continue
		}
		if message.JSONRPC != "2.0" || message.Method == "" {
			if len(message.ID) != 0 {
				if err = s.writeResponse(response{JSONRPC: "2.0", ID: message.ID, Error: &responseError{Code: -32600, Message: "invalid request"}}); err != nil {
					return err
				}
			}
			continue
		}
		if len(message.ID) != 0 {
			if _, valid := requestIDKey(message.ID); !valid {
				if err = s.writeResponse(response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &responseError{Code: -32600, Message: "invalid request id"}}); err != nil {
					return err
				}
				continue
			}
		}
		stop, handleErr := s.handle(message)
		if handleErr != nil {
			s.cancelPendingRequests()
			s.workers.Wait()
			return handleErr
		}
		if stop {
			s.cancelPendingRequests()
			s.workers.Wait()
			return s.asyncError()
		}
	}
}

func (s *Server) handle(message request) (bool, error) {
	isRequest := len(message.ID) != 0
	if message.Method == "$/cancelRequest" {
		if isRequest {
			return false, s.writeResponse(response{JSONRPC: "2.0", ID: message.ID, Error: &responseError{Code: -32600, Message: "$/cancelRequest must be a notification"}})
		}
		return false, s.cancelRequest(message.Params)
	}
	if !s.initialized && message.Method != "initialize" && message.Method != "exit" {
		if isRequest {
			return false, s.writeResponse(response{JSONRPC: "2.0", ID: message.ID, Error: &responseError{Code: -32002, Message: "server not initialized"}})
		}
		return false, nil
	}
	if s.shutdown && message.Method != "exit" {
		if isRequest {
			return false, s.writeResponse(response{JSONRPC: "2.0", ID: message.ID, Error: &responseError{Code: -32600, Message: "server has shut down"}})
		}
		return false, nil
	}
	switch message.Method {
	case "initialize":
		if !isRequest || s.initialized {
			if isRequest {
				return false, s.writeResponse(response{JSONRPC: "2.0", ID: message.ID, Error: &responseError{Code: -32600, Message: "initialize may be requested only once"}})
			}
			return false, nil
		}
		s.initialized = true
		result := map[string]any{
			"capabilities": map[string]any{
				"positionEncoding":       "utf-16",
				"textDocumentSync":       map[string]any{"openClose": true, "change": 2},
				"hoverProvider":          true,
				"definitionProvider":     true,
				"referencesProvider":     true,
				"renameProvider":         map[string]any{"prepareProvider": true},
				"signatureHelpProvider":  map[string]any{"triggerCharacters": []string{"(", ","}, "retriggerCharacters": []string{","}},
				"documentSymbolProvider": true,
				"completionProvider":     map[string]any{"resolveProvider": false, "triggerCharacters": []string{"."}},
			},
			"serverInfo": map[string]any{"name": product.DisplayName},
		}
		return false, s.writeResponse(response{JSONRPC: "2.0", ID: message.ID, Result: result})
	case "initialized":
		return false, nil
	case "shutdown":
		if !isRequest {
			return false, nil
		}
		s.shutdown = true
		s.workers.Wait()
		if err := s.asyncError(); err != nil {
			return false, err
		}
		return false, s.writeResponse(response{JSONRPC: "2.0", ID: message.ID, Result: json.RawMessage("null")})
	case "exit":
		if !s.shutdown {
			return true, ErrExitWithoutShutdown
		}
		return true, nil
	case "textDocument/didOpen":
		return false, s.didOpen(message.Params)
	case "textDocument/didChange":
		return false, s.didChange(message.Params)
	case "textDocument/didClose":
		return false, s.didClose(message.Params)
	case "textDocument/hover":
		if isRequest {
			return false, s.startRequest(message)
		}
		return false, nil
	case "textDocument/definition":
		if isRequest {
			return false, s.startRequest(message)
		}
		return false, nil
	case "textDocument/references":
		if isRequest {
			return false, s.startRequest(message)
		}
		return false, nil
	case "textDocument/prepareRename":
		if isRequest {
			return false, s.startRequest(message)
		}
		return false, nil
	case "textDocument/rename":
		if isRequest {
			return false, s.startRequest(message)
		}
		return false, nil
	case "textDocument/signatureHelp":
		if isRequest {
			return false, s.startRequest(message)
		}
		return false, nil
	case "textDocument/documentSymbol":
		if isRequest {
			return false, s.startRequest(message)
		}
		return false, nil
	case "textDocument/completion":
		if isRequest {
			return false, s.startRequest(message)
		}
		return false, nil
	default:
		if isRequest {
			return false, s.writeResponse(response{JSONRPC: "2.0", ID: message.ID, Error: &responseError{Code: -32601, Message: "method not found"}})
		}
		return false, nil
	}
}

func (s *Server) startRequest(message request) error {
	key, _ := requestIDKey(message.ID)
	ctx, cancel := context.WithCancel(context.Background())
	pending := &pendingRequest{id: append(json.RawMessage(nil), message.ID...), key: key, cancel: cancel}
	snapshot := s.requestSnapshot()
	var output bytes.Buffer
	snapshot.writer = &output

	s.requestMu.Lock()
	if _, exists := s.pendingRequests[key]; exists {
		s.requestMu.Unlock()
		cancel()
		return s.writeResponse(response{JSONRPC: "2.0", ID: message.ID, Error: &responseError{Code: -32600, Message: "duplicate request id"}})
	}
	pending.generation = s.generation
	s.pendingRequests[key] = pending
	s.workers.Add(1)
	s.requestMu.Unlock()

	go func() {
		defer s.workers.Done()
		if s.beforeRequest != nil {
			s.beforeRequest(ctx, message)
		}
		if ctx.Err() != nil {
			return
		}
		err := snapshot.handleRequest(message)
		s.finishRequest(pending, output.Bytes(), err)
	}()
	return nil
}

func (s *Server) handleRequest(message request) error {
	switch message.Method {
	case "textDocument/hover":
		return s.hover(message.ID, message.Params)
	case "textDocument/definition":
		return s.definition(message.ID, message.Params)
	case "textDocument/references":
		return s.references(message.ID, message.Params)
	case "textDocument/prepareRename":
		return s.prepareRename(message.ID, message.Params)
	case "textDocument/rename":
		return s.rename(message.ID, message.Params)
	case "textDocument/signatureHelp":
		return s.signatureHelp(message.ID, message.Params)
	case "textDocument/documentSymbol":
		return s.documentSymbols(message.ID, message.Params)
	case "textDocument/completion":
		return s.completion(message.ID, message.Params)
	default:
		return s.writeResponse(response{JSONRPC: "2.0", ID: message.ID, Error: &responseError{Code: -32601, Message: "method not found"}})
	}
}

func (s *Server) finishRequest(pending *pendingRequest, framed []byte, requestErr error) {
	s.requestMu.Lock()
	current := s.pendingRequests[pending.key]
	if current != pending {
		s.requestMu.Unlock()
		return
	}
	delete(s.pendingRequests, pending.key)
	stale := pending.generation != s.generation
	s.requestMu.Unlock()
	pending.cancel()

	if requestErr != nil {
		s.recordAsyncError(requestErr)
		return
	}
	if stale {
		requestErr = s.writeResponse(response{JSONRPC: "2.0", ID: pending.id, Error: &responseError{Code: contentModifiedCode, Message: "document changed while request was running"}})
	} else if len(framed) == 0 {
		requestErr = s.writeResponse(response{JSONRPC: "2.0", ID: pending.id, Error: &responseError{Code: -32603, Message: "request produced no response"}})
	} else {
		requestErr = s.writeFramed(framed)
	}
	if requestErr != nil {
		s.recordAsyncError(requestErr)
	}
}

func (s *Server) cancelRequest(raw json.RawMessage) error {
	var params struct {
		ID json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(raw, &params); err != nil || len(params.ID) == 0 {
		return nil
	}
	key, valid := requestIDKey(params.ID)
	if !valid {
		return nil
	}
	s.requestMu.Lock()
	pending := s.pendingRequests[key]
	if pending != nil {
		delete(s.pendingRequests, key)
	}
	s.requestMu.Unlock()
	if pending == nil {
		return nil
	}
	pending.cancel()
	return s.writeResponse(response{JSONRPC: "2.0", ID: pending.id, Error: &responseError{Code: requestCancelledCode, Message: "request cancelled"}})
}

func (s *Server) cancelPendingRequests() {
	s.requestMu.Lock()
	pending := make([]*pendingRequest, 0, len(s.pendingRequests))
	for key, request := range s.pendingRequests {
		delete(s.pendingRequests, key)
		pending = append(pending, request)
	}
	s.requestMu.Unlock()
	for _, request := range pending {
		request.cancel()
	}
}

func (s *Server) requestSnapshot() *Server {
	documents := make(map[string]document, len(s.documents))
	for uri, doc := range s.documents {
		documents[uri] = doc
	}
	diagnostics := make(map[string]map[string][]protocolDiagnostic, len(s.diagnostics))
	for rootURI, byURI := range s.diagnostics {
		copyByURI := make(map[string][]protocolDiagnostic, len(byURI))
		for uri, items := range byURI {
			copyByURI[uri] = append([]protocolDiagnostic(nil), items...)
		}
		diagnostics[rootURI] = copyByURI
	}
	return &Server{
		documents: documents, diagnostics: diagnostics,
		initialized: s.initialized, shutdown: s.shutdown,
		pendingRequests: map[string]*pendingRequest{},
	}
}

func requestIDKey(id json.RawMessage) (string, bool) {
	decoder := json.NewDecoder(strings.NewReader(string(id)))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return "", false
	}
	switch value := decoded.(type) {
	case string:
		return "string:" + value, true
	case json.Number:
		integer, err := strconv.ParseInt(value.String(), 10, 64)
		if err != nil || integer < -2147483648 || integer > 2147483647 {
			return "", false
		}
		return "number:" + strconv.FormatInt(integer, 10), true
	default:
		return "", false
	}
}

func (s *Server) advanceGeneration() {
	s.requestMu.Lock()
	s.generation++
	s.requestMu.Unlock()
}

func (s *Server) recordAsyncError(err error) {
	s.asyncErrMu.Lock()
	if s.asyncErr == nil {
		s.asyncErr = err
	}
	s.asyncErrMu.Unlock()
}

func (s *Server) asyncError() error {
	s.asyncErrMu.Lock()
	defer s.asyncErrMu.Unlock()
	return s.asyncErr
}

func (s *Server) didOpen(raw json.RawMessage) error {
	var params struct {
		TextDocument struct {
			URI     string `json:"uri"`
			Version int    `json:"version"`
			Text    string `json:"text"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil
	}
	path, err := filePath(params.TextDocument.URI)
	if err != nil {
		return nil
	}
	s.advanceGeneration()
	s.documents[params.TextDocument.URI] = document{URI: params.TextDocument.URI, Path: path, Text: params.TextDocument.Text, Version: params.TextDocument.Version}
	return s.publishFor(params.TextDocument.URI)
}

func (s *Server) didChange(raw json.RawMessage) error {
	var params struct {
		TextDocument struct {
			URI     string `json:"uri"`
			Version int    `json:"version"`
		} `json:"textDocument"`
		ContentChanges []contentChange `json:"contentChanges"`
	}
	if err := json.Unmarshal(raw, &params); err != nil || len(params.ContentChanges) == 0 {
		return nil
	}
	doc, ok := s.documents[params.TextDocument.URI]
	if !ok || params.TextDocument.Version <= doc.Version {
		return nil
	}
	updated, err := applyContentChanges(doc.Text, params.ContentChanges)
	if err != nil {
		return nil
	}
	doc.Text = updated
	doc.Version = params.TextDocument.Version
	s.advanceGeneration()
	s.documents[params.TextDocument.URI] = doc
	return s.publishFor(params.TextDocument.URI)
}

func applyContentChanges(text string, changes []contentChange) (string, error) {
	updated := text
	for index, change := range changes {
		if change.Range == nil {
			if change.RangeLength != nil {
				return "", fmt.Errorf("change %d has rangeLength without a range", index)
			}
			updated = change.Text
			continue
		}
		start, startErr := byteOffsetAtPosition(updated, change.Range.Start)
		if startErr != nil {
			return "", fmt.Errorf("change %d has invalid start position: %w", index, startErr)
		}
		end, endErr := byteOffsetAtPosition(updated, change.Range.End)
		if endErr != nil {
			return "", fmt.Errorf("change %d has invalid end position: %w", index, endErr)
		}
		if start > end {
			return "", fmt.Errorf("change %d range starts after it ends", index)
		}
		if change.RangeLength != nil {
			actual := utf16Length(updated[start:end])
			if *change.RangeLength != actual {
				return "", fmt.Errorf("change %d rangeLength is %d, want %d", index, *change.RangeLength, actual)
			}
		}
		updated = updated[:start] + change.Text + updated[end:]
	}
	return updated, nil
}

func (s *Server) didClose(raw json.RawMessage) error {
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil
	}
	if _, open := s.documents[params.TextDocument.URI]; !open {
		if _, diagnosed := s.diagnostics[params.TextDocument.URI]; !diagnosed {
			return nil
		}
	}
	s.advanceGeneration()
	delete(s.documents, params.TextDocument.URI)
	previous := s.diagnostics[params.TextDocument.URI]
	delete(s.diagnostics, params.TextDocument.URI)
	affected := map[string]bool{params.TextDocument.URI: true}
	for uri := range previous {
		affected[uri] = true
	}
	for uri := range affected {
		if err := s.publish(uri, s.combinedDiagnostics(uri)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) publishFor(rootURI string) error {
	root, ok := s.documents[rootURI]
	if !ok {
		return nil
	}
	overlay := make(map[string]string, len(s.documents))
	for _, doc := range s.documents {
		overlay[doc.Path] = doc.Text
	}
	result, checkErr := compiler.CheckFilesWithOverlay([]string{root.Path}, overlay)
	grouped := map[string][]protocolDiagnostic{}
	if checkErr != nil {
		grouped[rootURI] = []protocolDiagnostic{{Range: protocolRange{}, Severity: 1, Source: product.CommandName, Message: checkErr.Error()}}
	} else {
		for _, item := range result.Diagnostics {
			uri := rootURI
			text := root.Text
			if item.Span.Path != "" {
				if absolute, err := filepath.Abs(item.Span.Path); err == nil {
					absolute = filepath.Clean(absolute)
					uri = pathURI(absolute)
					if open, exists := s.documentByPath(absolute); exists {
						text = open.Text
					} else if contents, readErr := os.ReadFile(absolute); readErr == nil {
						text = string(contents)
					}
				}
			}
			grouped[uri] = append(grouped[uri], toProtocolDiagnostic(item, text))
		}
	}
	if _, exists := grouped[rootURI]; !exists {
		grouped[rootURI] = nil
	}
	previous := s.diagnostics[rootURI]
	s.diagnostics[rootURI] = grouped
	affected := map[string]bool{}
	for uri := range previous {
		affected[uri] = true
	}
	for uri := range grouped {
		affected[uri] = true
	}
	for uri := range affected {
		if err := s.publish(uri, s.combinedDiagnostics(uri)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) combinedDiagnostics(uri string) []protocolDiagnostic {
	var combined []protocolDiagnostic
	for _, byURI := range s.diagnostics {
		combined = append(combined, byURI[uri]...)
	}
	return combined
}

func (s *Server) documentByPath(path string) (document, bool) {
	for _, doc := range s.documents {
		if filepath.Clean(doc.Path) == path {
			return doc, true
		}
	}
	return document{}, false
}

func (s *Server) publish(uri string, diagnostics []protocolDiagnostic) error {
	if diagnostics == nil {
		diagnostics = []protocolDiagnostic{}
	}
	return s.write(notification{JSONRPC: "2.0", Method: "textDocument/publishDiagnostics", Params: map[string]any{"uri": uri, "diagnostics": diagnostics}})
}

func toProtocolDiagnostic(item diagnostic.Diagnostic, text string) protocolDiagnostic {
	start := protocolPosition(text, item.Span.Start)
	end := protocolPosition(text, item.Span.End)
	if end.Line < start.Line || (end.Line == start.Line && end.Character < start.Character) {
		end = start
	}
	return protocolDiagnostic{Range: protocolRange{Start: start, End: end}, Severity: 1, Source: product.CommandName, Message: item.Message}
}

func protocolPosition(text string, fallback source.Position) position {
	offset := fallback.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > len(text) {
		offset = len(text)
	}
	line, character, current := 0, 0, 0
	for current < offset {
		r, size := utf8.DecodeRuneInString(text[current:offset])
		if r == '\r' {
			current += size
			if current < offset && text[current] == '\n' {
				current++
			}
			line, character = line+1, 0
			continue
		}
		if r == '\n' {
			line, character, current = line+1, 0, current+size
			continue
		}
		if r > 0xffff {
			character += 2
		} else {
			character++
		}
		current += size
	}
	return position{Line: line, Character: character}
}

func filePath(uri string) (string, error) {
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != "file" || (parsed.Host != "" && parsed.Host != "localhost") {
		return "", fmt.Errorf("unsupported document URI %q", uri)
	}
	path, err := url.PathUnescape(parsed.EscapedPath())
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" && len(path) >= 3 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}
	return filepath.Clean(filepath.FromSlash(path)), nil
}

func pathURI(path string) string {
	slashPath := filepath.ToSlash(path)
	if filepath.VolumeName(path) != "" && !strings.HasPrefix(slashPath, "/") {
		slashPath = "/" + slashPath
	}
	return (&url.URL{Scheme: "file", Path: slashPath}).String()
}

func readMessage(reader *bufio.Reader) ([]byte, error) {
	contentLength := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("invalid LSP header %q", line)
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			parsed, parseErr := strconv.Atoi(strings.TrimSpace(value))
			if parseErr != nil || parsed < 0 || parsed > maxMessageSize {
				return nil, fmt.Errorf("invalid LSP Content-Length %q", strings.TrimSpace(value))
			}
			contentLength = parsed
		}
	}
	if contentLength < 0 {
		return nil, errors.New("LSP message is missing Content-Length")
	}
	payload := make([]byte, contentLength)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (s *Server) writeResponse(message response) error { return s.write(message) }

func (s *Server) write(message any) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	var framed bytes.Buffer
	fmt.Fprintf(&framed, "Content-Length: %d\r\n\r\n", len(payload))
	framed.Write(payload)
	return s.writeFramed(framed.Bytes())
}

func (s *Server) writeFramed(framed []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	written, err := s.writer.Write(framed)
	if err == nil && written != len(framed) {
		return io.ErrShortWrite
	}
	return err
}
