package ast

// NativeContract retains source distinctions erased by Go storage. Parameters
// are supplied by the native caller, not guessed from reflect.TypeFor[T].
// A missing witness is deliberately not a proof of any source contract.
type NativeContract struct {
	Tag       string
	Parameter string
	Parts     []NativeContract
}
