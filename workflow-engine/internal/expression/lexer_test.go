package expression

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLexer_BasicTokens(t *testing.T) {
	tests := []struct {
		input    string
		expected []Token
	}{
		{
			input: "==",
			expected: []Token{
				{Type: TokenEQ, Literal: "==", Pos: 0},
				{Type: TokenEOF, Literal: "", Pos: 2},
			},
		},
		{
			input: "!=",
			expected: []Token{
				{Type: TokenNE, Literal: "!=", Pos: 0},
				{Type: TokenEOF, Literal: "", Pos: 2},
			},
		},
		{
			input: "< > <= >=",
			expected: []Token{
				{Type: TokenLT, Literal: "<", Pos: 0},
				{Type: TokenGT, Literal: ">", Pos: 2},
				{Type: TokenLE, Literal: "<=", Pos: 4},
				{Type: TokenGE, Literal: ">=", Pos: 7},
				{Type: TokenEOF, Literal: "", Pos: 9},
			},
		},
		{
			input: "( )",
			expected: []Token{
				{Type: TokenLParen, Literal: "(", Pos: 0},
				{Type: TokenRParen, Literal: ")", Pos: 2},
				{Type: TokenEOF, Literal: "", Pos: 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lexer := NewLexer(tt.input)
			for _, expected := range tt.expected {
				tok := lexer.NextToken()
				assert.Equal(t, expected.Type, tok.Type, "token type mismatch")
				assert.Equal(t, expected.Literal, tok.Literal, "token literal mismatch")
			}
		})
	}
}

func TestLexer_LogicalOperators(t *testing.T) {
	tests := []struct {
		input    string
		expected []Token
	}{
		{
			input: "AND",
			expected: []Token{
				{Type: TokenAND, Literal: "AND", Pos: 0},
				{Type: TokenEOF, Literal: "", Pos: 3},
			},
		},
		{
			input: "and",
			expected: []Token{
				{Type: TokenAND, Literal: "and", Pos: 0},
				{Type: TokenEOF, Literal: "", Pos: 3},
			},
		},
		{
			input: "OR",
			expected: []Token{
				{Type: TokenOR, Literal: "OR", Pos: 0},
				{Type: TokenEOF, Literal: "", Pos: 2},
			},
		},
		{
			input: "NOT",
			expected: []Token{
				{Type: TokenNOT, Literal: "NOT", Pos: 0},
				{Type: TokenEOF, Literal: "", Pos: 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lexer := NewLexer(tt.input)
			for _, expected := range tt.expected {
				tok := lexer.NextToken()
				assert.Equal(t, expected.Type, tok.Type, "token type mismatch")
				assert.Equal(t, expected.Literal, tok.Literal, "token literal mismatch")
			}
		})
	}
}

func TestLexer_Literals(t *testing.T) {
	tests := []struct {
		input    string
		expected Token
	}{
		{input: "123", expected: Token{Type: TokenInt, Literal: "123"}},
		{input: "-456", expected: Token{Type: TokenInt, Literal: "-456"}},
		{input: "3.14", expected: Token{Type: TokenFloat, Literal: "3.14"}},
		{input: "-2.5", expected: Token{Type: TokenFloat, Literal: "-2.5"}},
		{input: `"hello"`, expected: Token{Type: TokenString, Literal: "hello"}},
		{input: `'world'`, expected: Token{Type: TokenString, Literal: "world"}},
		{input: "true", expected: Token{Type: TokenBool, Literal: "true"}},
		{input: "false", expected: Token{Type: TokenBool, Literal: "false"}},
		{input: "TRUE", expected: Token{Type: TokenBool, Literal: "TRUE"}},
		{input: "FALSE", expected: Token{Type: TokenBool, Literal: "FALSE"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lexer := NewLexer(tt.input)
			tok := lexer.NextToken()
			assert.Equal(t, tt.expected.Type, tok.Type, "token type mismatch")
			assert.Equal(t, tt.expected.Literal, tok.Literal, "token literal mismatch")
		})
	}
}

func TestLexer_VariableReferences(t *testing.T) {
	tests := []struct {
		input    string
		expected Token
	}{
		{input: "${a}", expected: Token{Type: TokenVarRef, Literal: "a"}},
		{input: "${login.status}", expected: Token{Type: TokenVarRef, Literal: "login.status"}},
		{input: "${env:API_URL}", expected: Token{Type: TokenVarRef, Literal: "env:API_URL"}},
		{input: "${secret:password}", expected: Token{Type: TokenVarRef, Literal: "secret:password"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lexer := NewLexer(tt.input)
			tok := lexer.NextToken()
			assert.Equal(t, tt.expected.Type, tok.Type, "token type mismatch")
			assert.Equal(t, tt.expected.Literal, tok.Literal, "token literal mismatch")
		})
	}
}

func TestLexer_ComplexExpression(t *testing.T) {
	input := `${login.status} == 200 AND ${response.body.success} == true`

	expected := []Token{
		{Type: TokenVarRef, Literal: "login.status"},
		{Type: TokenEQ, Literal: "=="},
		{Type: TokenInt, Literal: "200"},
		{Type: TokenAND, Literal: "AND"},
		{Type: TokenVarRef, Literal: "response.body.success"},
		{Type: TokenEQ, Literal: "=="},
		{Type: TokenBool, Literal: "true"},
		{Type: TokenEOF, Literal: ""},
	}

	lexer := NewLexer(input)
	for i, exp := range expected {
		tok := lexer.NextToken()
		assert.Equal(t, exp.Type, tok.Type, "token %d type mismatch", i)
		assert.Equal(t, exp.Literal, tok.Literal, "token %d literal mismatch", i)
	}
}

func TestLexer_Identifiers(t *testing.T) {
	tests := []struct {
		input    string
		expected Token
	}{
		{input: "foo", expected: Token{Type: TokenIdent, Literal: "foo"}},
		{input: "bar_baz", expected: Token{Type: TokenIdent, Literal: "bar_baz"}},
		{input: "test123", expected: Token{Type: TokenIdent, Literal: "test123"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lexer := NewLexer(tt.input)
			tok := lexer.NextToken()
			assert.Equal(t, tt.expected.Type, tok.Type, "token type mismatch")
			assert.Equal(t, tt.expected.Literal, tok.Literal, "token literal mismatch")
		})
	}
}

func TestTokenType_String(t *testing.T) {
	tests := []struct {
		tokenType TokenType
		expected  string
	}{
		{TokenEOF, "EOF"},
		{TokenIllegal, "ILLEGAL"},
		{TokenIdent, "IDENT"},
		{TokenInt, "INT"},
		{TokenFloat, "FLOAT"},
		{TokenString, "STRING"},
		{TokenBool, "BOOL"},
		{TokenVarRef, "VARREF"},
		{TokenEQ, "=="},
		{TokenNE, "!="},
		{TokenLT, "<"},
		{TokenGT, ">"},
		{TokenLE, "<="},
		{TokenGE, ">="},
		{TokenAND, "AND"},
		{TokenOR, "OR"},
		{TokenNOT, "NOT"},
		{TokenLParen, "("},
		{TokenRParen, ")"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.tokenType.String())
		})
	}
}
