package sema

import (
	"sort"
	"strconv"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

type matchProductTest struct {
	path string
	set  bool // true or non-null, depending on the type at path
}

func (c *Checker) productMatchExhaustive(patterns []*ast.MatchPattern, value Type) bool {
	if len(patterns) < 2 {
		return false
	}
	var rows [][]matchProductTest
	for _, pattern := range patterns {
		var row []matchProductTest
		if c.collectMatchProductTests(pattern, value, "", &row) {
			sort.Slice(row, func(i, j int) bool { return row[i].path < row[j].path })
			rows = append(rows, row)
		}
	}
	// Splitting a union of partial rows can be exponential. Stop the proof on
	// large inputs and keep the existing fallback requirement in that case.
	work := 1 << 16
	return coversMatchProduct(rows, &work, 128)
}

// Flatten checked patterns into tests of fields and indices copied with the
// subject. A guard can mutate shared slice contents, so a slice only counts
// here when its pattern matches all contents and lengths. Omit any row with
// a test we cannot represent.
func (c *Checker) collectMatchProductTests(pattern *ast.MatchPattern, value Type, path string, row *[]matchProductTest) bool {
	switch pattern.Kind {
	case ast.MatchWildcard, ast.MatchBinding:
		return true
	case ast.MatchNonNullBinding:
		if value.Kind == Nullable {
			*row = append(*row, matchProductTest{path: path, set: true})
			return true
		}
	case ast.MatchLiteral:
		if literal, ok := pattern.Literal.(*ast.LiteralExpr); ok {
			if literal.Kind == ast.BooleanLiteral && value.IsBoolean() {
				*row = append(*row, matchProductTest{path: path, set: literal.Text == "true"})
				return true
			}
			if literal.Kind == ast.NullLiteral && value.Kind == Nullable {
				*row = append(*row, matchProductTest{path: path, set: false})
				return true
			}
		}
	case ast.MatchRecord:
		if shape, native := c.nativeDefinedShape(value); native {
			value = shape
		}
		if value.Kind != Object && value.Kind != Struct {
			return false
		}
		for i := range pattern.Fields {
			field := &pattern.Fields[i]
			direct, found := c.matchRecordField(value, field.Name)
			if !found || !c.collectMatchProductTests(&field.Pattern, direct.typeInfo, path+"."+field.Name, row) {
				return false
			}
		}
		return true
	case ast.MatchSequence:
		return c.collectMatchProductSequence(pattern, value, path, row)
	}
	return false
}

func (c *Checker) collectMatchProductSequence(pattern *ast.MatchPattern, value Type, path string, row *[]matchProductTest) bool {
	rest := pattern.SequenceRestIndex()
	size := len(pattern.Elements)
	if !pattern.FixedLength {
		return pattern.CoversSliceLength && size == 1 && rest == 0
	}
	value = c.constraintArgumentShape(value)
	if value.Kind != FixedArray || value.Element == nil {
		return false
	}
	if rest < 0 && int64(size) != value.Length || rest >= 0 && int64(size-1) > value.Length {
		return false
	}
	for i := range pattern.Elements {
		if i == rest {
			continue
		}
		index := int64(i)
		if rest >= 0 && i > rest {
			index = value.Length - int64(size-i)
		}
		if !c.collectMatchProductTests(&pattern.Elements[i], *value.Element, path+"["+strconv.FormatInt(index, 10)+"]", row) {
			return false
		}
	}
	return true
}

func coversMatchProduct(rows [][]matchProductTest, work *int, depth int) bool {
	if len(rows) == 0 || *work < len(rows) || depth == 0 {
		return false
	}
	*work -= len(rows)
	var next string
	for i, row := range rows {
		if len(row) == 0 {
			return true
		}
		if i == 0 || row[0].path < next {
			next = row[0].path
		}
	}
	if len(rows) == 1 {
		return false
	}
	var unset, set [][]matchProductTest
	for _, row := range rows {
		test := row[0]
		switch {
		case test.path != next:
			unset = append(unset, row)
			set = append(set, row)
		case test.set:
			set = append(set, row[1:])
		default:
			unset = append(unset, row[1:])
		}
	}
	return coversMatchProduct(unset, work, depth-1) && coversMatchProduct(set, work, depth-1)
}
