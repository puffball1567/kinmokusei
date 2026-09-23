# Decorator language foundation

Status: metadata registration foundation implemented. Applications retain their
targets in the AST, factories undergo ordinary expression/call checking,
target-specific callback types reject invalid applications, and generated Go
evaluates factories and invokes their callbacks during `init`.
Callable constructor/method adapters for automatic DI and routing remain future
work; the current feature is usable for typed metadata collection.

## Goal

External Kinmokusei packages must be able to implement NestJS-style declaration
APIs, without hard-coding Controller, Injectable, Get or Param into the compiler.
Parameter decorators and type information needed for dependency injection are
part of the goal, not optional substitutes for method wrappers.

```ts
@Controller("/users")
export class Users {
  constructor(@Inject("users") private service: UserService) {}

  @Get("/:id")
  public function find(@Param("id") id: string): Result<User> {
    return this.service.find(id);
  }
}
```

## Frontend slice

- Applications have the form `@Name`, `@Name(arguments)` or a qualified name
  such as `@http.Get("/")`. Arguments retain ordinary expression ASTs.
- Class decorators may precede export or follow it. Applications are also
  retained on class methods, accessors, fields and constructors, and on parsed
  callable parameters including constructor parameter properties.
- Application lists retain source order and their own spans. Parsing a bare
  decorator must not consume the following class body as a composite literal.
- Speculative parameter/arrow parsing rolls back the application index along
  with token changes and diagnostics.
- The program application index survives source module linking, so a decorator
  in a dependency cannot silently disappear and produce undecorated Go output.
- Class/member/constructor/method-parameter applications now carry checked
  compiler metadata: declaration identity, owning declaration, parameter index,
  static/visibility flags and the declared value or callable signature.
  These descriptors are not yet runtime contexts accessible to library code.
- Factories are module-scoped expressions. Their names and arguments use normal
  import/re-export linking, expression resolution and argument checks. Member
  and parameter names do not shadow decorator factories. A resolved application
  must return `(context: DecoratorContext) => void` or one of the target-specific
  context callbacks below; returning an ordinary value or an incompatible
  callback is diagnosed.
- Applications on other parsed targets (such as local arrow parameters) are
  explicitly rejected.

## Runtime contract

`DecoratorContext` is a compiler-provided value type with these readable fields:

```ts
kind: string
identity: string
classIdentity: string
baseIdentity: string
overrideChain: string[]
className: string
memberName: string
parameterName: string
parameterIndex: int
static: boolean
visibility: string
valueType: string
valueIdentity: string
```

Factories that only support one declaration kind should use its nominal context
type instead of the unrestricted `DecoratorContext`:

| Context type | Accepted target |
|---|---|
| `ClassDecoratorContext` | class |
| `FieldDecoratorContext` | field |
| `ConstructorDecoratorContext` | constructor |
| `MethodDecoratorContext` | ordinary method |
| `GetterDecoratorContext` | getter |
| `SetterDecoratorContext` | setter |
| `ParameterDecoratorContext` | constructor or method parameter |

All context types expose the same fields for a stable library-facing metadata
contract, but they are nominally distinct. Assigning a target-specific callback
to an unrestricted callback type is an error, so a restriction cannot be erased
through a function variable or returned callback.

```ts
export function Controller(path: string):
    (context: ClassDecoratorContext) => void {
  return (context) => registerController(path, context.classIdentity);
}

export function Get(path: string):
    (context: MethodDecoratorContext) => void {
  return (context) => registerRoute(path, context.identity);
}
```

`identity` is opaque and stable for a target within repeatable builds.
`classIdentity` identifies the declaring class without requiring consumers to
parse a member identity, and `baseIdentity` identifies its direct base class
when one exists. For an overriding method, getter or setter, `overrideChain`
lists overridden target identities from the nearest declaration to the oldest
declaration. Parameter targets carry the corresponding parameter target chain
by position. Other targets use an empty list. The name fields preserve source
spellings for diagnostics and framework metadata. Empty
member/parameter names and `parameterIndex == -1` mean that the field does not
apply to that target. `valueType` is the readable declared type. For nominal
Kinmokusei class, interface and struct values, `valueIdentity` supplies a stable
opaque type key; it is empty for structural and primitive values. DI containers
must match this key rather than parsing `valueType` or comparing display names.

An external package can define decorators without compiler knowledge of its API:

```ts
export function Register(label: string): (context: DecoratorContext) => void {
  return (context) => {
    registrations = append(registrations, context);
  };
}
```

Factories on one target are evaluated from top to bottom and their callbacks are
applied from bottom to top. Generated registration runs in Go `init`, after
package variables have been initialized. Decorator code never runs in the
compiler process.

## Remaining implementation

1. Supply checked callable construction and method-invocation adapters so DI and
   routing libraries can operate without reflection or generated-code edits.
   Preserve `Result`, visibility, nullable and generic contracts.

All decorator context types provide type/field hover and completion. Imported
decorator calls participate in ordinary definition, hover, signature, reference
and rename operations.

The runtime contract is metadata-only and does not promise JavaScript reflection
or arbitrary declaration rewriting. Unsupported applications must remain hard
errors rather than silently producing undecorated Go.

Decorators execute only for declarations on which they are written. An override
does not implicitly execute or copy decorators from a base declaration. A
framework may use `overrideChain` to look up registered base metadata and apply
its own merge, replacement or inheritance policy without relying on parsed
identity strings.

External-package target identities use the package graph's canonical module and
source identity when a linked name is required. Moving a checkout or selecting
a consumer-local import alias therefore leaves generated output and identities
unchanged. Registration follows dependency order before consumer modules; this
ordering and repeated-build output are covered across independently versioned
source packages.
