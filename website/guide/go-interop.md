# Go interoperability

`import go` connects a namespace to an ordinary Go package. This is the primary ecosystem boundary, not a compatibility shim.

```ts
import go strings from "strings";

function normalize(value: string): string {
  return strings.ToUpper(strings.TrimSpace(value));
}
```

OnsenTamago preserves Go constants, functions, variables, basic and named types, aliases, structs, fields, pointers, `nil`, methods, interfaces, multiple results, callbacks, generics, channels, and explicit conversions where supported.

External modules use the existing Go module graph. Normal check/build/run operations are read-only and offline with respect to dependency resolution. Add an exact module deliberately:

```sh
ontama install --go-module github.com/google/uuid@v1.6.0
ontama deps check
ontama licenses
```

Unsupported reachable API shapes are diagnosed at the OnsenTamago use site. Audit a package or the standard library surface with `ontama interop audit` before choosing a dependency boundary.
