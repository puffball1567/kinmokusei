package source

// Position identifies a byte offset and its one-based line and column.
type Position struct {
	Offset int
	Line   int
	Column int
}

// Span identifies a half-open region in a source file.
type Span struct {
	Path  string
	Start Position
	End   Position
}

func (s Span) Merge(other Span) Span {
	if s.Path == "" {
		return other
	}
	if other.Path == "" || s.Path != other.Path {
		return s
	}
	return Span{Path: s.Path, Start: s.Start, End: other.End}
}
