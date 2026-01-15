package expression

import (
	"strings"
	"unicode"
)

// Lexer tokenizes expression strings.
type Lexer struct {
	input   string
	pos     int  // current position in input
	readPos int  // current reading position (after current char)
	ch      byte // current char under examination
}

// NewLexer creates a new Lexer for the given input.
func NewLexer(input string) *Lexer {
	l := &Lexer{input: input}
	l.readChar()
	return l
}

// readChar reads the next character and advances the position.
func (l *Lexer) readChar() {
	if l.readPos >= len(l.input) {
		l.ch = 0 // ASCII NUL signifies EOF
	} else {
		l.ch = l.input[l.readPos]
	}
	l.pos = l.readPos
	l.readPos++
}

// peekChar returns the next character without advancing the position.
func (l *Lexer) peekChar() byte {
	if l.readPos >= len(l.input) {
		return 0
	}
	return l.input[l.readPos]
}

// NextToken returns the next token from the input.
func (l *Lexer) NextToken() Token {
	l.skipWhitespace()

	var tok Token
	tok.Pos = l.pos

	switch l.ch {
	case '=':
		if l.peekChar() == '=' {
			l.readChar()
			// 支持 === (当作 == 处理)
			if l.peekChar() == '=' {
				l.readChar()
			}
			tok = Token{Type: TokenEQ, Literal: "==", Pos: tok.Pos}
		} else {
			tok = Token{Type: TokenIllegal, Literal: string(l.ch), Pos: tok.Pos}
		}
	case '!':
		if l.peekChar() == '=' {
			l.readChar()
			// 支持 !== (当作 != 处理)
			if l.peekChar() == '=' {
				l.readChar()
			}
			tok = Token{Type: TokenNE, Literal: "!=", Pos: tok.Pos}
		} else {
			tok = Token{Type: TokenIllegal, Literal: string(l.ch), Pos: tok.Pos}
		}
	case '<':
		if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TokenLE, Literal: "<=", Pos: tok.Pos}
		} else {
			tok = Token{Type: TokenLT, Literal: "<", Pos: tok.Pos}
		}
	case '>':
		if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TokenGE, Literal: ">=", Pos: tok.Pos}
		} else {
			tok = Token{Type: TokenGT, Literal: ">", Pos: tok.Pos}
		}
	case '(':
		tok = Token{Type: TokenLParen, Literal: "(", Pos: tok.Pos}
	case ')':
		tok = Token{Type: TokenRParen, Literal: ")", Pos: tok.Pos}
	case '$':
		if l.peekChar() == '{' {
			tok = l.readVarRef()
			return tok
		}
		tok = Token{Type: TokenIllegal, Literal: string(l.ch), Pos: tok.Pos}
	case '"', '\'':
		tok = l.readString()
		return tok
	case 0:
		tok = Token{Type: TokenEOF, Literal: "", Pos: tok.Pos}
	default:
		if isDigit(l.ch) || (l.ch == '-' && isDigit(l.peekChar())) {
			tok = l.readNumber()
			return tok
		} else if isLetter(l.ch) {
			tok = l.readIdentifier()
			return tok
		}
		tok = Token{Type: TokenIllegal, Literal: string(l.ch), Pos: tok.Pos}
	}

	l.readChar()
	return tok
}

// skipWhitespace skips whitespace characters.
func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
}

// readIdentifier reads an identifier or keyword.
func (l *Lexer) readIdentifier() Token {
	pos := l.pos
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}
	literal := l.input[pos:l.pos]
	tokType := lookupIdent(literal)
	return Token{Type: tokType, Literal: literal, Pos: pos}
}

// readNumber reads an integer or float literal.
func (l *Lexer) readNumber() Token {
	pos := l.pos
	isFloat := false

	// Handle negative sign
	if l.ch == '-' {
		l.readChar()
	}

	for isDigit(l.ch) {
		l.readChar()
	}

	// Check for decimal point
	if l.ch == '.' && isDigit(l.peekChar()) {
		isFloat = true
		l.readChar() // consume '.'
		for isDigit(l.ch) {
			l.readChar()
		}
	}

	literal := l.input[pos:l.pos]
	if isFloat {
		return Token{Type: TokenFloat, Literal: literal, Pos: pos}
	}
	return Token{Type: TokenInt, Literal: literal, Pos: pos}
}

// readString reads a string literal.
func (l *Lexer) readString() Token {
	pos := l.pos
	quote := l.ch
	l.readChar() // consume opening quote

	start := l.pos
	for l.ch != quote && l.ch != 0 {
		l.readChar()
	}

	literal := l.input[start:l.pos]
	if l.ch == quote {
		l.readChar() // consume closing quote
	}

	return Token{Type: TokenString, Literal: literal, Pos: pos}
}

// readVarRef reads a variable reference ${...}.
func (l *Lexer) readVarRef() Token {
	pos := l.pos
	l.readChar() // consume '$'
	l.readChar() // consume '{'

	start := l.pos
	depth := 1
	for depth > 0 && l.ch != 0 {
		if l.ch == '{' {
			depth++
		} else if l.ch == '}' {
			depth--
		}
		if depth > 0 {
			l.readChar()
		}
	}

	literal := l.input[start:l.pos]
	if l.ch == '}' {
		l.readChar() // consume closing '}'
	}

	return Token{Type: TokenVarRef, Literal: literal, Pos: pos}
}

// lookupIdent returns the token type for an identifier.
func lookupIdent(ident string) TokenType {
	upper := strings.ToUpper(ident)
	switch upper {
	case "AND":
		return TokenAND
	case "OR":
		return TokenOR
	case "NOT":
		return TokenNOT
	case "TRUE", "FALSE":
		return TokenBool
	default:
		return TokenIdent
	}
}

func isLetter(ch byte) bool {
	return unicode.IsLetter(rune(ch)) || ch == '_'
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}
