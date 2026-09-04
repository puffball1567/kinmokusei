package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHTTPRouterMatchesIndependentGoAndIsPubliclyConsumable(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "router.km")
	input := `
import { App, Context } from "kinmokusei/http";
import go fmt from "fmt";
import go http from "net/http";

function NewRouter(): http.Handler {
  const app = new App();
  app.get("/health", (ctx: Context): void => {
    ctx.writer.Header().Set("X-Route", "health");
    fmt.Fprint(ctx.writer, "ok");
  });
  app.get("/users/{id}", (ctx: Context): void => {
    ctx.writer.Header().Set("X-Request", ctx.header("X-Request"));
    fmt.Fprintf(ctx.writer, "%s|%s", ctx.path("id"), ctx.query("view"));
  });
  app.get("/cookie", (ctx: Context): void => {
    const [cookie, err] = ctx.cookie("session");
    if (err != nil) {
      ctx.writer.WriteHeader(http.StatusBadRequest);
      return;
    }
    let seen = http.Cookie{Name: "seen", Value: cookie.Value, Path: "/", HttpOnly: true};
    ctx.setCookie(&seen);
    fmt.Fprint(ctx.writer, cookie.Value);
  });
  app.post("/items", (ctx: Context): void => { ctx.writer.WriteHeader(http.StatusCreated); });
  app.put("/items/{id}", (ctx: Context): void => { ctx.writer.WriteHeader(http.StatusNoContent); });
  app.patch("/items/{id}", (ctx: Context): void => { ctx.writer.WriteHeader(http.StatusNoContent); });
  app.delete("/items/{id}", (ctx: Context): void => { ctx.writer.WriteHeader(http.StatusNoContent); });
  app.handle(http.MethodOptions, "/items", (ctx: Context): void => {
    ctx.writer.Header().Set("Allow", "POST, OPTIONS");
    ctx.writer.WriteHeader(http.StatusNoContent);
  });
  return app.handler();
}
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "httpkernel")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v\n%s", err, diagnostics, generated)
	}
	for _, expected := range []string{
		"type Context struct", "func NewContext", "func (this *Context) Path", "func (this *Context) Context() context.Context",
		"func (this *Context) Cookie(name string) (*http.Cookie, error)", "func (this *Context) SetCookie",
		"type App struct", "var _ http.Handler = &App{}", "func NewApp() *App",
		"func (this *App) Handle", "func (this *App) Get", "func (this *App) Delete",
		"func (this *App) Handler() http.Handler", "func (this *App) ServeHTTP",
		"func __kinmokuseiStaticApproutePattern", "func NewRouter() http.Handler",
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated HTTP router does not contain %q:\n%s", expected, generated)
		}
	}
	for _, forbidden := range []string{"func AppRoutePattern", "func App_routePattern", directory, source, "github.com/puffball1567/kinmokusei"} {
		if strings.Contains(string(generated), forbidden) {
			t.Errorf("publishable generated router contains %q:\n%s", forbidden, generated)
		}
	}

	referenceSource := `package reference

import (
  "context"
  "net/http"
)

type Context struct {
  Writer http.ResponseWriter
  Request *http.Request
}
func NewContext(writer http.ResponseWriter, request *http.Request) *Context { return &Context{Writer: writer, Request: request} }
func (ctx *Context) Path(name string) string { return ctx.Request.PathValue(name) }
func (ctx *Context) Query(name string) string { return ctx.Request.URL.Query().Get(name) }
func (ctx *Context) Header(name string) string { return ctx.Request.Header.Get(name) }
func (ctx *Context) Context() context.Context { return ctx.Request.Context() }
func (ctx *Context) Cookie(name string) (*http.Cookie, error) { return ctx.Request.Cookie(name) }
func (ctx *Context) SetCookie(cookie *http.Cookie) { http.SetCookie(ctx.Writer, cookie) }

type App struct { mux *http.ServeMux }
func NewApp() *App { return &App{mux: http.NewServeMux()} }
func routePattern(method, pattern string) string { return method + " " + pattern }
func (app *App) Handle(method, pattern string, handler func(*Context)) {
  app.mux.HandleFunc(routePattern(method, pattern), func(writer http.ResponseWriter, request *http.Request) {
    handler(NewContext(writer, request))
  })
}
func (app *App) Get(pattern string, handler func(*Context)) { app.Handle(http.MethodGet, pattern, handler) }
func (app *App) Post(pattern string, handler func(*Context)) { app.Handle(http.MethodPost, pattern, handler) }
func (app *App) Put(pattern string, handler func(*Context)) { app.Handle(http.MethodPut, pattern, handler) }
func (app *App) Patch(pattern string, handler func(*Context)) { app.Handle(http.MethodPatch, pattern, handler) }
func (app *App) Delete(pattern string, handler func(*Context)) { app.Handle(http.MethodDelete, pattern, handler) }
func (app *App) Handler() http.Handler { return app }
func (app *App) ServeHTTP(writer http.ResponseWriter, request *http.Request) { app.mux.ServeHTTP(writer, request) }
`
	testSource := `package httpkernel_test

import (
  "fmt"
  "net/http"
  "net/http/httptest"
  "sync"
  "testing"
  generated "http-router.test"
  reference "http-router.test/reference"
)

type observation struct { Status int; Body, Allow, Route, Request, SetCookie string }

func request(handler http.Handler, method, target, requestHeader string) observation {
  recorder := httptest.NewRecorder()
  incoming := httptest.NewRequest(method, target, nil)
  incoming.Header.Set("X-Request", requestHeader)
  if requestHeader != "" { incoming.AddCookie(&http.Cookie{Name: "session", Value: requestHeader}) }
  handler.ServeHTTP(recorder, incoming)
  return observation{
    Status: recorder.Code,
    Body: recorder.Body.String(),
    Allow: recorder.Header().Get("Allow"),
    Route: recorder.Header().Get("X-Route"),
    Request: recorder.Header().Get("X-Request"),
    SetCookie: recorder.Header().Get("Set-Cookie"),
  }
}

func configureGenerated() *generated.App {
  app := generated.NewApp()
  app.Get("/health", func(ctx *generated.Context) { ctx.Writer.Header().Set("X-Route", "health"); fmt.Fprint(ctx.Writer, "ok") })
  app.Get("/users/{id}", func(ctx *generated.Context) { ctx.Writer.Header().Set("X-Request", ctx.Header("X-Request")); fmt.Fprintf(ctx.Writer, "%s|%s", ctx.Path("id"), ctx.Query("view")) })
  app.Get("/cookie", func(ctx *generated.Context) { cookie, err := ctx.Cookie("session"); if err != nil { ctx.Writer.WriteHeader(http.StatusBadRequest); return }; ctx.SetCookie(&http.Cookie{Name: "seen", Value: cookie.Value, Path: "/", HttpOnly: true}); fmt.Fprint(ctx.Writer, cookie.Value) })
  app.Post("/items", func(ctx *generated.Context) { ctx.Writer.WriteHeader(http.StatusCreated) })
  app.Put("/items/{id}", func(ctx *generated.Context) { ctx.Writer.WriteHeader(http.StatusNoContent) })
  app.Patch("/items/{id}", func(ctx *generated.Context) { ctx.Writer.WriteHeader(http.StatusNoContent) })
  app.Delete("/items/{id}", func(ctx *generated.Context) { ctx.Writer.WriteHeader(http.StatusNoContent) })
  app.Handle(http.MethodOptions, "/items", func(ctx *generated.Context) { ctx.Writer.Header().Set("Allow", "POST, OPTIONS"); ctx.Writer.WriteHeader(http.StatusNoContent) })
  return app
}

func configureReference() *reference.App {
  app := reference.NewApp()
  app.Get("/health", func(ctx *reference.Context) { ctx.Writer.Header().Set("X-Route", "health"); fmt.Fprint(ctx.Writer, "ok") })
  app.Get("/users/{id}", func(ctx *reference.Context) { ctx.Writer.Header().Set("X-Request", ctx.Header("X-Request")); fmt.Fprintf(ctx.Writer, "%s|%s", ctx.Path("id"), ctx.Query("view")) })
  app.Get("/cookie", func(ctx *reference.Context) { cookie, err := ctx.Cookie("session"); if err != nil { ctx.Writer.WriteHeader(http.StatusBadRequest); return }; ctx.SetCookie(&http.Cookie{Name: "seen", Value: cookie.Value, Path: "/", HttpOnly: true}); fmt.Fprint(ctx.Writer, cookie.Value) })
  app.Post("/items", func(ctx *reference.Context) { ctx.Writer.WriteHeader(http.StatusCreated) })
  app.Put("/items/{id}", func(ctx *reference.Context) { ctx.Writer.WriteHeader(http.StatusNoContent) })
  app.Patch("/items/{id}", func(ctx *reference.Context) { ctx.Writer.WriteHeader(http.StatusNoContent) })
  app.Delete("/items/{id}", func(ctx *reference.Context) { ctx.Writer.WriteHeader(http.StatusNoContent) })
  app.Handle(http.MethodOptions, "/items", func(ctx *reference.Context) { ctx.Writer.Header().Set("Allow", "POST, OPTIONS"); ctx.Writer.WriteHeader(http.StatusNoContent) })
  return app
}

func TestRouterMatrix(t *testing.T) {
  generatedApp, referenceApp := configureGenerated(), configureReference()
  if generatedApp.Handler() != generatedApp || referenceApp.Handler() != referenceApp { t.Error("Handler did not preserve app identity") }
  tests := []struct { method, target, header string }{
    {http.MethodGet, "/health", ""},
    {http.MethodHead, "/health", ""},
    {http.MethodPost, "/health", ""},
    {http.MethodGet, "/users/alice?view=full", "trace-1"},
    {http.MethodGet, "/users/%E6%B8%A9%E6%B3%89?view=%E8%A9%B3%E7%B4%B0", "Trace-Two"},
    {http.MethodGet, "/cookie", "hot-spring"},
    {http.MethodGet, "/cookie", ""},
    {http.MethodPost, "/items", ""},
    {http.MethodPut, "/items/1", ""},
    {http.MethodPatch, "/items/2", ""},
    {http.MethodDelete, "/items/3", ""},
    {http.MethodOptions, "/items", ""},
    {http.MethodGet, "/missing", ""},
  }
  for _, test := range tests {
    got := request(generatedApp, test.method, test.target, test.header)
    want := request(referenceApp, test.method, test.target, test.header)
    if got != want { t.Errorf("%s %s = %#v, Go = %#v", test.method, test.target, got, want) }
  }
  generatedSource := generated.NewRouter()
  if got, want := request(generatedSource, http.MethodGet, "/users/source?view=api", "source-header"), request(referenceApp, http.MethodGet, "/users/source?view=api", "source-header"); got != want {
    t.Errorf("source configured router = %#v, Go = %#v", got, want)
  }
}

func TestDuplicateRoutePanicsLikeGo(t *testing.T) {
  generatedApp, referenceApp := generated.NewApp(), reference.NewApp()
  generatedApp.Get("/duplicate", func(*generated.Context) {})
  referenceApp.Get("/duplicate", func(*reference.Context) {})
  got, want := didPanic(func() { generatedApp.Get("/duplicate", func(*generated.Context) {}) }), didPanic(func() { referenceApp.Get("/duplicate", func(*reference.Context) {}) })
  if got != want || !got { t.Errorf("duplicate route panic = %v, Go = %v", got, want) }
}

func TestConcurrentRouterRequests(t *testing.T) {
  generatedApp, referenceApp := configureGenerated(), configureReference()
  var wait sync.WaitGroup
  failures := make(chan string, 64)
  for index := 0; index < 64; index++ {
    wait.Add(1)
    go func(index int) {
      defer wait.Done()
      target := fmt.Sprintf("/users/item-%d?view=%d", index, index*2)
      header := fmt.Sprintf("request-%d", index)
      got := request(generatedApp, http.MethodGet, target, header)
      want := request(referenceApp, http.MethodGet, target, header)
      if got != want { failures <- fmt.Sprintf("%d: generated=%#v Go=%#v", index, got, want) }
    }(index)
  }
  wait.Wait()
  close(failures)
  for failure := range failures { t.Error(failure) }
}

func didPanic(call func()) (panicked bool) { defer func() { panicked = recover() != nil }(); call(); return false }
`
	runGeneratedGoDifferentialTest(t, directory, "http-router.test", generated, referenceSource, testSource)
}
