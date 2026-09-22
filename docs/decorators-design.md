# Decorator language foundation

Status: metadata registration foundation implemented. Applications retain their
targets in the AST, factories undergo ordinary expression/call checking, and
generated Go evaluates factories and invokes their callbacks during `init`.
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
  must return `(context: DecoratorContext) => void`; returning an ordinary value
  or an incompatible callback is diagnosed.
- Applications on other parsed targets (such as local arrow parameters) are
  explicitly rejected.

## Runtime contract

`DecoratorContext` is a compiler-provided value type with these readable fields:

```ts
kind: string
identity: string
classIdentity: string
baseIdentity: string
className: string
memberName: string
parameterName: string
parameterIndex: int
static: boolean
visibility: string
valueType: string
valueIdentity: string
```

`identity` is opaque and stable for a target within repeatable builds.
`classIdentity` identifies the declaring class without requiring consumers to
parse a member identity, and `baseIdentity` identifies its direct base class
when one exists. The name fields preserve source spellings for diagnostics and framework metadata. Empty
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
2. Add per-factory target restrictions and define inheritance behavior for
   metadata on overridden/inherited members.
3. Validate package identity and initialization order across independently
   versioned external packages and repeated builds.

`DecoratorContext` type/field hover and completion are implemented. Imported
decorator calls participate in ordinary definition, hover, signature, reference
and rename operations.

The runtime contract is metadata-only and does not promise JavaScript reflection
or arbitrary declaration rewriting. Unsupported applications must remain hard
errors rather than silently producing undecorated Go.
