# Feasibility of reference classes and inheritance

## Conclusion

Go can support OnsenTamago reference classes, encapsulation, interfaces, dynamic polymorphism, single class inheritance, explicit virtual/override behavior, `super`, safe upcasts, and checked downcasts without a custom garbage collector or object runtime. The compiler must generate more than simple embedding, because Go embedding alone is not subtype inheritance.

Relevant Go references:

- [Go FAQ: object orientation, dynamic dispatch, and inheritance](https://go.dev/doc/faq)
- [Go specification: embedded fields and promoted methods](https://go.dev/ref/spec)
- [Effective Go: embedding is analogous but not identical to subclassing](https://go.dev/doc/effective_go)
- [A Guide to the Go Garbage Collector](https://go.dev/doc/gc-guide)

## Reference classes

A non-inherited class can lower directly:

```go
type Counter struct { value int }

func NewCounter(initial int) *Counter {
    return &Counter{value: initial}
}

func (c *Counter) Increment() { c.value++ }
```

The class value is one Go pointer. Escape analysis chooses stack or heap placement, and the ordinary Go GC owns lifetime. Assignment copies the pointer and therefore aliases the same state.

Class identity must be nominal: two classes with identical fields remain distinct types. Structural object types are a separate language feature.

## Why embedding alone is insufficient

Embedding provides field/method promotion, but:

- `*Derived` is not assignable to `*Base`.
- Calls from base implementation do not automatically dispatch to a derived replacement.
- Base/derived identity and checked downcasts are not modeled.
- Constructor and override rules remain undefined.

Therefore OnsenTamago cannot call embedding alone “inheritance.”

## Interface polymorphism

Ordinary interfaces map naturally to Go interfaces. A class pointer implements the generated method set, and an interface value carries dynamic type information and the concrete reference. This provides the dispatch mechanism for explicit virtual methods and imported Go interface conformance.

## Implemented single-inheritance lowering

The compiler stores base state plus an internal self interface used only for virtual calls:

```go
type animalVirtual interface {
    Speak() string
}

type Animal struct {
    root any
    self animalVirtual
    name string
}

func initAnimal(base *Animal, self animalVirtual, name string) {
    base.self = self
    base.name = name
}

func (a *Animal) Describe() string {
    return a.name + ": " + a.self.Speak()
}

type Dog struct {
    Animal
}

func NewDog(name string) *Dog {
    result := &Dog{}
    initAnimal(&result.Animal, result, name)
    return result
}

func (d *Dog) Speak() string { return "woof" }
```

This preserves one derived object and one embedded base-state region. Direct nonvirtual calls remain ordinary methods. Virtual calls from base implementation dispatch through the internal interface.

Source-level base references remain `*Animal` in generated Go. An implicit
derived-to-base conversion calls a generated helper that evaluates once,
returns nil for a nil derived reference, and otherwise returns the address of
the embedded base-state region. Repeated upcasts therefore compare equal.
Virtual self state retains the most-derived dispatch target after construction.
The hierarchy root also stores the current constructed class reference as a Go
interface. Checked helpers inspect it through an unexported typed projection
interface, preserve nil, and recover the target's embedded view even when the
dynamic object is a deeper generic descendant. Generic projection arguments
must match exactly, so remapped and concretely fixed base types remain statically
safe. Forced helpers reuse the checked operation and panic on failure. This adds
identity metadata only to class hierarchies; classes without inheritance retain
their direct pointer-only representation.

For cross-package Go callers, the compiler exposes explicit exported
upcast/downcast functions. Public virtual methods are wrappers over the internal
self interface, while `super` targets a separate unexported direct slot. This
preserves dynamic dispatch for ordinary calls and method values obtained from a
base pointer outside the generated package. Nil receivers and Go-created zero
values fall back to the base direct slot rather than dereferencing an
uninitialized self interface.

## `super`

`super.method()` should lower to a statically selected base implementation, not virtual dispatch. Base constructor calls should lower to explicit initialization functions in a fixed order.

## Why virtual is explicit

- Normal methods use direct calls.
- Only declared virtual methods participate in interface dispatch.
- Derived replacements require `override`.
- Final classes/methods can omit virtual state and dispatch.
- Accidental method-name reuse is diagnosed.

This keeps runtime cost visible and makes generated Go easier to understand.

## Runtime cost

### Reference class without inheritance

- One Go pointer per reference.
- Ordinary pointer-receiver calls.
- No custom runtime overhead.
- Ordinary Go escape analysis and GC.

### Interface polymorphism

- Go interface value representation and interface dispatch.
- Potentially less inlining than concrete calls.

### Inheritance with virtual methods

- Additional internal self-interface state in inheritable base objects.
- Interface dispatch only for virtual calls.
- No added cost for final/nonvirtual designs when specialized lowering is used.

## Rejected alternatives

- **Custom vtable**: more unsafe machinery and less idiomatic/debuggable Go.
- **Flatten every derived class**: duplicates base layout/implementation and complicates base identity.
- **Call embedding inheritance**: does not satisfy subtype or virtual-call semantics.

## Hard cases

- Virtual calls during construction before derived initialization is complete.
- Typed nil inside interface values.
- Reference equality across upcasts.
- Public Go API shape for base references.
- Generic classes and pointer receiver constraints.
- JSON/serialization layout.
- Checked downcast errors and exhaustiveness.

Multiple inheritance is intentionally excluded because it introduces diamond state, collision resolution, constructor ordering, identity ambiguity, and significantly worse Go APIs.

## Validation requirements

The inheritance work is gated by executable coverage of:

1. Base/derived construction and field initialization order. (implemented)
2. Direct, virtual, override, and `super` calls. (implemented)
3. Base upcast and checked-downcast identity. (implemented)
4. Nil and equality behavior. (implemented)
5. Two- and three-level inheritance. (implemented)
6. Multiple interface implementation. (implemented)
7. Final/nonvirtual controls. (implemented; additional final-aware optimization remains optional)
8. Public Go API consumption. (implemented)
9. JSON and generic interactions. (implemented)
10. Race and allocation profiles. (race gate implemented; allocation profile remains)

The initial single-inheritance stage is implemented and differentially tested.
The remaining prototype requirements apply to optional final-aware optimization
and allocation profiling.
