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
```

String escapes and decimal integer/float literals are validated lexically. Numeric separators and base prefixes are not implemented. A malformed escape, unterminated comment/string, invalid UTF-8 byte, or overflowing constant is reported against the original source span.

Semicolons terminate imports, bindings, returns, expression statements, assignments, updates, branches, `throw`, `defer`, and C export blocks where shown. Braced declaration/control-flow bodies do not take a trailing semicolon.

## Top-level declaration inventory

| Declaration | Core form |
| --- | --- |
| Relative import | `import { A, functionName } from "./module";` |
| Go import | `import go alias from "package/path";` |
| Binding | `const name: T = value;`, `let name = value;` |
| Function | `function name<T>(value: T): T { ... }` |
| Class | `class Name extends Base implements Contract { ... }` |
| Struct | `struct Name { public field: T; ... }` |
| Interface | `interface Name { function method(): T; }` |
| Enum | `enum Name: int32 { First, Second = 4 }` |
| Transparent alias | `alias Name = T;` |
| Defined type | `type Name = distinct T;` |
| External receiver method | `public function method(this: *T): void { ... }` |
| C export | `export c("symbol") function name(): int32 { ... }` |

Declarations may refer to later types in supported finite shapes. Duplicate source names and generated Go public-name collisions are diagnosed rather than renamed silently.

Relative source imports are explicit and do not infer visibility from capitalization. In emitted Go, top-level identifiers preserve their written case: an initial uppercase Unicode letter makes the declaration exported under Go rules. Class and struct members instead use the documented `public`/`protected`/`private` contract before their Go names are generated.

## Files and imports

```ts
import { Name, functionName } from "./relative-module";
import go alias from "go/package/path";
```

Files use `.km`, have independent scopes, and expose no transitive imports. `kinmokusei/http` is the implemented compiler-managed standard module.

Relative imports are selective. Paths resolve relative to the importing file, with optional `.km`; missing declarations, cycles, duplicate imports, and conflicts with local declarations are errors. Bare package imports are not supported—Go packages always use `import go`, while compiler-managed modules use the named relative-import form.

## Bindings and functions

```ts
const immutable: int = 1;
let mutable = 2;

function name<T extends comparable>(value: T): T { return value; }
const arrow = (value: int): int => value + 1;
function variadic(prefix: int, ...values: int[]): int { return prefix; }
```

`const`/`let` apply to bindings. Functions, arrows, function types, methods, interfaces, and constructors support final rest parameters. Generic calls accept inference, `<T>`, or `[T]` type arguments.

Destructuring is limited to compiler-known multiple results:

```ts
const [value, err] = operation();
let [next, open] = receive();
[next, open] = receive();
```

Multiple results are not tuple values and cannot be stored as one value. Every result must have a binding; use `_` to discard a position explicitly.

## Named type declarations

```ts
enum Status: uint16 { Pending, Running = 4, Complete }
alias Text = string;
type UserID = distinct string;

struct Point<T> { public value: T; }
interface Reader<T> { function read(): T; }
class Box<T> implements Reader<T> { /* ... */ }
```

Generic aliases, native constraint type sets beyond `comparable`, and distinct definitions over native classes/structs/interfaces are unsupported.

Enum initializers are integer constant expressions. Implicit members start at zero or increment the previous value; range is checked against the declared integer underlying type.

## Struct methods

```ts
struct Counter {
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

Only `virtual` base methods dispatch dynamically. Generic classes currently reject inheritance and virtual/static members.

`implements` is explicit even when a method set happens to match. `super(...)` is available in a derived constructor, and `super.method()` statically selects the immediate base implementation. `override` must match a virtual base member; `final` closes a class or override.

### Member modifier forms

| Context | Accepted shape | Contract |
| --- | --- | --- |
| Top-level receiver method | `[public \| private] function name(this: T, ...): R` | Visibility precedes `function`; `this` must be first |
| Class field | `[public \| protected \| private] name: T;` | Defaults to private |
| Class constructor | `constructor([visibility] name: T, ...) { ... }` | At most one; a parameter visibility declares a field |
| Class method | `[visibility] [static \| virtual \| override \| final]* function ...` | Member-kind modifiers may be ordered freely but not repeated |
| Struct field | `[public \| private] name: T;` | Defaults to private |
| Struct method | `[public \| private] [pointer] function ...` | `pointer` selects a pointer receiver; default is a value receiver |

Class fields cannot be `static`, `virtual`, `override`, or `final`. Constructors have no independent visibility contract and reject those four member-kind modifiers. `protected` is class-only; struct members and top-level receiver methods use public/private visibility.

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
