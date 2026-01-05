package expression

import (
	"strconv"
	"strings"
)

// Parser parses expression strings into AST.
type Parser struct {
	lexer     *Lexer
	curToken  Token
	peekToken Token
}

// NewParser creates a new Parser for the given input.
func NewParser(input string) *Parser {
	p := &Parser{lexer: NewLexer(input)}
	// Read two tokens to initialize curToken and peekToken
	p.nextToken()
	p.nextToken()
	return p
}

// nextToken advances to the next token.
func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.lexer.NextToken()
}

// Parse parses the expression and returns the AST.
func (p *Parser) Parse() (*ExpressionAST, error) {
	node, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	// Ensure we've consumed all tokens
	if p.curToken.Type != TokenEOF {
		return nil, NewParseError(p.curToken.Pos, "end of expression", p.curToken.Literal)
	}

	return &ExpressionAST{Root: node}, nil
}

// parseExpression parses an expression (handles OR - lowest precedence).
func (p *Parser) parseExpression() (Node, error) {
	return p.parseOr()
}

// parseOr parses OR expressions.
func (p *Parser) parseOr() (Node, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}

	for p.curToken.Type == TokenOR {
		op := p.curToken.Literal
		p.nextToken()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &LogicalNode{Left: left, Operator: strings.ToUpper(op), Right: right}
	}

	return left, nil
}

// parseAnd parses AND expressions.
func (p *Parser) parseAnd() (Node, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}

	for p.curToken.Type == TokenAND {
		op := p.curToken.Literal
		p.nextToken()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = &LogicalNode{Left: left, Operator: strings.ToUpper(op), Right: right}
	}

	return left, nil
}

// parseNot parses NOT expressions.
func (p *Parser) parseNot() (Node, error) {
	if p.curToken.Type == TokenNOT {
		p.nextToken()
		operand, err := p.parseNot() // NOT is right-associative
		if err != nil {
			return nil, err
		}
		return &NotNode{Operand: operand}, nil
	}

	return p.parseComparison()
}

// parseComparison parses comparison expressions.
func (p *Parser) parseComparison() (Node, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	if isComparisonOperator(p.curToken.Type) {
		op := p.curToken.Literal
		p.nextToken()
		right, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return &ComparisonNode{Left: left, Operator: op, Right: right}, nil
	}

	return left, nil
}

// parsePrimary parses primary expressions (literals, variables, parenthesized expressions).
func (p *Parser) parsePrimary() (Node, error) {
	switch p.curToken.Type {
	case TokenLParen:
		p.nextToken() // consume '('
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if p.curToken.Type != TokenRParen {
			return nil, NewParseError(p.curToken.Pos, ")", p.curToken.Literal)
		}
		p.nextToken() // consume ')'
		return expr, nil

	case TokenVarRef:
		node := &VariableNode{Name: p.curToken.Literal}
		p.nextToken()
		return node, nil

	case TokenInt:
		val, err := strconv.ParseInt(p.curToken.Literal, 10, 64)
		if err != nil {
			return nil, NewExpressionError(p.curToken.Pos, "invalid integer: "+p.curToken.Literal, err)
		}
		node := &LiteralNode{Value: val}
		p.nextToken()
		return node, nil

	case TokenFloat:
		val, err := strconv.ParseFloat(p.curToken.Literal, 64)
		if err != nil {
			return nil, NewExpressionError(p.curToken.Pos, "invalid float: "+p.curToken.Literal, err)
		}
		node := &LiteralNode{Value: val}
		p.nextToken()
		return node, nil

	case TokenString:
		node := &LiteralNode{Value: p.curToken.Literal}
		p.nextToken()
		return node, nil

	case TokenBool:
		val := strings.ToLower(p.curToken.Literal) == "true"
		node := &LiteralNode{Value: val}
		p.nextToken()
		return node, nil

	case TokenIdent:
		// Treat bare identifiers as variable references
		node := &VariableNode{Name: p.curToken.Literal}
		p.nextToken()
		return node, nil

	case TokenEOF:
		return nil, NewParseError(p.curToken.Pos, "expression", "end of input")

	default:
		return nil, NewParseError(p.curToken.Pos, "expression", p.curToken.Literal)
	}
}

// isComparisonOperator returns true if the token is a comparison operator.
func isComparisonOperator(t TokenType) bool {
	switch t {
	case TokenEQ, TokenNE, TokenLT, TokenGT, TokenLE, TokenGE:
		return true
	default:
		return false
	}
}

// ParseExpression is a convenience function to parse an expression string.
func ParseExpression(input string) (*ExpressionAST, error) {
	parser := NewParser(input)
	return parser.Parse()
}
