package lsp

import (
	"fmt"
	"runtime"
	"runtime/pprof"
	"strings"
	"testing"
	"time"
)

func TestServerLifecycleDoesNotLeakGoroutines(t *testing.T) {
	baseline := runtime.NumGoroutine()
	for index := 0; index < 50; index++ {
		uri := fmt.Sprintf("file:///tmp/kinmokusei-leak-%d.km", index)
		hover := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":%q},"position":{"line":0,"character":10}}}`, uri)
		if _, err := serveAsyncTest(t, nil,
			`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
			openDocument(uri, `function value(input: int): int { return input; }`),
			hover,
			`{"jsonrpc":"2.0","id":90,"method":"shutdown"}`,
			`{"jsonrpc":"2.0","method":"exit"}`,
		); err != nil {
			t.Fatal(err)
		}
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		runtime.GC()
		if runtime.NumGoroutine() <= baseline {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	var stacks strings.Builder
	_ = pprof.Lookup("goroutine").WriteTo(&stacks, 2)
	t.Fatalf("LSP lifecycle leaked goroutines: baseline=%d current=%d\n%s", baseline, runtime.NumGoroutine(), stacks.String())
}
