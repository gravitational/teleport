package tns

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/gravitational/trace"
)

// Node holds a key-value pair and any child nodes (if value is another list).
// ParseErr (if set) indicates a problem specifically encountered in this node.
type Node struct {
	Key      string
	Value    string
	Children []Node
	ParseErr string
}

// String returns a string representation of the Node in a compact parenthesized format.
func (n *Node) String() string {
	// If there's an error at this node, just note it briefly.
	if n.ParseErr != "" {
		return fmt.Sprintf("(%s=?ERROR? %v)", n.Key, n.ParseErr)
	}
	// If there are children, expand them recursively.
	if len(n.Children) > 0 {
		var sb strings.Builder
		sb.WriteString("(" + n.Key + "=")
		for _, child := range n.Children {
			sb.WriteString(child.String())
		}
		sb.WriteString(")")
		return sb.String()
	}
	// Otherwise, it's just (Key=Value).
	return fmt.Sprintf("(%s=%s)", n.Key, n.Value)
}

// Path returns sub node matching given sequence of keys. Returns nil if not found.
func (n *Node) Path(keys ...string) *Node {
	if len(keys) == 0 {
		return n
	}
	key := keys[0]
	for _, child := range n.Children {
		if child.Key == key {
			return child.Path(keys[1:]...)
		}
	}
	return nil
}

// GetValue is nil-safe Value accessor.
func (n *Node) GetValue() string {
	if n == nil {
		return ""
	}
	return n.Value
}

// ParseNodes scans the entire string for top-level parentheses and parses
// each one into a Node.
func ParseNodes(input string) ([]Node, error) {
	p := &parser{input: []rune(input)}
	var nodes []Node
	for !p.done() {
		node := p.parseNode()
		if node.ParseErr != "" {
			return nil, trace.BadParameter("failed to parse expression %q", node.ParseErr)
		}
		nodes = append(nodes, node)
		// trim whitespace after the last node or before the next one.
		p.skipWhitespace()
	}
	return nodes, nil
}

// Parser is responsible for scanning and parsing.
type parser struct {
	input    []rune
	position int
}

// parseValue populates the value of the node.
// Value can be either a simple string e.g. "FOO BAR BAZ" or a list of nodes, e.g. "(FOO=BAR)(BAZ=BAR)".
func (p *parser) parseValue(node *Node) {
	// find the end of value; two options:
	// - end of input
	// - unmatched ')'.
	balance := 0
	paren := false

	value := p.readUntil(func(char rune) bool {
		switch char {
		case '(':
			paren = true
			balance++
		case ')':
			balance--
		}
		return balance < 0
	})

	// if there were no parentheses, simply return the value as plain string.
	// otherwise parse it recursively.
	if !paren {
		node.Value = strings.TrimSpace(value)
		return
	}
	nodes, err := ParseNodes(value)
	if err != nil {
		node.ParseErr = err.Error()
		return
	}
	node.Children = nodes
}

// parseNode parses a single node (KEY=VALUE).
// VALUE may be either simple string or a list of nodes.
// Trims redundant whitespace.
// In case of errors Node.ParseErr will be present and other values may be missing.
func (p *parser) parseNode() (node Node) {
	p.skipWhitespace()
	if !p.match('(') {
		if p.done() {
			node.ParseErr = "reading key: unexpected end of input"
			return
		}
		node.ParseErr = fmt.Sprintf("expected ')', got %q", p.current())
		return
	}

	// Read key. Expect to stop on '=', but will also stop on ')' to avoid consuming too much input in case of errors.
	rawKey := p.readUntil(func(r rune) bool {
		return r == '=' || r == ')'
	})

	// key cannot contain whitespace, but can be surrounded by it.
	node.Key = strings.TrimSpace(rawKey)
	if len(node.Key) == 0 {
		node.ParseErr = "key missing"
		return
	}
	if strings.ContainsFunc(node.Key, unicode.IsSpace) {
		node.ParseErr = "key contains space"
		return
	}

	// possible options: '=', ')' or end of input.
	if !p.match('=') {
		if p.done() {
			node.ParseErr = "reading value: unexpected end of input"
			return
		}
		node.ParseErr = fmt.Sprintf("expected '=', got %q", p.current())
		return
	}

	// parse value into node.
	p.parseValue(&node)

	// closing ')'
	if !p.match(')') {
		if p.done() {
			node.ParseErr = "reading closing parens: unexpected end of input"
			return
		}
		node.ParseErr = fmt.Sprintf("expected ')', got %q", p.current())
		return
	}
	return
}

// readUntil reads characters until stopFn returns true or input is done.
func (p *parser) readUntil(stopFn func(r rune) bool) string {
	start := p.position
	for !p.done() && !stopFn(p.current()) {
		p.advance()
	}
	return string(p.input[start:p.position])
}

// skipWhitespace advances over any whitespace.
func (p *parser) skipWhitespace() {
	_ = p.readUntil(func(r rune) bool { return !unicode.IsSpace(r) })
}

// match consumes the next rune if it matches r, returning true; otherwise false.
func (p *parser) match(r rune) bool {
	if p.done() {
		return false
	}
	if p.current() == r {
		p.advance()
		return true
	}
	return false
}

// current returns the current rune.
func (p *parser) current() rune {
	if p.done() {
		// this never actually happens since we always
		// check p.done() before calling p.current().
		return 0
	}
	return p.input[p.position]
}

// advance moves one rune forward in the input.
func (p *parser) advance() {
	if !p.done() {
		p.position++
	}
}

// done returns true if we've consumed all runes.
func (p *parser) done() bool {
	return p.position >= len(p.input)
}
