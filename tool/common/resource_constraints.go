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
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// inlineConstraintRe matches the start of an inline constraint suffix: a "?"
// immediately followed by a "<key>=" token. Anchoring on the shape rather than
// a fixed set of key names lets us split constraints off a resource ID without
// knowing every key ahead of time, so a resource string written by a newer
// client (with keys this build has never seen) still splits cleanly instead of
// being mistaken for part of the resource name.
var inlineConstraintRe = regexp.MustCompile(`\?[A-Za-z_][A-Za-z0-9_]*=`)

// ParseResourceValues parses --resource flag values into ResourceAccessIDs.
// Each value takes one of two forms:
//
//  1. a plain slash-delimited ResourceID (unconstrained, unchanged behavior):
//     /cluster/node/web-1
//  2. inline query-form constraints appended after the ResourceID:
//     /cluster/node/web-1?logins=root,admin
//
// JSON ResourceAccessIDs are not accepted here; they take the dedicated
// --resource-json flag (ParseResourceJSONValues).
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

// ParseResourceJSONValues parses --resource-json flag values, each a single
// JSON ResourceAccessID: the canonical form for automation and the fallback
// for values the inline form cannot express.
func ParseResourceJSONValues(values []string) ([]types.ResourceAccessID, error) {
	out := make([]types.ResourceAccessID, 0, len(values))
	for _, v := range values {
		raid, err := parseJSONResource(v)
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
		if err := validateConstraintWildcards(raid.GetConstraints()); err != nil {
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
		return types.ResourceAccessID{}, trace.BadParameter("--resource does not accept JSON; pass a JSON ResourceAccessID via --resource-json")
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
	if err := validateConstraintWildcards(raid.GetConstraints()); err != nil {
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
// an optional constraint suffix ("key=v1,v2&key2=..." without the leading "?").
// Resource names may themselves contain "?", so we anchor on the first
// "?<key>=" (see inlineConstraintRe) rather than the first "?". A resource
// name that itself contains "?<ident>=" is ambiguous; the JSON resource form
// is unambiguous for those.
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
	// Pair boundaries are anchored the same way as the resource split: an
	// unescaped "&" immediately followed by "<key>=". Values may contain "&"
	// (an IAM role name allows only "+=,.@-" and word characters, but an ARN's
	// path admits any printable ASCII); a value carrying the literal "&<key>="
	// shape escapes the ampersand as "\&" (see splitConstraintValues). Within a
	// pair, only the first "=" splits key from values, so a literal "=" inside
	// a value needs no escaping. Slashes are never special inside the suffix.
	merged := make(map[string][]string)
	for _, pair := range splitConstraintPairs(suffix) {
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
		if err := validateWildcardValues(key, vals); err != nil {
			return nil, trace.Wrap(err)
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

// pairKeyRe matches the "<key>=" token that must immediately follow a pair
// boundary.
var pairKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// splitConstraintPairs splits a constraint suffix on each unescaped "&" that
// is immediately followed by "<key>=". An "&" without that shape, or escaped
// as "\&", stays part of the current value.
func splitConstraintPairs(suffix string) []string {
	var pairs []string
	start := 0
	for i := 0; i < len(suffix); i++ {
		switch {
		case suffix[i] == '\\':
			i++ // skip the escaped character
		case suffix[i] == '&' && pairKeyRe.MatchString(suffix[i+1:]):
			pairs = append(pairs, suffix[start:i])
			start = i + 1
		}
	}
	return append(pairs, suffix[start:])
}

// wildcardCapableKeys are inline keys whose dimension accepts a "*" value per
// RFD 228 (db_users and db_names, and later azure_identities and
// gcp_service_accounts, once their constraint variants are implemented). For
// a key in this set, "*" must be the key's only value. No currently
// implemented key is wildcard-capable, so the set is empty.
var wildcardCapableKeys = map[string]bool{}

// validateWildcardValues enforces RFD 228's wildcard rules for one key's
// merged values: "*" is rejected outright for keys outside
// wildcardCapableKeys, and for keys in the set it must stand alone.
func validateWildcardValues(key string, vals []string) error {
	hasWildcard := slices.Contains(vals, types.Wildcard)
	if !hasWildcard {
		return nil
	}
	if !wildcardCapableKeys[key] {
		return trace.BadParameter("constraint key %q does not accept a wildcard value", key)
	}
	if len(vals) > 1 {
		return trace.BadParameter("a wildcard value for constraint key %q must be its only value", key)
	}
	return nil
}

// validateConstraintWildcards applies the same wildcard rules the inline form
// enforces to an already built ResourceConstraints, so the JSON --resource and
// --resource-file paths cannot smuggle in a "*" the inline form would reject.
func validateConstraintWildcards(rc *types.ResourceConstraints) error {
	if rc == nil {
		return nil
	}
	switch d := rc.GetDetails().(type) {
	case *types.ResourceConstraints_Ssh:
		if d.Ssh != nil {
			return trace.Wrap(validateWildcardValues("logins", d.Ssh.Logins))
		}
	case *types.ResourceConstraints_AwsConsole:
		if d.AwsConsole != nil {
			return trace.Wrap(validateWildcardValues("role_arns", d.AwsConsole.RoleArns))
		}
	}
	return nil
}

// plannedConstraintKeys are inline keys reserved by RFD 228 for resource kinds
// whose ResourceConstraints variants are not implemented yet. They get a
// distinct error so scripts and agents can tell "not supported yet" apart from
// a typo.
var plannedConstraintKeys = map[string]bool{
	"db_users":             true,
	"db_names":             true,
	"db_roles":             true,
	"kube_users":           true,
	"kube_groups":          true,
	"desktop_logins":       true,
	"azure_identities":     true,
	"gcp_service_accounts": true,
}

// EscapeConstraintValue escapes the characters the inline constraint grammar
// treats specially ("\", "," and "&") so the value round-trips through
// splitConstraintValues.
func EscapeConstraintValue(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, ",", `\,`)
	v = strings.ReplaceAll(v, "&", `\&`)
	return v
}

// splitConstraintValues splits a constraint's raw value list on unescaped
// commas. AWS role names may contain literal commas (IAM allows "+=,.@-"), so
// "\," escapes a comma within a value, "\&" an ampersand (letting a value
// carry the literal "&<key>=" shape without it reading as a pair boundary),
// and "\\" a literal backslash; any other escape, or a trailing backslash, is
// rejected rather than passed through.
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
			if r != ',' && r != '\\' && r != '&' {
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
