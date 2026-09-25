package ast

import "github.com/puffball1567/kinmokusei/internal/source"

// Names always retain the selected export spelling. BindingName/BindingSpan
// describe the local name, without changing declaration or runtime identity.
func (d ImportDecl) BindingName(index int) string {
	if index < len(d.NameAliases) && d.NameAliases[index] != "" {
		return d.NameAliases[index]
	}
	return d.Names[index]
}

func (d ImportDecl) BindingSpan(index int) source.Span {
	if index < len(d.NameAliasSpans) && d.NameAliasSpans[index].Path != "" {
		return d.NameAliasSpans[index]
	}
	if index < len(d.NameSpans) {
		return d.NameSpans[index]
	}
	return d.Span
}

func (d ImportDecl) HasNameAlias(index int) bool {
	return index < len(d.NameAliases) && d.NameAliases[index] != ""
}
