package source

import "testing"

func TestSpanMergeMatrix(t *testing.T) {
	left := Span{Path: "a.km", Start: Position{Offset: 1, Line: 1, Column: 2}, End: Position{Offset: 2, Line: 1, Column: 3}}
	right := Span{Path: "a.km", Start: Position{Offset: 4, Line: 2, Column: 1}, End: Position{Offset: 8, Line: 2, Column: 5}}
	tests := []struct {
		name  string
		left  Span
		right Span
		want  Span
	}{
		{"same path", left, right, Span{Path: "a.km", Start: left.Start, End: right.End}},
		{"empty left", Span{}, right, right},
		{"empty right", left, Span{}, left},
		{"different path", left, Span{Path: "b.km", Start: right.Start, End: right.End}, left},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.left.Merge(test.right); got != test.want {
				t.Fatalf("Merge() = %#v, want %#v", got, test.want)
			}
		})
	}
}
