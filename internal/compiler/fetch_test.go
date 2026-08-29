package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchLibraryMatchesIndependentGo(t *testing.T) {
	temp := t.TempDir()
	entry := filepath.Join(temp, "entry.otm")
	entrySource := `
import { Response, fetch, fetchLimited, send, sendWith } from "ontama/http";
import go context from "context";
import go http from "net/http";

function load(ctx: context.Context, url: string): Result<Response> {
  const task: Task<Result<Response>> = go fetch(ctx, url);
  const response = await task?;
  return ok(response);
}
function loadLimited(ctx: context.Context, url: string, maxResponseBytes: int64): Result<Response> {
  const task = go fetchLimited(ctx, url, maxResponseBytes);
  const response = await task?;
  return ok(response);
}
function execute(request: *http.Request, maxResponseBytes: int64): Result<Response> {
  return send(request, maxResponseBytes);
}
function executeWith(client: *http.Client, request: *http.Request, maxResponseBytes: int64): Result<Response> {
  return sendWith(client, request, maxResponseBytes);
}
`
	if err := os.WriteFile(entry, []byte(entrySource), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "fetchclient")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v\n%s", err, diagnostics, generated)
	}
	for _, want := range []string{"http.NewRequestWithContext", "io.LimitReader", ".Header.Clone()", "string(this.body)", "go func()"} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated fetch library does not contain %q:\n%s", want, generated)
		}
	}

	referenceSource := `package reference
import (
  "context"
  "errors"
  "io"
  "net/http"
)
const defaultMaxResponseBytes int64 = 4 * 1024 * 1024
const largestResponseLimit int64 = 9223372036854775807
type Response struct {
  Status int
  Headers http.Header
  body []byte
}
func (response Response) Ok() bool { return response.Status >= 200 && response.Status < 300 }
func (response Response) Header(name string) string { return response.Headers.Get(name) }
func (response Response) Text() string { return string(response.body) }
func (response Response) Bytes() []byte { return append([]byte(nil), response.body...) }
func SendWith(client *http.Client, request *http.Request, maxResponseBytes int64) (Response, error) {
  if client == nil { return Response{}, errors.New("fetch client must not be nil") }
  if request == nil { return Response{}, errors.New("fetch request must not be nil") }
  if maxResponseBytes < 0 { return Response{}, errors.New("fetch response limit must not be negative") }
  response, err := client.Do(request)
  if err != nil { return Response{}, err }
  defer response.Body.Close()
  readLimit := maxResponseBytes
  if maxResponseBytes < largestResponseLimit { readLimit++ }
  body, err := io.ReadAll(io.LimitReader(response.Body, readLimit))
  if err != nil { return Response{}, err }
  if int64(len(body)) > maxResponseBytes { return Response{}, errors.New("fetch response exceeds the configured byte limit") }
  return Response{Status: response.StatusCode, Headers: response.Header.Clone(), body: body}, nil
}
func Send(request *http.Request, maxResponseBytes int64) (Response, error) {
  return SendWith(http.DefaultClient, request, maxResponseBytes)
}
func FetchLimited(ctx context.Context, url string, maxResponseBytes int64) (Response, error) {
  request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
  if err != nil { return Response{}, err }
  return Send(request, maxResponseBytes)
}
func Fetch(ctx context.Context, url string) (Response, error) {
  return FetchLimited(ctx, url, defaultMaxResponseBytes)
}
type taskResult struct { response Response; err error; panicValue any }
func Load(ctx context.Context, url string) (Response, error) {
  done := make(chan taskResult, 1)
  go func() {
    result := taskResult{}
    defer func() { result.panicValue = recover(); done <- result }()
    result.response, result.err = Fetch(ctx, url)
  }()
  result := <-done
  if result.panicValue != nil { panic(result.panicValue) }
  return result.response, result.err
}
func LoadLimited(ctx context.Context, url string, maxResponseBytes int64) (Response, error) {
  done := make(chan taskResult, 1)
  go func() {
    result := taskResult{}
    defer func() { result.panicValue = recover(); done <- result }()
    result.response, result.err = FetchLimited(ctx, url, maxResponseBytes)
  }()
  result := <-done
  if result.panicValue != nil { panic(result.panicValue) }
  return result.response, result.err
}
`
	testSource := `package fetchclient
import (
  "context"
  "errors"
  "fmt"
  "io"
  "net/http"
  "net/http/httptest"
  "strings"
  "sync"
  "sync/atomic"
  "testing"
  reference "fetch.test/reference"
)
func sameError(left, right error) bool {
  if left == nil || right == nil { return left == nil && right == nil }
  return left.Error() == right.Error()
}
func assertResponse(t *testing.T, gotStatus int, gotOK bool, gotHeader, gotText string, gotBytes []byte, want reference.Response) {
  t.Helper()
  if gotStatus != want.Status || gotOK != want.Ok() || gotHeader != want.Header("X-Test") || gotText != want.Text() || string(gotBytes) != string(want.Bytes()) {
    t.Errorf("OnsenTamago=(%d,%v,%q,%q,%v), Go=(%d,%v,%q,%q,%v)", gotStatus, gotOK, gotHeader, gotText, gotBytes, want.Status, want.Ok(), want.Header("X-Test"), want.Text(), want.Bytes())
  }
}
func TestFetchResponseMatrix(t *testing.T) {
  server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
    writer.Header().Set("X-Test", "first")
    writer.Header().Add("X-Test", "second")
    switch request.URL.Path {
    case "/ok": writer.Header().Set("Content-Type", "application/json"); writer.WriteHeader(http.StatusOK); _, _ = io.WriteString(writer, ` + "`" + `{"message":"温泉"}` + "`" + `)
    case "/teapot": writer.WriteHeader(http.StatusTeapot); _, _ = io.WriteString(writer, "short")
    case "/empty": writer.WriteHeader(http.StatusNoContent)
    case "/large": _, _ = io.WriteString(writer, "12345")
    case "/echo": _, _ = io.WriteString(writer, request.Method+":"+request.Header.Get("X-Request")+":"); _, _ = io.Copy(writer, request.Body)
    default: http.NotFound(writer, request)
    }
  }))
  defer server.Close()
  for _, path := range []string{"/ok", "/teapot", "/empty", "/missing"} {
    got, gotErr := load(context.Background(), server.URL+path)
    want, wantErr := reference.Load(context.Background(), server.URL+path)
    if !sameError(gotErr, wantErr) { t.Fatalf("%s errors: OnsenTamago=%v Go=%v", path, gotErr, wantErr) }
    if gotErr == nil { assertResponse(t, got.Status, got.Ok(), got.Header("X-Test"), got.Text(), got.Bytes(), want) }
  }
  got, gotErr := loadLimited(context.Background(), server.URL+"/large", 5)
  want, wantErr := reference.LoadLimited(context.Background(), server.URL+"/large", 5)
  if !sameError(gotErr, wantErr) { t.Fatalf("exact limit errors: OnsenTamago=%v Go=%v", gotErr, wantErr) }
  assertResponse(t, got.Status, got.Ok(), got.Header("X-Test"), got.Text(), got.Bytes(), want)
  gotMaximum, gotErr := loadLimited(context.Background(), server.URL+"/large", 9223372036854775807)
  wantMaximum, wantErr := reference.LoadLimited(context.Background(), server.URL+"/large", 9223372036854775807)
  if !sameError(gotErr, wantErr) || gotMaximum.Text() != wantMaximum.Text() || gotMaximum.Text() != "12345" { t.Errorf("maximum limit: OnsenTamago=(%q,%v) Go=(%q,%v)", gotMaximum.Text(), gotErr, wantMaximum.Text(), wantErr) }
  for _, limit := range []int64{4, 0, -1} {
    _, gotErr := loadLimited(context.Background(), server.URL+"/large", limit)
    _, wantErr := reference.LoadLimited(context.Background(), server.URL+"/large", limit)
    if !sameError(gotErr, wantErr) { t.Errorf("limit %d errors: OnsenTamago=%v Go=%v", limit, gotErr, wantErr) }
  }
  gotEmpty, gotErr := loadLimited(context.Background(), server.URL+"/empty", 0)
  wantEmpty, wantErr := reference.LoadLimited(context.Background(), server.URL+"/empty", 0)
  if !sameError(gotErr, wantErr) || gotEmpty.Text() != wantEmpty.Text() { t.Errorf("zero limit empty: OnsenTamago=(%q,%v) Go=(%q,%v)", gotEmpty.Text(), gotErr, wantEmpty.Text(), wantErr) }

  generatedRequest, _ := http.NewRequest(http.MethodPost, server.URL+"/echo", strings.NewReader("payload"))
  generatedRequest.Header.Set("X-Request", "header")
  goRequest, _ := http.NewRequest(http.MethodPost, server.URL+"/echo", strings.NewReader("payload"))
  goRequest.Header.Set("X-Request", "header")
  gotPost, gotErr := execute(generatedRequest, 100)
  wantPost, wantErr := reference.Send(goRequest, 100)
  if !sameError(gotErr, wantErr) || gotPost.Text() != wantPost.Text() || gotPost.Text() != "POST:header:payload" { t.Errorf("custom request: OnsenTamago=(%q,%v) Go=(%q,%v)", gotPost.Text(), gotErr, wantPost.Text(), wantErr) }

  first := gotPost.Bytes()
  first[0] = 'X'
  second := gotPost.Bytes()
  if gotPost.Text() != "POST:header:payload" || second[0] != 'P' { t.Errorf("Bytes must return an independent copy: text=%q bytes=%q", gotPost.Text(), second) }
}
func TestFetchValidationCancellationAndClose(t *testing.T) {
  request, _ := http.NewRequest(http.MethodGet, "http://example.invalid", nil)
  matrices := []struct { generated func() error; goReference func() error }{
    {func() error { _, err := executeWith(nil, nil, -1); return err }, func() error { _, err := reference.SendWith(nil, nil, -1); return err }},
    {func() error { _, err := executeWith(http.DefaultClient, nil, -1); return err }, func() error { _, err := reference.SendWith(http.DefaultClient, nil, -1); return err }},
    {func() error { _, err := executeWith(http.DefaultClient, request, -1); return err }, func() error { _, err := reference.SendWith(http.DefaultClient, request, -1); return err }},
  }
  for index, matrix := range matrices {
    if got, want := matrix.generated(), matrix.goReference(); !sameError(got, want) { t.Errorf("validation %d: OnsenTamago=%v Go=%v", index, got, want) }
  }

  server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) { <-request.Context().Done() }))
  defer server.Close()
  generatedContext, generatedCancel := context.WithCancel(context.Background()); generatedCancel()
  goContext, goCancel := context.WithCancel(context.Background()); goCancel()
  _, gotErr := load(generatedContext, server.URL)
  _, wantErr := reference.Load(goContext, server.URL)
  if !errors.Is(gotErr, context.Canceled) || !errors.Is(wantErr, context.Canceled) { t.Errorf("cancellation: OnsenTamago=%v Go=%v", gotErr, wantErr) }

  for _, test := range []struct { name string; body func() io.Reader; limit int64; wantError bool }{
    {"success", func() io.Reader { return strings.NewReader("body") }, 4, false},
    {"oversize", func() io.Reader { return strings.NewReader("body") }, 3, true},
    {"read error", func() io.Reader { return errorReader{} }, 4, true},
  } {
    t.Run(test.name, func(t *testing.T) {
      var generatedCloses, goCloses atomic.Int64
      generatedClient := responseClient(test.body(), &generatedCloses)
      goClient := responseClient(test.body(), &goCloses)
      generatedRequest, _ := http.NewRequest(http.MethodGet, "http://fixture.invalid", nil)
      goRequest, _ := http.NewRequest(http.MethodGet, "http://fixture.invalid", nil)
      _, generatedErr := executeWith(generatedClient, generatedRequest, test.limit)
      _, goErr := reference.SendWith(goClient, goRequest, test.limit)
      if (generatedErr != nil) != test.wantError || !sameError(generatedErr, goErr) { t.Errorf("errors: OnsenTamago=%v Go=%v", generatedErr, goErr) }
      if got, want := generatedCloses.Load(), goCloses.Load(); got != want || got != 1 { t.Errorf("close count: OnsenTamago=%d Go=%d", got, want) }
    })
  }
}
func TestConcurrentFetchTasks(t *testing.T) {
  server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) { _, _ = fmt.Fprintf(writer, "item=%s", request.URL.Query().Get("id")) }))
  defer server.Close()
  var wait sync.WaitGroup
  for index := 0; index < 24; index++ {
    index := index
    wait.Add(1)
    go func() {
      defer wait.Done()
      target := fmt.Sprintf("%s?id=%d", server.URL, index)
      got, gotErr := load(context.Background(), target)
      want, wantErr := reference.Load(context.Background(), target)
      if !sameError(gotErr, wantErr) || got.Text() != want.Text() { t.Errorf("request %d: OnsenTamago=(%q,%v) Go=(%q,%v)", index, got.Text(), gotErr, want.Text(), wantErr) }
    }()
  }
  wait.Wait()
}
type transportFunc func(*http.Request) (*http.Response, error)
func (function transportFunc) RoundTrip(request *http.Request) (*http.Response, error) { return function(request) }
type trackedBody struct { reader io.Reader; closes *atomic.Int64 }
func (body *trackedBody) Read(buffer []byte) (int, error) { return body.reader.Read(buffer) }
func (body *trackedBody) Close() error { body.closes.Add(1); return nil }
type errorReader struct{}
func (errorReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func responseClient(reader io.Reader, closes *atomic.Int64) *http.Client {
  return &http.Client{Transport: transportFunc(func(request *http.Request) (*http.Response, error) {
    return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: &trackedBody{reader: reader, closes: closes}, Request: request}, nil
  })}
}
`
	runGeneratedGoDifferentialTest(t, temp, "fetch.test", generated, referenceSource, testSource)
}
