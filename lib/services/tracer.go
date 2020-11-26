package services

// Tracer traces decisions made by role set
type Tracer interface {
	// AddChild adds a child node
	AddChild(interface{}) Tracer
	// Write records fields
	Write(...interface{})
	// Visit vists tree recursively,
	// can be called concurrently, assuming
	// there are no concurrent writes
	Visit(Visitor)
}

// Visitor visits node
type Visitor interface {
	VisitNode(n Node)
}

// Node is tree based implementation of tracer. It's write methods
// can not be called concurrently without external locking.
type Node struct {
	// ID identifies this node
	ID interface{}
	// Fields contains fields recorded for this node
	Fields []interface{}
	// Children contains child nodes
	Children []Node
}

// Visit vists tree recursively,
// can be called concurrently, assuming
// there are no concurrent writes
func (n *Node) Visit(v Visitor) {
	v.VisitNode(*n)
	for _, c := range n.Children {
		v.VisitNode(c)
	}
}

// AddChild adds a child node
func (n *Node) AddChild(id interface{}) Tracer {
	c := Node{ID: id}
	n.Children = append(n.Children, c)
	return n
}

// Write records fields for this node. When called many times, appends fields.
func (n *Node) Write(in ...interface{}) {
	if len(n.Fields) == 0 {
		n.Fields = make([]interface{}, 0, len(in))
	}
	n.Fields = append(n.Fields, in...)
}

type ConditionTrace struct {
	Role string
	Cond RoleConditionType
}

type MatchTrace struct {
	Result   bool
	Message  string
	Error    error
	Input    interface{}
	Selector interface{}
}

type ServerAccessTrace struct {
	Login   string
	Server  Server
	RoleSet RoleSet
}

type AppAccessTrace struct {
	App *App
}

type ResourceAccessTrace struct {
	Verb     string
	Resource Resource
	RoleSet  RoleSet
}

// DiscardTracer discards all calls and records nothing
type DiscardTracer struct {
}

// Visit does nothing for discard visitor
func (d *DiscardTracer) Visit(v Visitor) {
}

// AddChild adds a child node
func (d *DiscardTracer) AddChild(interface{}) Tracer {
	return d
}

// Write records fields
func (d *DiscardTracer) Write(...interface{}) {
}
