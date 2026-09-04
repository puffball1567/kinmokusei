package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const jsonAPIDogfoodSource = `
import go json from "encoding/json";
import go http from "net/http";
import go log from "log";

function writeJSON(writer: http.ResponseWriter, status: int, body: { message: string }): error {
  writer.Header().Set("Content-Type", "application/json; charset=utf-8");
  writer.WriteHeader(status);
  return json.NewEncoder(writer).Encode(body);
}

function respond(writer: http.ResponseWriter, status: int, body: { message: string }): void {
  const err: error = writeJSON(writer, status, body);
  if (err != nil) { log.Print(err.Error()); }
}

function health(writer: http.ResponseWriter, request: *http.Request): void {
  if (request.Method != http.MethodGet) {
    respond(writer, http.StatusMethodNotAllowed, { message: "method not allowed" });
    return;
  }
  respond(writer, http.StatusOK, { message: "ok" });
}

function greet(writer: http.ResponseWriter, request: *http.Request): void {
  if (request.Method != http.MethodGet) {
    respond(writer, http.StatusMethodNotAllowed, { message: "method not allowed" });
    return;
  }
  const name: string = request.URL.Query().Get("name");
  if (name == "") {
    respond(writer, http.StatusBadRequest, { message: "name is required" });
    return;
  }
  respond(writer, http.StatusOK, { message: "hello " + name });
}

function newHandler(): http.Handler {
  const mux: *http.ServeMux = http.NewServeMux();
  mux.HandleFunc("/health", health);
  mux.HandleFunc("/greet", greet);
  return mux;
}

function main(): void {
  log.Fatal(http.ListenAndServe(":8080", newHandler()));
}
`

func TestJSONAPIDogfoodCompileRunAndRaceMatrix(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "api.km")
	if err := os.WriteFile(source, []byte(jsonAPIDogfoodSource), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "jsonapi")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{`json "encoding/json"`, `http "net/http"`, `log "log"`, "func writeJSON", `writer.Header().Set("Content-Type"`, "return json.NewEncoder(writer).Encode(body)", `log.Print(err.Error())`, `mux.HandleFunc("/health", health)`, `log.Fatal(http.ListenAndServe(":8080", newHandler()))`} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import (
  "encoding/json"
  "log"
  "net/http"
)
type payload struct { Message string ` + "`json:\"message\"`" + ` }
func writeJSON(writer http.ResponseWriter, status int, body payload) error {
  writer.Header().Set("Content-Type", "application/json; charset=utf-8")
  writer.WriteHeader(status)
  return json.NewEncoder(writer).Encode(body)
}
func respond(writer http.ResponseWriter, status int, body payload) {
  if err := writeJSON(writer, status, body); err != nil { log.Print(err.Error()) }
}
func health(writer http.ResponseWriter, request *http.Request) {
  if request.Method != http.MethodGet { respond(writer, http.StatusMethodNotAllowed, payload{Message: "method not allowed"}); return }
  respond(writer, http.StatusOK, payload{Message: "ok"})
}
func greet(writer http.ResponseWriter, request *http.Request) {
  if request.Method != http.MethodGet { respond(writer, http.StatusMethodNotAllowed, payload{Message: "method not allowed"}); return }
  name := request.URL.Query().Get("name")
  if name == "" { respond(writer, http.StatusBadRequest, payload{Message: "name is required"}); return }
  respond(writer, http.StatusOK, payload{Message: "hello "+name})
}
func NewHandler() http.Handler {
  mux := http.NewServeMux()
  mux.HandleFunc("/health", health)
  mux.HandleFunc("/greet", greet)
  return mux
}
`
	testSource := `package jsonapi
import (
  "encoding/json"
  "fmt"
  "net/http"
  "net/http/httptest"
  "sync"
  "testing"
  reference "jsonapi.test/reference"
)
type payload struct { Message string ` + "`json:\"message\"`" + ` }
type observation struct { Status int; ContentType, Message string }
func request(handler http.Handler, method, target string) (observation, error) {
  recorder := httptest.NewRecorder()
  handler.ServeHTTP(recorder, httptest.NewRequest(method, target, nil))
  var body payload
  if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil { return observation{}, fmt.Errorf("decode %q: %w", recorder.Body.String(), err) }
  return observation{Status: recorder.Code, ContentType: recorder.Header().Get("Content-Type"), Message: body.Message}, nil
}
func TestHTTPMatrix(t *testing.T) {
  generatedHandler, referenceHandler := newHandler(), reference.NewHandler()
  tests := []struct { name, method, target string; status int; message string }{
    {"health", http.MethodGet, "/health", http.StatusOK, "ok"},
    {"health method", http.MethodPost, "/health", http.StatusMethodNotAllowed, "method not allowed"},
    {"greet", http.MethodGet, "/greet?name=Tamago", http.StatusOK, "hello Tamago"},
    {"missing", http.MethodGet, "/greet", http.StatusBadRequest, "name is required"},
    {"unicode and escaping", http.MethodGet, "/greet?name=%E6%B8%A9%E6%B3%89%22", http.StatusOK, "hello 温泉\""},
    {"greet method", http.MethodDelete, "/greet?name=x", http.StatusMethodNotAllowed, "method not allowed"},
  }
  for _, test := range tests {
    t.Run(test.name, func(t *testing.T) {
      got, gotErr := request(generatedHandler, test.method, test.target)
      want, wantErr := request(referenceHandler, test.method, test.target)
      if gotErr != nil || wantErr != nil { t.Fatalf("Kinmokusei error=%v, Go error=%v", gotErr, wantErr) }
      if got != want { t.Errorf("Kinmokusei=%#v, Go=%#v", got, want) }
      if got.Status != test.status || got.Message != test.message || got.ContentType != "application/json; charset=utf-8" { t.Errorf("protocol result=%#v", got) }
    })
  }
  got := httptest.NewRecorder()
  generatedHandler.ServeHTTP(got, httptest.NewRequest(http.MethodGet, "/missing", nil))
  want := httptest.NewRecorder()
  referenceHandler.ServeHTTP(want, httptest.NewRequest(http.MethodGet, "/missing", nil))
  if got.Code != want.Code || got.Body.String() != want.Body.String() || got.Code != http.StatusNotFound { t.Errorf("404 Kinmokusei=(%d,%q), Go=(%d,%q)", got.Code, got.Body.String(), want.Code, want.Body.String()) }
}
func TestConcurrentRequests(t *testing.T) {
  generatedHandler, referenceHandler := newHandler(), reference.NewHandler()
  var wait sync.WaitGroup
  failures := make(chan string, 32)
  for index := 0; index < 32; index++ {
    wait.Add(1)
    go func(index int) {
      defer wait.Done()
      target := fmt.Sprintf("/greet?name=item-%d", index)
      got, gotErr := request(generatedHandler, http.MethodGet, target)
      want, wantErr := request(referenceHandler, http.MethodGet, target)
      if gotErr != nil || wantErr != nil || got != want || got.Status != http.StatusOK || got.Message != fmt.Sprintf("hello item-%d", index) {
        failures <- fmt.Sprintf("index=%d Kinmokusei=%#v/%v Go=%#v/%v", index, got, gotErr, want, wantErr)
      }
    }(index)
  }
  wait.Wait()
  close(failures)
  for failure := range failures { t.Error(failure) }
}
`
	t.Setenv("KINMOKUSEI_DIFFERENTIAL_RACE", "1")
	t.Setenv("GOPROXY", "off")
	runGeneratedGoDifferentialTest(t, directory, "jsonapi.test", generated, referenceSource, testSource)
}
