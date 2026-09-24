# Decorator language foundation

Status: metadata registration, checked constructor adapters and checked method
and accessor invocation adapters are implemented.
Applications retain their targets in the AST, factories undergo ordinary
expression/call checking, target-specific callback types reject invalid
applications, and generated Go evaluates factories and invokes their callbacks
during `init`.

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
  static/visibility flags and the declared value or callable signature. These
  descriptors are supplied to library code as runtime contexts.
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
constructible: boolean
constructUnavailableReason: string
construct: (arguments: DecoratorValue[]) => Result<DecoratorValue>
invocable: boolean
invokeUnavailableReason: string
invoke: (receiver: DecoratorValue, arguments: DecoratorValue[]) => Result<DecoratorValue>
staticInvocable: boolean
staticInvokeUnavailableReason: string
invokeStatic: (arguments: DecoratorValue[]) => Result<DecoratorValue>
```

`DecoratorValue` is a compiler-owned opaque value with one readable field:

```ts
typeIdentity: string
```

Its payload is not exposed to source code. A framework can retain and pass the
value to another checked adapter, while generated Go performs an exact type
assertion for every constructor argument before calling the ordinary typed
`New<Class>` function. A mismatch or wrong argument count is returned through
`Result`; it does not panic or fall back to reflection. Object literals cannot
forge `DecoratorValue` or any decorator context.

Ordinary runtime values cross the same boundary through two compiler built-ins:

```ts
const boxed = decoratorValue("route parameter");
const text = decoratorValueAs<string>(boxed)?;
```

`decoratorValue` infers its payload type (or accepts one explicit type
argument). `decoratorValueAs<T>` requires an explicit target and returns
`Result<T>`; a mismatched payload is an error rather than a panic or zero-value
substitution. This is the scalar path needed by routing libraries and is also
available for nominal, collection and callable values that have Go storage.

The boundary preserves the **declared storage type**, not just the dynamic Go
value. Numeric literals retain explicit widths (`decoratorValue<int8>(1)`),
nullable values (including nil interfaces) round-trip, and source contracts
remain distinct even when Go lowers them to identical types. This also applies
inside collections, callbacks and generic instantiations. A nullable payload
cannot be extracted as non-null, nor can one nominal context type be extracted
as another. Constructor and method adapters use the same checks.

Box at the contract expected by the consumer. For interface-based DI, use
`decoratorValue<Service>(implementation)`; boxing the concrete class and then
extracting a different interface contract is not an implicit conversion.
Concrete generic types such as `Box<int>` are supported. Boxing/extracting an
open type parameter such as `T` or `T[]` is a compile-time error until its full
source contract can be preserved across generic Go lowering. The public
`typeIdentity` is descriptive metadata, not the authority for runtime checks.

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

A DI library can retain constructor adapters and compose them without generated
code edits or Go reflection:

```ts
alias ConstructorAdapter =
  (arguments: DecoratorValue[]) => Result<DecoratorValue>;

let constructors: ConstructorAdapter[] = [];

export function Injectable():
    (context: ClassDecoratorContext) => void {
  return (context) => {
    if (context.constructible) {
      constructors = append(constructors, context.construct);
    }
  };
}
```

Concrete, non-generic classes are constructible, including zero-argument and
variadic constructors. Variadic arguments are checked individually against the
declared element type before the typed Go call is expanded. Abstract classes
and generic classes without concrete type arguments expose `constructible ==
false` with a stable human-readable reason. Calling their adapter still returns
that failure as a `Result`, so a framework does not need an unchecked branch.

For a public instance method, `invoke` accepts a boxed receiver and boxed
arguments. It verifies the receiver's concrete Go type and non-null value,
checks each argument, and calls the generated Go method. A method returning
`Result<T>` forwards its error; plain and `Result<void>` methods return a
`DecoratorValue` whose `typeIdentity` is `void`. Variadic arguments are checked
individually. Virtual methods retain their existing dispatch behavior. A
receiver boxed as a derived type must be explicitly upcast before use with a
base class method adapter.

Static methods use `invokeStatic(arguments)` and require no receiver. The
`invocable` field refers only to instance invocation; `staticInvocable` refers
only to static invocation. Calling the wrong adapter returns its unavailable
reason through `Result`. Private and protected methods, abstract methods, and
methods whose class or signature still requires generic type arguments expose
both flags as false with a human-readable reason.

Public getters and setters use the same adapters on their respective
`GetterDecoratorContext` and `SetterDecoratorContext`. A getter takes no
arguments and returns its boxed property value. A setter takes one boxed value
of the exact declared property type and returns the `void` marker. Instance
accessors use `invoke(receiver, arguments)`; static accessors use
`invokeStatic(arguments)`. Visibility is checked independently for each
accessor: a public getter does not expose a private or protected setter.
Virtual/override accessors preserve ordinary property dispatch, including
explicitly upcast base receivers.

```ts
alias Setter = (receiver: DecoratorValue, arguments: DecoratorValue[]) => Result<DecoratorValue>;
let setters: Setter[] = [];

function CaptureSetter(context: SetterDecoratorContext): void {
  // A framework can retain this checked operation for later property injection.
  if (context.invocable) {
    setters = append(setters, context.invoke);
  }
}
```

Static accessors on generic classes are also invocable: they belong to the
class declaration and cannot use its type parameters. Generic instance
accessors still require concrete receiver specialization. Multiple-result
methods cannot currently be represented by the single boxed return value;
their adapters expose both availability flags as false with an explicit reason.
Their ordinary typed calls and metadata registration remain available.

Factories on one target are evaluated from top to bottom and their callbacks are
applied from bottom to top. Generated registration runs in Go `init`, after
package variables have been initialized. Decorator code never runs in the
compiler process.

## Remaining implementation

1. Extend constructor and method adapters to explicit concrete generic
   instantiations.

All decorator context types provide type/field hover and completion. Imported
decorator calls participate in ordinary definition, hover, signature, reference
and rename operations.

The runtime contract does not promise JavaScript reflection or arbitrary
declaration rewriting. Unsupported decorator applications remain hard errors;
unsupported callable targets report their unavailability through checked
context fields and `Result`.

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
