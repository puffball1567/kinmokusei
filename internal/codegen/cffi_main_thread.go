package codegen

import "strings"

// The marker is local to the OS thread, so checking it does not depend on
// goroutine IDs or Go heap addresses. Guards pin before checking and stay
// pinned until the native call and its cleanup finish.
func generateCFFIMainThreadCHelpers(source *strings.Builder) {
	source.WriteString(`
#if defined(_MSC_VER)
static __declspec(thread) unsigned char kinmokusei_cffi_main_thread;
#else
static _Thread_local unsigned char kinmokusei_cffi_main_thread;
#endif
static inline void kinmokusei_cffi_mark_main_thread(void) { kinmokusei_cffi_main_thread = 1; }
static inline int kinmokusei_cffi_is_main_thread(void) { return kinmokusei_cffi_main_thread != 0; }
`)
}

func generateCFFIMainThreadInit(source *strings.Builder) {
	source.WriteString(`
var ErrWrongThread = errors.New("C FFI call requires the process startup thread")
func init() {
  runtime.LockOSThread()
  C.kinmokusei_cffi_mark_main_thread()
}
`)
}

func generateCFFIMainThreadGuard(source *strings.Builder, failure string) {
	source.WriteString("runtime.LockOSThread()\ndefer runtime.UnlockOSThread()\n")
	source.WriteString("if C.kinmokusei_cffi_is_main_thread() == 0 { " + failure + " }\n")
}
