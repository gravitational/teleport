package services

type Tracer interface {
	// AddChild adds a child clause
	AddChild(v interface{}) Tracer
	// SetMatch sets match result for this clause
	SetMatch(Match)
	// Visit vists clause recursively,
	// can be called concurrently, assuming
	// there are no concurrent writes
	Visit(Visitor)
}

// Visitor visits clasuses
type Visitor interface {
	VisitNode(n Node)
}

const (
	// TracerCaptureAll collects all levels
	// it is capped at 1K levels to be on the safe side
	TracerCaptureAll = 1000
	// TracerDiscardAllLevels discards all levels
	TracerDiscardAll = iota
)

// NewLevelTracer returns new level tracer, if the level is TracerCaptureAll(-1)
// it will capture all the traces.
//
// When it's set TracerDiscardAll(0), it will discard all levels.
//
// 1 - one trace level, 2 - trace levels and so on, it maxes out at 1K levels
//
func NewLevelTracer(captureLevels int) Tracer {
	return &Node{
		remainingLevels: captureLevels,
	}
}

// Node holds traces in a tree
type Node struct {
	captureLevels int
	// Value contains node value used for debug purposes
	Value interface{}
	// Children contains child nodes with clauses
	Children []Node
	// Match contains match result
	Match Match
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

func (n *Node) AddChild(v interface{}) Tracer {
	if n.remainingLevels <= 0 {
		return &DiscardTracer{}
	}
	c := Node{Value: v, Level: n.Level + 1, captureLevels: n.remainingLevels - 1}
	n.Children = append(n.Children, c)
	return n
}

// SetMatch sets match result. When called many times, overwrites previous result
func (n *Node) SetMatch(m Match) {
	n.Match = m
}

type RuleTrace struct {
	Rule     Rule
	Resource string
}

type ConditionTrace struct {
	Role string
	Cond RoleConditionType
}

type Match struct {
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

// SetMatch discards all values
func (d *DiscardTracer) SetMatch(Match) {
}
