package patch

import (
	"encoding/json"
	"strings"

	"github.com/scim2/filter-parser/v2"
)

// MemberAction identifies what a "members" PATCH operation does.
type MemberAction int

const (
	// MemberAdd adds a member to the group.
	MemberAdd MemberAction = iota
	// MemberRemove removes a member from the group.
	MemberRemove
)

func (op MemberAction) String() string {
	switch op {
	case MemberAdd:
		return "Add"
	case MemberRemove:
		return "Remove"
	default:
		return "Unknown"
	}
}

// MemberOp is a single, self-describing membership change extracted from a
// SCIM PATCH request: it names exactly which member to add or remove, so
// applying it never requires knowing the rest of the group's membership.
type MemberOp struct {
	Action MemberAction
	Value  string
}

// ExtractMemberOps parses a SCIM PATCH request payload and returns every
// membership change it contains, as a flat list of self-describing
// add/remove operations, in request order.
//
// It succeeds (ok=true) only if the ENTIRE request can be expressed this
// way. It returns ok=false if the request contains anything that isn't a
// plain "add to members" or "remove a single member from members by value"
// operation - e.g. an operation on a non-"members" attribute (such as
// displayName), a bulk replace of the whole members array, a no-filter
// "remove everything", or a malformed payload - since those either require
// the current membership state to apply, or aren't safe to guess at, and
// can't be expressed as standalone MemberOps. Callers should fall back to
// the existing full-state PATCH flow in that case, which will surface any
// malformed-payload error on its own.
func ExtractMemberOps(patchData []byte) ([]MemberOp, bool) {
	var p patch
	if err := json.Unmarshal(patchData, &p); err != nil {
		return nil, false
	}

	var ops []MemberOp
	for _, op := range p.Operations {
		attrPath, expr, subAttr, err := parsePath(op.Path)
		if err != nil || attrPath.AttributeName != "members" {
			return nil, false
		}
		if subAttr != nil {
			return nil, false
		}

		switch strings.ToLower(op.Op) {
		case opAdd:
			if expr != nil {
				return nil, false
			}
			values, ok := memberValuesFromAddPayload(op.Value)
			if !ok {
				return nil, false
			}
			for _, v := range values {
				ops = append(ops, MemberOp{Action: MemberAdd, Value: v})
			}

		case opRemove:
			value, ok := singleEqualsFilterValue(expr)
			if !ok {
				return nil, false
			}
			ops = append(ops, MemberOp{Action: MemberRemove, Value: value})

		default:
			// opReplace on members (bulk replace of the whole array), or
			// anything else - these need the full membership state.
			return nil, false
		}
	}
	return ops, true
}

// memberValuesFromAddPayload extracts the member "value" fields from the
// payload of an "add" operation on the members attribute.
// Example payload: [{"value":"alice"},{"value":"bob"}]
func memberValuesFromAddPayload(value any) ([]string, bool) {
	arr, ok := value.([]any)
	if !ok {
		return nil, false
	}
	values := make([]string, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, false
		}
		v, ok := m["value"].(string)
		if !ok || v == "" {
			return nil, false
		}
		values = append(values, v)
	}
	return values, true
}

// singleEqualsFilterValue returns the compared value if expr is exactly a
// plain  value from EQ filter. For instance `value eq "alice"` is extracted to "alice", true
// otherwise if false is returned indicating expression parsing can't be applied.
func singleEqualsFilterValue(expr filter.Expression) (string, bool) {
	ae, ok := expr.(*filter.AttributeExpression)
	if !ok || ae.Operator != filter.EQ || ae.AttributePath.AttributeName != "value" {
		return "", false
	}
	v, ok := ae.CompareValue.(string)
	return v, ok
}
