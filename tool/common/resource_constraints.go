/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"encoding/json"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// inlineConstraintRe matches the start of an inline constraint suffix: a "|"
// immediately followed by a "<key>=" token. Anchoring on the shape rather than
// a fixed set of key names lets us split constraints off a resource ID without
// knowing every key ahead of time, so a resource string written by a newer
// client (with keys this build has never seen) still splits cleanly instead of
// being mistaken for part of the resource name.
var inlineConstraintRe = regexp.MustCompile(`\|[A-Za-z_][A-Za-z0-9_]*=`)

// ParseResourceValues parses --resource flag values into ResourceAccessIDs.
// Each value takes one of three forms (selected by its shape):
//
//  1. a plain slash-delimited ResourceID (unconstrained, unchanged behavior):
//     /cluster/node/web-1
//  2. inline anchored-key constraints appended after the ResourceID:
//     /cluster/node/web-1|logins=root,admin
//  3. a single JSON ResourceAccessID (canonical form for automation):
//     {"id":{"cluster":"main","kind":"node","name":"web-1"},"constraints":{"version":"v1","ssh":{"logins":["root"]}}}
func ParseResourceValues(values []string) ([]types.ResourceAccessID, error) {
	out := make([]types.ResourceAccessID, 0, len(values))
	for _, v := range values {
		raid, err := parseResourceValue(v)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, raid)
	}
	return out, nil
}

// ParseResourceAccessIDListJSON parses the contents of --resource-file (or
// stdin): a JSON ResourceAccessIDList, the same shape serialized into the
// request and cert.
func ParseResourceAccessIDListJSON(data []byte) ([]types.ResourceAccessID, error) {
	raids, err := types.ResourceAccessIDsFromString(string(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, raid := range raids {
		if err := validateConstraintKind(raid); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return raids, nil
}

// ParseResourceAccessIDListFile reads a --resource-file input (a path, or "-"
// to read stdin from the given reader) and parses it with
// ParseResourceAccessIDListJSON.
func ParseResourceAccessIDListFile(path string, stdin io.Reader) ([]types.ResourceAccessID, error) {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	raids, err := ParseResourceAccessIDListJSON(data)
	return raids, trace.Wrap(err)
}

func parseResourceValue(value string) (types.ResourceAccessID, error) {
	if strings.HasPrefix(strings.TrimSpace(value), "{") {
		return parseJSONResource(value)
	}
	return parseInlineResource(value)
}

func parseJSONResource(value string) (types.ResourceAccessID, error) {
	var raid types.ResourceAccessID
	// ResourceConstraints carries a proto oneof and defines its own
	// MarshalJSON/UnmarshalJSON, so a stdlib Unmarshal into ResourceAccessID
	// round-trips the nested constraints correctly.
	if err := json.Unmarshal([]byte(value), &raid); err != nil {
		return types.ResourceAccessID{}, trace.BadParameter("invalid JSON resource %q: %v", value, err)
	}
	if err := raid.Id.CheckAndSetDefaults(); err != nil {
		return types.ResourceAccessID{}, trace.Wrap(err)
	}
	if rc := raid.GetConstraints(); rc != nil {
		if err := rc.CheckAndSetDefaults(); err != nil {
			return types.ResourceAccessID{}, trace.Wrap(err)
		}
	}
	if err := validateConstraintKind(raid); err != nil {
		return types.ResourceAccessID{}, trace.Wrap(err)
	}
	return raid, nil
}

func parseInlineResource(value string) (types.ResourceAccessID, error) {
	idStr, suffix := splitInlineConstraints(value)
	id, err := types.ResourceIDFromString(idStr)
	if err != nil {
		return types.ResourceAccessID{}, trace.Wrap(err)
	}
	if suffix == "" {
		return types.ResourceAccessID{Id: id}, nil
	}
	rc, err := buildConstraintsFromSuffix(suffix)
	if err != nil {
		return types.ResourceAccessID{}, trace.Wrap(err)
	}
	raid := types.ResourceAccessID{Id: id, Constraints: rc}
	// The suffix is parsed without reference to the resource kind, so confirm
	// the resulting constraint type actually applies to this resource.
	if err := validateConstraintKind(raid); err != nil {
		return types.ResourceAccessID{}, trace.Wrap(err)
	}
	return raid, nil
}

// splitInlineConstraints splits a resource value into its ResourceID string and
// an optional constraint suffix ("key=v1,v2|key2=..." without the leading "|").
// Resource names may themselves contain "|", so we anchor on the first
// "|<key>=" (see inlineConstraintRe) rather than the first "|". A resource name
// that itself contains "|<ident>=" is ambiguous; the JSON resource form is
// unambiguous for those.
func splitInlineConstraints(value string) (idStr, suffix string) {
	loc := inlineConstraintRe.FindStringIndex(value)
	if loc == nil {
		return value, ""
	}
	return value[:loc[0]], value[loc[0]+1:]
}

// buildConstraintsFromSuffix parses an inline constraint suffix into a
// ResourceConstraints. It only understands the keys this build can encode into
// the proto (logins, role_arns); any other key is rejected by name rather than
// matched against an enumerated list, so an unimplemented or future key gives a
// clear error instead of silently changing behavior. The suffix must name a
// single constraint type.
func buildConstraintsFromSuffix(suffix string) (*types.ResourceConstraints, error) {
	// Values never contain "|" (AWS role names allow "+=,.@-" but not "|"; see
	// the IAM CreateRole naming rules), so splitting the suffix on "|" and each
	// pair on the first "=" is unambiguous. A literal "=" inside a value is fine
	// since only the first "=" splits the pair; literal commas are escaped as
	// "\," and handled by splitConstraintValues.
	merged := make(map[string][]string)
	for _, pair := range strings.Split(suffix, "|") {
		key, rawVals, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, trace.BadParameter("invalid constraint %q, expected key=value", pair)
		}
		vals, err := splitConstraintValues(key, rawVals)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		merged[key] = append(merged[key], vals...)
	}

	rc := &types.ResourceConstraints{Version: types.ResourceConstraintVersionV1}
	for key, vals := range merged {
		switch key {
		case "logins":
			rc.Details = &types.ResourceConstraints_Ssh{Ssh: &types.SSHResourceConstraints{Logins: vals}}
		case "role_arns":
			rc.Details = &types.ResourceConstraints_AwsConsole{AwsConsole: &types.AWSConsoleResourceConstraints{RoleArns: vals}}
		default:
			if plannedConstraintKeys[key] {
				return nil, trace.BadParameter("constraint key %q is not yet supported", key)
			}
			return nil, trace.BadParameter("unknown constraint key %q", key)
		}
	}
	// Every key is known but there is more than one, so they map to different
	// (mutually exclusive) proto variants.
	if len(merged) > 1 {
		return nil, trace.BadParameter("a resource cannot combine multiple constraint types")
	}
	if err := rc.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return rc, nil
}

// plannedConstraintKeys are inline keys reserved by RFD 228 for resource kinds
// whose ResourceConstraints variants have not landed yet. They get a distinct
// error so scripts and agents can tell "not supported yet" apart from a typo.
var plannedConstraintKeys = map[string]bool{
	"db_users":             true,
	"db_names":             true,
	"db_roles":             true,
	"kube_users":           true,
	"kube_groups":          true,
	"azure_identities":     true,
	"gcp_service_accounts": true,
}

// splitConstraintValues splits a constraint's raw value list on unescaped
// commas. AWS role names may contain literal commas (IAM allows "+=,.@-"), so
// "\," escapes a comma within a value and "\\" a literal backslash; any other
// escape, or a trailing backslash, is rejected rather than passed through.
func splitConstraintValues(key, raw string) ([]string, error) {
	var vals []string
	var b strings.Builder
	flush := func() error {
		v := strings.TrimSpace(b.String())
		if v == "" {
			return trace.BadParameter("constraint %q contains an empty value", key)
		}
		vals = append(vals, v)
		b.Reset()
		return nil
	}
	escaped := false
	for _, r := range raw {
		switch {
		case escaped:
			if r != ',' && r != '\\' {
				return nil, trace.BadParameter(`constraint %q contains unsupported escape sequence "\%c"`, key, r)
			}
			b.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == ',':
			if err := flush(); err != nil {
				return nil, trace.Wrap(err)
			}
		default:
			b.WriteRune(r)
		}
	}
	if escaped {
		return nil, trace.BadParameter("constraint %q ends with a dangling escape character", key)
	}
	if err := flush(); err != nil {
		return nil, trace.Wrap(err)
	}
	return vals, nil
}

// validateConstraintKind ensures a ResourceAccessID's constraint variant matches
// its ResourceID kind.
func validateConstraintKind(raid types.ResourceAccessID) error {
	rc := raid.GetConstraints()
	if rc == nil {
		return nil
	}
	var wantKind string
	switch rc.GetDetails().(type) {
	case *types.ResourceConstraints_Ssh:
		wantKind = types.KindNode
	case *types.ResourceConstraints_AwsConsole:
		wantKind = types.KindApp
	default:
		return trace.BadParameter("unsupported constraint type on resource %q", types.ResourceIDToString(raid.Id))
	}
	if raid.Id.Kind != wantKind {
		return trace.BadParameter("constraint does not apply to resources of kind %q", raid.Id.Kind)
	}
	return nil
}
