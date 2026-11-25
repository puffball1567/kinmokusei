package sema

import (
	"slices"
	"sort"
	"strconv"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

type matchProductTest struct {
	path         string
	alternative  int
	alternatives int
	enum         bool
}

type matchEnumAlternatives map[*enumSymbol]map[string]int

func (c *Checker) productMatchExhaustive(patterns []*ast.MatchPattern, value Type) (exhaustive, usesEnums bool) {
	enums := make(matchEnumAlternatives)
	var rows [][]matchProductTest
	for _, pattern := range patterns {
		var row []matchProductTest
		if c.collectMatchProductTests(pattern, value, "", &row, enums) {
			sort.Slice(row, func(i, j int) bool { return row[i].path < row[j].path })
			rows = append(rows, row)
			for _, test := range row {
				usesEnums = usesEnums || test.enum
			}
		}
	}
	// Splitting a union of partial rows can be exponential. Stop the proof on
	// large inputs and keep the existing fallback requirement in that case.
	work := 1 << 16
	return coversMatchProduct(rows, &work, 128), usesEnums
}

// Flatten checked patterns into tests of fields and indices copied with the
// subject. A guard can mutate shared slice contents, so a slice only counts
// here when its pattern matches all contents and lengths. Omit any row with
// a test we cannot represent.
func (c *Checker) collectMatchProductTests(pattern *ast.MatchPattern, value Type, path string, row *[]matchProductTest, enums matchEnumAlternatives) bool {
	switch pattern.Kind {
	case ast.MatchWildcard, ast.MatchBinding:
		return true
	case ast.MatchNonNullBinding:
		if value.Kind == Nullable {
			*row = append(*row, matchProductTest{path: path, alternative: 1, alternatives: 2})
			return true
		}
	case ast.MatchLiteral:
		if literal, ok := pattern.Literal.(*ast.LiteralExpr); ok {
			if literal.Kind == ast.BooleanLiteral && value.IsBoolean() {
				index := 0
				if literal.Text == "true" {
					index = 1
				}
				*row = append(*row, matchProductTest{path: path, alternative: index, alternatives: 2})
				return true
			}
			if literal.Kind == ast.NullLiteral && value.Kind == Nullable {
				*row = append(*row, matchProductTest{path: path, alternative: 0, alternatives: 2})
				return true
			}
		}
		if enum := c.matchEnum(value); enum != nil {
			return c.collectMatchProductEnum(pattern.Literal, enum, path, row, enums)
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
			if !found || !c.collectMatchProductTests(&field.Pattern, direct.typeInfo, path+"."+field.Name, row, enums) {
				return false
			}
		}
		return true
	case ast.MatchSequence:
		return c.collectMatchProductSequence(pattern, value, path, row, enums)
	}
	return false
}

func (c *Checker) collectMatchProductEnum(literal ast.Expression, enum *enumSymbol, path string, row *[]matchProductTest, enums matchEnumAlternatives) bool {
	key, known := c.matchEnumConstant(literal)
	if !known {
		return false
	}
	values, cached := enums[enum]
	if !cached {
		c.ensureEnumChecked(enum.declaration)
		values = make(map[string]int, len(enum.declaration.Members))
		for _, member := range enum.declaration.Members {
			if _, exists := values[member.ResolvedValue]; member.ResolvedValue != "" && !exists {
				values[member.ResolvedValue] = len(values)
			}
		}
		enums[enum] = values
	}
	index, declared := values[key]
	if !declared {
		return false
	}
	*row = append(*row, matchProductTest{path: path, alternative: index, alternatives: len(values), enum: true})
	return true
}

func (c *Checker) collectMatchProductSequence(pattern *ast.MatchPattern, value Type, path string, row *[]matchProductTest, enums matchEnumAlternatives) bool {
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
		if !c.collectMatchProductTests(&pattern.Elements[i], *value.Element, path+"["+strconv.FormatInt(index, 10)+"]", row, enums) {
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
	var next matchProductTest
	for i, row := range rows {
		if len(row) == 0 {
			return true
		}
		if i == 0 || row[0].path < next.path {
			next = row[0]
		}
	}
	var wildcards [][]matchProductTest
	branches := make(map[int][][]matchProductTest)
	var alternatives []int
	for _, row := range rows {
		test := row[0]
		if test.path != next.path {
			wildcards = append(wildcards, row)
			continue
		}
		if len(branches[test.alternative]) == 0 {
			alternatives = append(alternatives, test.alternative)
		}
		branches[test.alternative] = append(branches[test.alternative], row[1:])
	}
	// All unmentioned alternatives have the same set of candidate rows.
	if len(branches) < next.alternatives && !coversMatchProduct(wildcards, work, depth-1) {
		return false
	}
	for _, alternative := range alternatives {
		if !coversMatchProduct(slices.Concat(wildcards, branches[alternative]), work, depth-1) {
			return false
		}
	}
	return true
}
