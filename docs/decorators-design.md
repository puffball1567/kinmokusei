# Decorator language foundation

Status: implementation in progress. Applications retain their targets in the
AST, and factories now undergo ordinary expression/call checking. Decorated programs are deliberately
rejected by semantic checking until metadata generation and lowering exist.
This is not yet a usable decorator feature.

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
  must be callable; returning an ordinary value from a factory is diagnosed.
- Applications on other parsed targets (such as local arrow parameters) are
  explicitly rejected. The callable's compatibility with the future runtime
  context, generic specialization and permitted per-decorator target kinds
  remain subsequent work.

## Remaining implementation

1. Define typed decorator contexts and metadata values usable by ordinary
   external packages. Preserve owner/member identity, parameter position,
   declared types, static/instance distinction and constructor information.
   Names alone must not be used as globally unique type identities.
2. Resolve and type-check decorator definitions/factories through normal
   imports and exports. Diagnose unsupported targets, arity and argument types.
   Specify factory evaluation, application order and inheritance explicitly.
3. Generate metadata and registration code in Go. Supply callable construction
   and invocation facilities so DI and routing libraries can operate without
   users editing generated code. Preserve Result handling, visibility and
   nullable contracts. Decorators do not run during compiler type checking.
4. Add cross-package runtime tests with an independent minimal registration/DI
   consumer, plus editor completion, hover, references and rename support.
   Validate repeated builds, package identity and initialization order.

The initial syntax/AST commit intentionally does not invent a string-only
metadata registry or promise JavaScript runtime compatibility. Remove the
temporary semantic rejection only when a checked application has a defined
Go lowering; do not silently ignore unsupported applications.
