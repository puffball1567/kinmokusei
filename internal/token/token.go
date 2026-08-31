package token

import "ontama.local/ontama/internal/source"

type Kind string

const (
	Illegal Kind = "illegal"
	EOF     Kind = "eof"

	Identifier Kind = "identifier"
	Integer    Kind = "integer"
	Float      Kind = "float"
	String     Kind = "string"

	Function    Kind = "function"
	Const       Kind = "const"
	Let         Kind = "let"
	Return      Kind = "return"
	If          Kind = "if"
	Else        Kind = "else"
	True        Kind = "true"
	False       Kind = "false"
	Import      Kind = "import"
	From        Kind = "from"
	While       Kind = "while"
	For         Kind = "for"
	Of          Kind = "of"
	Select      Kind = "select"
	Switch      Kind = "switch"
	Case        Kind = "case"
	Default     Kind = "default"
	Break       Kind = "break"
	Continue    Kind = "continue"
	Goto        Kind = "goto"
	Fallthrough Kind = "fallthrough"
	New         Kind = "new"
	Class       Kind = "class"
	Struct      Kind = "struct"
	Constructor Kind = "constructor"
	Public      Kind = "public"
	Private     Kind = "private"
	Protected   Kind = "protected"
	Static      Kind = "static"
	This        Kind = "this"
	Interface   Kind = "interface"
	Implements  Kind = "implements"
	Extends     Kind = "extends"
	Virtual     Kind = "virtual"
	Override    Kind = "override"
	Final       Kind = "final"
	Super       Kind = "super"
	Go          Kind = "go"
	Await       Kind = "await"
	Detach      Kind = "detach"
	Defer       Kind = "defer"
	Nil         Kind = "nil"
	Null        Kind = "null"
	As          Kind = "as"
	Export      Kind = "export"
	Try         Kind = "try"
	Catch       Kind = "catch"
	Finally     Kind = "finally"
	Throw       Kind = "throw"

	LeftParen    Kind = "("
	RightParen   Kind = ")"
	LeftBrace    Kind = "{"
	RightBrace   Kind = "}"
	Colon        Kind = ":"
	Comma        Kind = ","
	Semicolon    Kind = ";"
	LeftBracket  Kind = "["
	RightBracket Kind = "]"
	Dot          Kind = "."
	Ellipsis     Kind = "..."

	Assign        Kind = "="
	PlusAssign    Kind = "+="
	MinusAssign   Kind = "-="
	StarAssign    Kind = "*="
	SlashAssign   Kind = "/="
	PercentAssign Kind = "%="
	AndAssign     Kind = "&="
	OrAssign      Kind = "|="
	XorAssign     Kind = "^="
	AndNotAssign  Kind = "&^="
	ShlAssign     Kind = "<<="
	ShrAssign     Kind = ">>="
	Increment     Kind = "++"
	Decrement     Kind = "--"
	FatArrow      Kind = "=>"
	Plus          Kind = "+"
	Minus         Kind = "-"
	Star          Kind = "*"
	Slash         Kind = "/"
	Percent       Kind = "%"
	Bang          Kind = "!"
	Equal         Kind = "=="
	StrictEqual   Kind = "==="
	NotEqual      Kind = "!="
	StrictUnequal Kind = "!=="
	Less          Kind = "<"
	LeftArrow     Kind = "<-"
	LessEqual     Kind = "<="
	Greater       Kind = ">"
	GreaterEqual  Kind = ">="
	And           Kind = "&&"
	Or            Kind = "||"
	Ampersand     Kind = "&"
	Pipe          Kind = "|"
	Caret         Kind = "^"
	ShiftLeft     Kind = "<<"
	ShiftRight    Kind = ">>"
	AndNot        Kind = "&^"
	Question      Kind = "?"
)

var keywords = map[string]Kind{
	"function":    Function,
	"const":       Const,
	"let":         Let,
	"return":      Return,
	"if":          If,
	"else":        Else,
	"true":        True,
	"false":       False,
	"import":      Import,
	"from":        From,
	"while":       While,
	"for":         For,
	"of":          Of,
	"select":      Select,
	"switch":      Switch,
	"case":        Case,
	"default":     Default,
	"break":       Break,
	"continue":    Continue,
	"goto":        Goto,
	"fallthrough": Fallthrough,
	"new":         New,
	"class":       Class,
	"struct":      Struct,
	"constructor": Constructor,
	"public":      Public,
	"private":     Private,
	"protected":   Protected,
	"static":      Static,
	"this":        This,
	"interface":   Interface,
	"implements":  Implements,
	"extends":     Extends,
	"virtual":     Virtual,
	"override":    Override,
	"final":       Final,
	"super":       Super,
	"go":          Go,
	"await":       Await,
	"detach":      Detach,
	"defer":       Defer,
	"nil":         Nil,
	"null":        Null,
	"as":          As,
	"export":      Export,
	"try":         Try,
	"catch":       Catch,
	"finally":     Finally,
	"throw":       Throw,
}

func LookupIdentifier(text string) Kind {
	if kind, ok := keywords[text]; ok {
		return kind
	}
	return Identifier
}

type Token struct {
	Kind   Kind
	Lexeme string
	Span   source.Span
}
