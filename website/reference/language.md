---
title: Language syntax reference
description: Searchable declaration, expression, and statement forms implemented by the Kinmokusei compiler.
---

# Language syntax reference

This page indexes implemented forms. Detailed semantic rules live in the linked guide and type/reference pages.

## Lexical conventions

Source is UTF-8 and files use `.km`. Identifiers are case-sensitive and may contain Unicode letters; generated public Go names are checked for collisions after normalization. Keywords such as `function`, `class`, `go`, `null`, and `select` cannot be declaration names.

```ts
// line comment
/* block comment */
const enabled = true;
const count = 1000;
const ratio = 1.25;
const label = "hello\nたまご";
const raw = b"\xff";
```

String escapes and Go-shaped numeric literals are validated lexically. Integers
accept decimal, binary (`0b`), octal (`0o` or a leading zero), hexadecimal (`0x`),
and valid underscore separators. Floating-point literals accept decimal and
hexadecimal exponents, and an `i` suffix produces an imaginary constant.
A malformed escape, separator, literal, unterminated comment/string, invalid
UTF-8 byte, or overflowing constant is reported against the original source span.
Ordinary decoded literals must be valid UTF-8. Immediately prefixed `b"..."`
literals instead have type `bstring` and permit arbitrary decoded bytes.

Semicolons terminate imports, bindings, fields, type declarations, interface signatures, returns, expression statements, assignments, updates, branches, `throw`, `defer`, and grouped C exports. They may be omitted after a complete statement when the next token is on a later line, is `}`, or is end of file. Three-clause `for` separators remain explicit. Braced declaration/control-flow bodies do not take a trailing semicolon.

Calls, indexing, selectors, operators, and unfinished expressions/types can continue across newlines. Use `;` if a new statement beginning with `(` or `[` could otherwise continue the previous expression. Newlines inside comments count. Immediately after `return` or `throw`, a newline instead ends a bare return/rethrow; optional break/continue labels must stay on the keyword's line. These newline rules apply with or without explicit semicolons elsewhere.

## Top-level declaration inventory

| Declaration | Core form |
| --- | --- |
| Relative import | `import { A, functionName } from "./module";` |
| Go import | `import go alias from "package/path";` |
| Named Go import | `import go { Name, Other } from "package/path"` |
| Local import alias | `import { Name as LocalName } from "./module"`, `import go { Println as print } from "fmt"` |
| Source export | `export function name(): T { ... }`, `export const name = value`, `export { name as publicName }`, `export { name } from "./module"` |
| Binding | `const name: T = value;`, `let name = value;` |
| Function | `function name<T>(value: T): T { ... }` |
| Class | `class Name extends Base implements Contract { ... }` |
| Decorated class/member | `@Register class Name { ... }`, `@Route("/users") public function list(): T { ... }` |
| Struct | `struct Name { public field: T; ... }` |
| Interface | `interface Name { function method(): T; }` |
| Enum | `enum Name: int32 { First, Second = 4 }` |
| Transparent alias | `alias Name = T;` |
| Defined type | `type Name = distinct T;` |
| Type-set constraint | `constraint Number = ~int \| ~float64;` |
| External receiver method | `public function method(this: *T): void { ... }` |
| C export | `export c("symbol") function name(): int32 { ... }` |

Declarations may refer to later types in supported finite shapes. Duplicate source names and generated Go public-name collisions are diagnosed rather than renamed silently.

Relative source imports are explicit and do not infer visibility from capitalization. In emitted Go, top-level identifiers preserve their written case: an initial uppercase Unicode letter makes the declaration exported under Go rules. Class and struct members instead use the documented `public`/`protected`/`private` contract before their Go names are generated.

Any source export opts its file into explicit visibility:
only selected local declarations or explicitly re-exported source bindings can be imported, and `export {}`
exports none. Without a source export, all top-level declarations retain legacy
selective importability. Export lists can precede declarations, cannot repeat
names, and support trailing commas and omitted semicolons. C ABI `export c(...)`
does not opt into this mode. See [modules and imports](../book/modules-and-imports).

## Decorator applications

`@Name` selects a decorator callback; `@Factory(arguments)` calls a factory
that returns one. Namespace-qualified and imported/re-exported names are
supported. Place applications before a class or class member, or before a
constructor/method parameter. Callbacks have a single compiler-provided
decorator context parameter and return `void`. They execute during package
initialization, not on each construction or method call. See
[typed decorators](../book/structs-classes-interfaces#typed-decorators) for
supported targets, registration order and checked adapters.

## Files and imports

```ts
import { Name, functionName } from "./relative-module";
import go alias from "go/package/path";
import go { Name, Other } from "go/package/path";
import { Name as LocalName } from "./relative-module";
import go { Println as print } from "fmt";
```

Files use `.km`, have independent scopes, and expose no transitive imports. `kinmokusei/http` is the implemented compiler-managed standard module.

Source imports are selective. Relative paths resolve from the importing file,
with optional `.km`. Standard modules and locked external Kinmokusei packages
use the same `import { Name } from "path"` form; manifest import aliases may
shorten an external path without changing its module or type identity.
Missing declarations, cycles, duplicate imports, and conflicts with local
declarations are errors. Go packages always use `import go`; a bare side-effect
import without named bindings is not supported.

`as` introduces only the selected local name and retains declaration identity,
signatures and mutable storage. Selecting an export twice is permitted when the
local names differ. This differs from `[imports]`, which renames package-path
prefixes rather than individual symbols. See
[local import aliases](../book/modules-and-imports#local-import-aliases).

## Bindings and functions

```ts
const immutable: int = 1;
let mutable = 2;

function name<T extends comparable>(value: T): T { return value; }
const arrow = (value: int): int => value + 1;
function variadic(prefix: int, ...values: int[]): int { return prefix; }
```

`const`/`let` apply to bindings. Functions, arrows, function types, methods, interfaces, and constructors support final rest parameters. Generic calls accept inference, `<T>`, or `[T]` type arguments.

Module-level const arrows with inferred/unnamed function
types emit callable declarations, including `const main = () => { ... }`. Explicit
signatures enable recursion and forward calls. Callable declarations are not
addressable or reassignable; `let` retains mutable function storage. Expected
function types can supply omitted arrow parameter/result annotations. Without a
result context, block arrows infer a default result from their first return and
check the remaining returns against it; no value returns means `void`.
See [arrow semantics](../book/functions-and-generics#arrow-functions).

Destructuring is limited to compiler-known multiple results:

```ts
const [value, err] = operation();
let [next, open] = receive();
[next, open] = receive();
```

Multiple results are not tuple values and cannot be stored as one value. Source
functions, methods, arrows, interfaces, and function types may declare a result
list such as `(int, boolean)`. Use `return value, present;` or forward a matching
multiple-result call. A sole call argument may expand into matching parameters;
it cannot be mixed with additional arguments. Bind every returned position or
use `_` explicitly. See [multiple results](../book/functions-and-generics#multiple-results).

## Named type declarations

```ts
enum Status: uint16 { Pending, Running = 4, Complete }
alias Text = string;
type UserID = distinct string;

struct Point<T> { public value: T; }
interface Reader<T> { function read(): T; }
class Box<T> implements Reader<T> { /* ... */ }
```

Generic aliases such as `alias Values<T> = T[]` are transparent and expand in
generated Go. Native constraints support exact/underlying terms, unions,
intersections, and compatible method interfaces, for example
`constraint NumberOrText = ~int | ~string`. A `distinct` definition may use a
native struct, including a concrete generic struct; it does not inherit that
struct's methods. Distinct definitions over native classes and interfaces remain
unsupported. Generic named types require explicit complete instantiation.

Enum initializers are integer constant expressions. Implicit members start at zero or increment the previous value; range is checked against the declared integer underlying type.

## Struct methods

```ts
struct Counter {
  public value: int;
  public function snapshot(): int { return this.value; }
  public pointer function increment(): void { this.value++; }
}

public function reset(this: *Counter): void { this.value = 0; }
```

The nested default is a value receiver; `pointer function` is a pointer receiver. External receiver functions require `this` and a same-module native struct/enum/defined type as supported.

## Classes and inheritance

```ts
final class Derived extends Base implements Contract {
  constructor(public value: int) { super(); }
  public final override function render(): string { return super.render(); }
  public static function create(): Derived { return new Derived(1); }
}
```

Only `virtual` base methods dispatch dynamically. Generic classes support
single inheritance, substituted base arguments, virtual overrides, and static
methods. Static fields and properties are shared across instantiations and
cannot use class type parameters; static methods may use those parameters
through their generated generic function API.

`implements` is explicit even when a method set happens to match. `super(...)` is available in a derived constructor, and `super.method()` statically selects the immediate base implementation. `override` must match a virtual base member; `final` closes a class or override.

### Member modifier forms

| Context | Accepted shape | Contract |
| --- | --- | --- |
| Top-level receiver method | `[public \| private] function name(this: T, ...): R` | Visibility precedes `function`; `this` must be first |
| Class field | `[public \| protected \| private] name: T;` | Defaults to private |
| Static field/constant | `[visibility] static [const] name: T = value;` | Explicit type and initializer; shared class storage or constant |
| Class constructor | `constructor([visibility] name: T, ...) { ... }` | At most one; a parameter visibility declares a field |
| Class method | `[visibility] [static \| virtual \| override \| final \| abstract]* function ...` | Checked compatible modifier combinations; no duplicates |
| Instance accessor | `[visibility] [virtual \| override \| final \| abstract]* get name(): T`, `set name(value: T)` | Separate read/write visibility; abstract forms have no body |
| Static accessor | `[visibility] static get name(): T`, `static set name(value: T)` | Class-qualified calls without virtual dispatch |
| Interface accessor | `get name(): T;`, `set name(value: T);` | Public contract, no body |
| Struct field | `[public \| private] name: T;` | Defaults to private |
| Struct method | `[public \| private] [pointer] function ...` | `pointer` selects a pointer receiver; default is a value receiver |

Class fields may have per-instance initializers or be `static`, but are not
virtual, overriding, or final members. Instance initializers may read earlier
initialized fields and accessible inherited state through `this.field`; later
field reads and receiver capture are rejected. Constructors reject method-kind
modifiers. `protected` is class-only; struct members and top-level receiver
methods use public/private visibility. Abstract classes cannot be constructed;
concrete descendants must supply all abstract method/accessor implementations.
See [classes and structs](../guide/classes-and-structs).

## Expressions

| Family | Examples | Notes |
| --- | --- | --- |
| Literals | `42`, `1.5`, `"text"`, `true`, `nil`, `null` | `nil` and `null` belong to different boundaries |
| Collections | `[1, 2]`, `{ key: value }`, `Point { x: 1 }` | Context distinguishes object and struct literals |
| Selection | `value.field`, `Type.staticMethod()` | Visibility is checked before Go generation |
| Index/slice | `values[i]`, `values[a:b]`, `values[a:b:c]` | Bounds and addressability follow the documented Go model |
| Calls | `call(a)`, `call(values...)`, `identity<int>(a)` | Arguments evaluate once in source order |
| Construction | `new Class(args...)` | `new` is for classes, not native structs |
| Conversion | `int64(value)`, `UserID(text)` | Explicit and type-checked |
| Assertion | `value as? T`, `value as! T` | Checked pair versus panic-on-failure |
| Function | `(value: int): int => value + 1` | Expression or block body |
| Task | `go call()`, `await task`, `await task?` | Structured exactly-once capability |
| Receive | `<-channel` | May initialize one value or `[value, open]` |
| Result propagation | `operation()?` | Only in a compatible `Result` return context |

Object, call-target, receiver, index, assertion source, and arguments are evaluated once at observable boundaries. See [Operators](./operators) for precedence and valid operands.

## Control flow

### Statement inventory

| Statement | Form | Terminator or body |
| --- | --- | --- |
| Binding | `const name[: T] = expression;`, `let name[: T] = expression;` | `;` |
| Multiple-result binding | `const [first, second] = call();` | `;`; function-local only |
| Assignment | `target = expression;`, `target += expression;` | `;` |
| Multiple-result assignment | `[first, second] = call();` | `;`; name targets only |
| Update | `target++;`, `target--;` | `;` |
| Expression | `call();` | `;`; the value is discarded |
| Channel send | `channel <- value;` | `;` |
| Return | `return;`, `return expression;` | `;` |
| Throw/rethrow | `throw errorValue;`, `throw;` | `;`; bare form is catch-local |
| Branch | `break [label];`, `continue [label];`, `goto label;`, `fallthrough;` | `;` |
| Raw goroutine | `go call();` | `;`; distinct from task expression context |
| Task discard | `detach task;` | `;`; consumes exactly once |
| Deferred call | `defer call();` | `;` |
| Block | `{ statements }` | no trailing `;` |

An assignment target is a mutable name, writable selector/index, or pointer dereference. Assignment, send, increment, and decrement do not produce expression values. A three-clause `for` accepts binding/simple-statement forms in its initializer and a simple statement in its post clause; header clauses omit their ordinary trailing semicolon where the header punctuation already separates them.

```ts
if (condition) { } else { }
while (condition) { }
for (let i = 0; i < limit; i++) { }
for (const value of values) { }
for (const [key, value] of map) { }
switch (value) { case item { } default { } }
break; continue; goto label; label: statement;
defer call();
```

Conditions are parenthesized and must be `boolean`. `if`, `while`, `for`, `switch`, `select`, and `try` own braced bodies; a label instead prefixes one following statement as `name: statement`.

Value switch cases accept comma-separated expressions. Cases stop after the first match unless the final statement is explicit `fallthrough;`; fallthrough is not available in a type switch.

An untyped subject defaults before its cases are checked (`int` for integer
constants, `float` for floating-point constants). Constant cases include scalar
expressions and imported/re-exported constants. Duplicate integer, floating-point
and string cases follow Go's constant-case rules, including rounding to a concrete
subject type. For interface subjects, equal values with different dynamic types
(such as `int(1)` and `int64(1)`) remain distinct. Boolean/complex cases and runtime
expressions can repeat; the first match wins. Duplicate `nil`/`null` cases remain
rejected by Kinmokusei.

<<< ../snippets/switch-constants.km{ts}

<<< ../snippets-invalid/switch-rounded-duplicate.km{ts}

```ts
switch (reader) {
  case const text as *strings.Reader { return text.Len(); }
  case nil { return 0; }
  default { return -1; }
}
```

A type switch requires a Go interface subject. Each typed binding is scoped to its case and may be `const`, `let`, or `_`. Duplicate/impossible types, duplicate `nil`/`default`, and mixing typed/value cases are rejected.

Labels may target loops, switches, selects, or ordinary statements as appropriate. `goto` cannot enter a nested block, jump over a declaration, cross a `try`/`catch`/`finally` boundary, or leave a task unconsumed. Updates are statements, not expressions.

## Results and exceptions

```ts
function load(): Result<Value> {
  const value = operation()?;
  if (invalid) { return fail(errorValue); }
  return ok(value);
}

try { throw errorValue; }
catch (err: Exception) { throw; }
finally { cleanup(); }
```

`Result<T>` is a function/method return effect only. Catch clauses are ordered and typed; ordinary panics do not become exceptions.

## Nullability

```ts
let user: User | null = null;
if (user !== null) { use(user.name); }
```

Only nil-backed reference types accept `| null`. `null` is distinct from raw imported Go `nil`.

## Concurrency

```ts
go call();
const task: Task<T> = go call();
const value = await task;
detach go call();

channel <- value;
const [value, open] = <-channel;
select { case const value = <-channel { } default { } }
```

Task values are local, non-escaping, and exactly-once consumed. Channels preserve Go direction and runtime behavior.

Select receive cases support a discarded receive, one binding, checked `[value, open]`, or reassignment. A send case uses `case channel <- value`. Each case body is a block; `fallthrough` is invalid. `select {}` blocks forever, and `default` makes selection non-blocking when no communication is ready.

## C export

```ts
export c("symbol") function name(value: int32): int32 { return value; }
export c("one", "two") { first, second };
```

Only explicit fixed-width compatible signatures cross the stable outgoing C boundary.
