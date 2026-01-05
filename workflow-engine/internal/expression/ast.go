package expression

// NodeType represents the type of an AST node.
type NodeType int

const (
	NodeTypeLiteral    NodeType = iota // Literal value (string, int, float, bool)
	NodeTypeVariable                   // Variable reference
	NodeTypeComparison                 // Comparison expression
	NodeTypeLogical                    // Logical expression (AND, OR)
	NodeTypeNot                        // NOT expression
)

// String returns the string representation of the node type.
func (n NodeType) String() string {
	switch n {
	case NodeTypeLiteral:
		return "Literal"
	case NodeTypeVariable:
		return "Variable"
	case NodeTypeComparison:
		return "Comparison"
	case NodeTypeLogical:
		return "Logical"
	case NodeTypeNot:
		return "Not"
	default:
		return "Unknown"
	}
}

// Node represents a node in the AST.
type Node interface {
	nodeType() NodeType
}

// LiteralNode represents a literal value.
type LiteralNode struct {
	Value any // string, int64, float64, or bool
}

func (n *LiteralNode) nodeType() NodeType { return NodeTypeLiteral }

// VariableNode represents a variable reference.
type VariableNode struct {
	Name string // Variable name or path (e.g., "login.status", "env:VAR")
}

func (n *VariableNode) nodeType() NodeType { return NodeTypeVariable }

// ComparisonNode represents a comparison expression.
type ComparisonNode struct {
	Left     Node
	Operator string // ==, !=, <, >, <=, >=
	Right    Node
}

func (n *ComparisonNode) nodeType() NodeType { return NodeTypeComparison }

// LogicalNode represents a logical expression (AND, OR).
type LogicalNode struct {
	Left     Node
	Operator string // AND, OR
	Right    Node
}

func (n *LogicalNode) nodeType() NodeType { return NodeTypeLogical }

// NotNode represents a NOT expression.
type NotNode struct {
	Operand Node
}

func (n *NotNode) nodeType() NodeType { return NodeTypeNot }

// ExpressionAST wraps the root node of an expression AST.
type ExpressionAST struct {
	Root Node
}
