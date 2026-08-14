package patch

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	"github.com/gravitational/trace"
	"github.com/scim2/filter-parser/v2"
)

const (
	opAdd     = "add"
	opReplace = "replace"
	opRemove  = "remove"
)

// patch represents an SCIM PATCH request as defined in RFC 7644 Section 3.5.2.
// It contains a list of operations to be applied to a resource.
type patch struct {
	// Schemas is the list of SCIM schemas for this PATCH request.
	// Typically contains "urn:ietf:params:scim:api:messages:2.0:PatchOp".
	Schemas []string `json:"schemas"`
	// Operations is the list of patch operations to apply to the resource.
	// Operations are applied sequentially in the order they appear.
	Operations []patchOperation `json:"Operations"`
}

// patchOperation represents a single SCIM PATCH operation.
// Each operation specifies how to modify a resource (add, replace, or remove).
type patchOperation struct {
	// Op is the operation type: "add", "replace", or "remove".
	Op string `json:"op"`
	// Path is the attribute path to modify (optional for some operations).
	// Examples: "userName", "emails[type eq \"work\"].value", "name.givenName"
	Path string `json:"path,omitempty"`
	// Value is the value to set (required for add/replace, not used for remove).
	Value any `json:"value,omitempty"`
}

type attributes = map[string]any

// Apply applies an SCIM PATCH operation as defined in RFC 7644 Section 3.5.2
// (https://datatracker.ietf.org/doc/html/rfc7644#section-3.5.2) to the given
// JSON-encoded SCIM resource.
//
// Note: The standard `jsonpatch` library (RFC 6902) cannot be used here,
// as it does not support SCIM filtering, attribute subpaths, or schema-based paths.
// SCIM PATCH has distinct semantics and structure that require a dedicated parser and evaluator.
//
// Example SCIM PATCH operations:
//
//  1. Add operation with simple path:
//     Operation: {"op": "add", "path": "userName", "value": "alice"}
//     Before:    {}
//     After:     {"userName": "alice"}
//
//  2. Replace operation with filter (updates matching array elements):
//     Operation: {"op": "replace", "path": "emails[type eq \"work\"].value", "value": "alice@work.com"}
//     Before:    {"emails": [{"type": "work", "value": "old@work.com"}]}
//     After:     {"emails": [{"type": "work", "value": "alice@work.com"}]}
//
//  3. Remove operation with filter (removes matching array elements):
//     Operation: {"op": "remove", "path": "emails[type eq \"home\"]"}
//     Before:    {"emails": [{"type": "work", "value": "w@example.com"}, {"type": "home", "value": "h@example.com"}]}
//     After:     {"emails": [{"type": "work", "value": "w@example.com"}]}
//
//  4. Add to array:
//     Operation: {"op": "add", "path": "emails", "value": [{"type": "work", "value": "new@work.com"}]}
//     Before:    {"emails": [{"type": "home", "value": "home@example.com"}]}
//     After:     {"emails": [{"type": "home", "value": "home@example.com"}, {"type": "work", "value": "new@work.com"}]}
func Apply(target, patchData []byte) ([]byte, error) {
	// Step 1: Parse the target SCIM resource (the object we're modifying)
	// Example target: {"userName":"alice","emails":[{"type":"work","value":"old@work.com"}]}
	var obj attributes
	if err := json.Unmarshal(target, &obj); err != nil {
		return nil, trace.Wrap(err)
	}

	// Step 2: Parse the SCIM PATCH request
	// Example patch: {"Operations":[{"op":"replace","path":"emails[type eq \"work\"].value","value":"new@work.com"}]}
	var patch patch
	if err := json.Unmarshal(patchData, &patch); err != nil {
		return nil, trace.Wrap(err)
	}

	// Step 3: Apply each operation in sequence to the target object
	// Operations are applied in the order they appear in the patch request.
	// Each operation modifies the object, and subsequent operations work on the modified state.
	for _, op := range patch.Operations {
		switch strings.ToLower(op.Op) {
		case opAdd:
			// Add operation: adds or appends values to attributes
			// Example: {"op":"add","path":"emails","value":[{"type":"personal","value":"p@example.com"}]}
			if err := applyOpAddOrReplace(obj, op.Path, op.Value, opAdd); err != nil {
				return nil, trace.Wrap(err)
			}
		case opReplace:
			// Replace operation: replaces existing attribute values
			// Example: {"op":"replace","path":"userName","value":"bob"}
			if err := applyOpAddOrReplace(obj, op.Path, op.Value, opReplace); err != nil {
				return nil, trace.Wrap(err)
			}
		case opRemove:
			// Remove operation: deletes attributes or array elements
			// Example: {"op":"remove","path":"emails[type eq \"home\"]"}
			if err := applyOpRemove(obj, op.Path); err != nil {
				return nil, trace.Wrap(err)
			}
		default:
			return nil, trace.BadParameter("unknown operation %q", op.Op)
		}
	}

	buff, err := json.Marshal(obj)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return buff, nil
}

// applyOpAddOrReplace applies an add or replace operation to the target object.
// If path is empty, the operation is applied to the root object.
// Otherwise, the path is parsed and the operation is applied to the specified attribute.
//
// Flow:
//  1. Empty path - apply to root (merge all attributes from value into obj)
//  2. Non-empty path - parse the path to extract:
//     attrPath: the attribute name (e.g., "emails")
//     expr: optional filter expression (e.g., "type eq \"work\"")
//     subAttr: optional sub-attribute after the filter (e.g., "value")
//
// 3. Apply the operation to the parsed path
func applyOpAddOrReplace(obj attributes, path string, value any, op string) error {
	// Validate that the operation is one we expect to handle
	if op != opAdd && op != opReplace {
		return trace.BadParameter("applyOpAddOrReplace called with unexpected operation %q", op)
	}
	// Path-less operation: merge value into the root object
	// Example: {"op":"add","value":{"displayName":"Alice Smith"}}
	if path == "" {
		return applyToRoot(obj, value)
	}

	// Parse the path into its components
	// Example: "emails[type eq \"work\"].value" -> (emails, [type eq "work"], "value")
	attrPath, expr, subAttr, err := parsePath(path)
	if err != nil {
		return trace.Wrap(err)
	}
	return applyToPath(obj, attrPath, expr, subAttr, value, op)
}

// applyToRoot applies a value to the root object by copying all attributes
// from the value into the target object. This is used for path-less operations.
// This works for both "add" and "replace" operations - in both cases, the attributes
// from value are merged into the target object, adding new attributes or updating existing ones.
func applyToRoot(obj attributes, value any) error {
	newVals, ok := value.(map[string]any)
	if !ok {
		return trace.BadParameter("root-level operation requires object value, got %T", value)
	}
	maps.Copy(obj, newVals)
	return nil
}

// applyToPath applies an add or replace operation to a specific path in the object.
// If expr is nil, it applies directly to the attribute.
// If expr is not nil, it filters array elements and applies the operation to matching elements.
//
// Flow decision:
//
//   - expr == nil: Simple attribute path (e.g., "userName" or "name.givenName")
//     Apply directly to the attribute using applyToAttribute
//
//   - expr != nil: Array filtering path (e.g., "emails[type eq \"work\"].value")
//     Iterate through array elements, evaluate filter, and update matching elements
func applyToPath(obj attributes, attrPath filter.AttributePath, expr filter.Expression, subAttr *string, value any, op string) error {
	key := attrPath.AttributeName

	// Case 1: No filter expression - simple attribute operation
	// Example: path="userName", value="alice" -> sets obj["userName"] = "alice"
	// Example: path="name.givenName", value="Alice" -> sets obj["name"]["givenName"] = "Alice"
	if expr == nil {
		if err := applyToAttribute(obj, key, subAttr, value, op); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}

	// Case 2: Filter expression present - array filtering operation
	// Example: path="emails[type eq \"work\"].value", value="new@work.com"
	// Find array elements where type="work" and update their value field

	arr, ok := obj[key].([]any)
	if !ok {
		return trace.BadParameter("filter expression requires array value at %q, got %T", key, obj[key])
	}

	// Iterate through each array element to find and update matching items.
	// Example: For path "emails[type eq \"work\"].value" with value "new@work.com":
	// arr contains all email objects: [{"type":"work","value":"old@work.com"}, {"type":"home","value":"home@example.com"}]
	// We iterate through each item, checking if it matches the filter expression
	// Only matching items will have their specified sub-attribute (or entire value) updated
	for i, item := range arr {
		// Ensure the array element is an object (map). Non-object elements are skipped.
		// Example: If item = {"type":"work","value":"old@work.com"}, elem will be this map
		elem, ok := item.(attributes)
		if !ok {
			continue
		}

		// Evaluate the filter expression against this array element.
		// Example: For filter "type eq \"work\"", this checks if elem["type"] == "work"
		// If the filter doesn't match, skip to the next array element.
		if !evaluateFilter(expr, elem) {
			continue
		}

		// At this point, we found a matching array element. Now apply the operation.
		switch op {
		case opAdd:
			// Add operation: merges or sets values in the matching element
			if subAttr != nil {
				// Set a specific sub-attribute within the matching element
				// Example: path="emails[type eq \"work\"].value", value="new@work.com"
				// Sets elem["value"] = "new@work.com" within the matching email object
				elem[*subAttr] = value
			} else if v, ok := value.(attributes); ok {
				// Merge attributes into the matching element
				// Example: path="emails[type eq \"work\"]", value={"primary":true}
				// Adds/updates elem["primary"] = true while preserving other fields
				for k, v := range v {
					elem[k] = v
				}
			} else {
				// Per SCIM RFC 7644 Section 3.5.2.1, when adding to an array with a filter
				// but no sub-attribute and value is not an object, the behavior is to replace
				// the matching element entirely (effectively the same as replace operation).
				arr[i] = value
			}
		case opReplace:
			// Replace operation: sets or replaces values in the matching element
			if subAttr != nil {
				// Replace a specific sub-attribute within the matching element
				// Example: path="emails[type eq \"work\"].value", value="new@work.com"
				// Sets elem["value"] = "new@work.com" within the matching email object
				elem[*subAttr] = value
			} else {
				// Replace the entire array element
				// Example: path="emails[type eq \"work\"]", value={"type":"work","value":"replaced@work.com"}
				// Replaces the entire email object at arr[i] with the new value
				arr[i] = value
			}
		default:
			// This should never happen as we validate operations earlier,
			// but handle it defensively to prevent silent failures.
			return trace.BadParameter("unexpected operation %q", op)
		}
	}
	return nil
}

// applyToAttribute applies an add or replace operation to a simple attribute or sub-attribute.
// If subAttr is provided, it updates a nested field within the attribute.
// For add operations on arrays, it appends values instead of replacing them.
func applyToAttribute(obj attributes, key string, subAttr *string, value any, op string) error {
	if subAttr != nil {
		child, _ := obj[key].(attributes)
		if child == nil {
			child = make(attributes)
			obj[key] = child
		}
		// sub-attributes are scalar values (e.g., name.givenName),
		child[*subAttr] = value
		return nil
	}

	// Special handling for add-to-array behavior
	if op == opAdd {
		if existing, ok := obj[key].([]any); ok {
			if incoming, ok := value.([]any); ok {
				obj[key] = append(existing, incoming...)
				return nil
			}
		}
	}

	obj[key] = value
	return nil
}

// applyOpRemove applies a remove operation to the target object.
// The path parameter specifies which attribute or array element to remove.
func applyOpRemove(obj attributes, path string) error {
	if path == "" {
		return trace.BadParameter("path is required for remove operation")
	}
	attrPath, expr, subAttr, err := parsePath(path)
	if err != nil {
		return trace.BadParameter("invalid path %q: %v", path, err)
	}
	return removeAttrPath(obj, attrPath, expr, subAttr)
}

// removeAttrPath removes an attribute or array elements from the target object.
// The behavior depends on whether a filter expression is present.
//
// Flow decision:
//
//   - expr == nil: Simple attribute removal (e.g., "userName" or "name.givenName")
//     Delete the attribute or sub-attribute directly
//
//   - expr != nil: Array filtering removal (e.g., "emails[type eq \"home\"]")
//     Filter out matching array elements
func removeAttrPath(obj attributes, attrPath filter.AttributePath, expr filter.Expression, subAttr *string) error {
	key := attrPath.AttributeName

	// Case 1: No filter expression - simple attribute removal
	if expr == nil {
		if subAttr == nil {
			// Remove entire attribute
			// Example: path="userName" -> deletes obj["userName"]
			delete(obj, key)
		} else if child, ok := obj[key].(map[string]any); ok {
			// Remove sub-attribute from nested object
			// Example: path="name.givenName" -> deletes obj["name"]["givenName"]
			delete(child, *subAttr)
		} else {
			return trace.BadParameter("cannot remove sub-attribute from non-object value at %q, got %T", key, obj[key])
		}
		return nil
	}

	// Case 2: Filter expression present - array filtering removal
	// Example: path="emails[type eq \"home\"]"
	// Remove all array elements where type="home"
	arr, ok := obj[key].([]any)
	if !ok {
		return trace.BadParameter("filter expression requires array value at %q, got %T", key, obj[key])
	}

	// Create a new array to collect elements we want to keep.
	// Using arr[:0] reuses the underlying array's capacity for efficiency.
	newArr := arr[:0]

	// Iterate through each array element to filter out or modify matching items.
	// Example: For path "emails[type eq \"home\"]":
	// Original arr: [{"type":"work","value":"work@example.com"}, {"type":"home","value":"home@example.com"}]
	// We want to remove all items where type="home"
	// Result: [{"type":"work","value":"work@example.com"}]
	for _, item := range arr {
		elem, ok := item.(attributes)

		// Keep non-object items and items that don't match the filter.
		// Example: If item is not an object, or if elem["type"] != "home", keep it.
		if !ok || !evaluateFilter(expr, elem) {
			newArr = append(newArr, item)
			continue
		}

		// At this point, we found a matching element that should be removed or modified.

		// If subAttr is specified, we only remove the sub-attribute, not the entire element.
		// Example: path="emails[type eq \"work\"].primary"
		// Removes the "primary" field from matching email objects, but keeps the email object itself
		// elem changes from {"type":"work","value":"x","primary":true} to {"type":"work","value":"x"}
		if subAttr != nil {
			delete(elem, *subAttr)
			newArr = append(newArr, elem)
		}
		// If subAttr is nil, we remove the entire matching element by not appending it to newArr.
		// Example: path="emails[type eq \"home\"]"
		// The entire email object where type="home" is excluded from newArr
	}

	obj[key] = newArr
	return nil
}

// parsePath parses an SCIM path string and extracts its components.
// Returns: (attributePath, filterExpression, subAttribute, error)
//
// SCIM paths can have three forms:
//
//  1. Simple attribute path: "userName" or "name.givenName"
//     Returns: (AttributePath{userName}, nil, nil, nil)
//     Returns: (AttributePath{name}, nil, &"givenName", nil)
//
//  2. Value path with filter: "emails[type eq \"work\"]"
//     Returns: (AttributePath{emails}, Expression{type eq "work"}, nil, nil)
//     Used to target specific array elements
//
//  3. Value path with filter and sub-attribute: "emails[type eq \"work\"].value"
//     Returns: (AttributePath{emails}, Expression{type eq "work"}, &"value", nil)
//     Used to target a specific field within matching array elements
func parsePath(raw string) (filter.AttributePath, filter.Expression, *string, error) {
	// Case 1: Simple attribute path (no filter brackets)
	// Example: "userName" or "name.givenName"
	if !strings.Contains(raw, "[") {
		ap, err := filter.ParseAttrPath([]byte(raw))
		if err != nil {
			return filter.AttributePath{}, nil, nil, trace.Wrap(err)
		}
		return ap, nil, ap.SubAttribute, nil
	}

	// Case 2 & 3: Value path with filter (contains brackets)
	// Split on "]" to check if there's a sub-attribute after the filter
	parts := strings.SplitN(raw, "]", 2)

	// Case 3: Filter with sub-attribute after it
	// Example: "emails[type eq \"work\"].value"
	// parts[0] = "emails[type eq \"work\""
	// parts[1] = ".value"
	if len(parts) == 2 && strings.Contains(parts[1], ".") {
		sub := strings.TrimPrefix(parts[1], ".")
		vp, err := filter.ParseValuePath([]byte(parts[0] + "]"))
		if err != nil {
			return filter.AttributePath{}, nil, nil, trace.Wrap(err)
		}
		return vp.AttributePath, vp.ValueFilter, &sub, nil
	}

	// Case 2: Filter without sub-attribute
	// Example: "emails[type eq \"work\"]"
	vp, err := filter.ParseValuePath([]byte(raw))
	if err != nil {
		return filter.AttributePath{}, nil, nil, trace.Wrap(err)
	}
	return vp.AttributePath, vp.ValueFilter, nil, nil
}

// evaluateFilter evaluates an SCIM filter expression against an object's attributes.
// Example filters and their evaluation:
//   - "type eq \"work\"" -> checks if props["type"] == "work"
//   - "type ne \"home\"" -> checks if props["type"] != "home"
//   - "value co \"@work\"" -> checks if props["value"] contains "@work"
//   - "type eq \"work\" and primary eq true" -> checks both conditions using AND logic
//   - "type eq \"work\" or type eq \"home\"" -> checks either condition using OR logic
//
// TODO(smallinsky) : Add support for more operators as needed and more complex types.
func evaluateFilter(expr filter.Expression, props attributes) bool {
	switch e := expr.(type) {
	// Attribute expression: compares a single attribute value against a comparison value
	// Example: "type eq \"work\"" where e.AttributePath.AttributeName="type", e.Operator=EQ, e.CompareValue="work"
	case *filter.AttributeExpression:
		attrName := e.AttributePath.AttributeName
		val, exists := props[attrName]
		if !exists {
			return false
		}

		switch e.Operator {
		case filter.EQ:
			return compareEqual(val, e.CompareValue)
		case filter.NE:
			return !compareEqual(val, e.CompareValue)
		case filter.CO:
			return contains(val, e.CompareValue)
		case filter.SW:
			return startsWith(val, e.CompareValue)
		case filter.EW:
			return endsWith(val, e.CompareValue)
		case filter.PR:
			return val != nil
		default:
			return false
		}

	// Logical expression: combines multiple filter expressions with AND/OR
	// Example: "type eq \"work\" and primary eq true"
	case *filter.LogicalExpression:
		switch e.Operator {
		case filter.AND: // Both conditions must be true
			return evaluateFilter(e.Left, props) && evaluateFilter(e.Right, props)
		case filter.OR: // At least one condition must be true
			return evaluateFilter(e.Left, props) || evaluateFilter(e.Right, props)
		default:
			return false
		}

	// Not expression: negates the result of another expression
	// Example: "not(type eq \"work\")" -> returns true if type is NOT "work"
	case *filter.NotExpression:
		return !evaluateFilter(e.Expression, props)

	default:
		return false
	}
}

func compareEqual(a, b any) bool {
	switch av := a.(type) {
	case string:
		if bv, ok := b.(string); ok {
			return av == bv
		}
	case bool:
		if bv, ok := b.(bool); ok {
			return av == bv
		}
	case float64:
		if bv, ok := b.(float64); ok {
			return av == bv
		}
		if bv, ok := b.(int); ok {
			return av == float64(bv)
		}
	case int:
		if bv, ok := b.(int); ok {
			return av == bv
		}
		if bv, ok := b.(float64); ok {
			return float64(av) == bv
		}
	}
	return fmt.Sprint(a) == fmt.Sprint(b)
}

func contains(a, b any) bool {
	sa, sb := fmt.Sprint(a), fmt.Sprint(b)
	return strings.Contains(sa, sb)
}

func startsWith(a, b any) bool {
	sa, sb := fmt.Sprint(a), fmt.Sprint(b)
	return strings.HasPrefix(sa, sb)
}

func endsWith(a, b any) bool {
	sa, sb := fmt.Sprint(a), fmt.Sprint(b)
	return strings.HasSuffix(sa, sb)
}
