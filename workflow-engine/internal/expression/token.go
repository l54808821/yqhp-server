// Package expression provides condition expression parsing and evaluation.
package expression

// TokenType represents the type of a token.
type TokenType int

const (
	// Special tokens
	TokenEOF TokenType = iota
	TokenIllegal

	// Literals
	TokenIdent  // variable name
	TokenInt    // integer literal
	TokenFloat  // float literal
	TokenString // string literal
	TokenBool   // true/false

	// Variable reference
	TokenVarRef // ${...}

	// Operators
	TokenEQ // ==
	TokenNE // !=
	TokenLT // <
	TokenGT // >
	TokenLE // <=
	TokenGE // >=

	// Logical operators
	TokenAND // AND
	TokenOR  // OR
	TokenNOT // NOT

	// Delimiters
	TokenLParen // (
	TokenRParen // )
)

// String returns the string representation of the token type.
func (t TokenType) String() string {
	switch t {
	case TokenEOF:
		return "EOF"
	case TokenIllegal:
		return "ILLEGAL"
	case TokenIdent:
		return "IDENT"
	case TokenInt:
		return "INT"
	case TokenFloat:
		return "FLOAT"
	case TokenString:
		return "STRING"
	case TokenBool:
		return "BOOL"
	case TokenVarRef:
		return "VARREF"
	case TokenEQ:
		return "=="
	case TokenNE:
		return "!="
	case TokenLT:
		return "<"
	case TokenGT:
		return ">"
	case TokenLE:
		return "<="
	case TokenGE:
		return ">="
	case TokenAND:
		return "AND"
	case TokenOR:
		return "OR"
	case TokenNOT:
		return "NOT"
	case TokenLParen:
		return "("
	case TokenRParen:
		return ")"
	default:
		return "UNKNOWN"
	}
}

// Token represents a lexical token.
type Token struct {
	Type    TokenType
	Literal string
	Pos     int // Position in the input string
}
