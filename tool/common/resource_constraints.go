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
	"io"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// inlineConstraintRe matches the start of an inline constraint suffix: a "?"
// immediately followed by a "<key>=" token.
var inlineConstraintRe = regexp.MustCompile(`\?[A-Za-z_][A-Za-z0-9_]*=`)

// ParseResourceValues parses --resource flag values into ResourceAccessIDs.
// Each value takes one of two forms:
//
//  1. a plain slash-delimited ResourceID (unconstrained):
//     /cluster/node/web-1
//  2. inline query-form constraints appended after the ResourceID:
//     /cluster/node/web-1?logins=root,admin
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
		return types.ResourceAccessID{}, trace.BadParameter("--resource does not accept JSON; pass a JSON ResourceAccessIDList via --resource-file")
	}
	return parseInlineResource(value)
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
	if err := validateConstraintKind(raid); err != nil {
		return types.ResourceAccessID{}, trace.Wrap(err)
	}
	return raid, nil
}

// splitInlineConstraints splits a resource value into its ResourceID string
// and an optional constraint suffix. Resource names may themselves contain
// "?". The split therefore anchors on the first "?<key>=" rather than the
// first "?". A name that contains that whole shape needs --resource-file.
func splitInlineConstraints(value string) (idStr, suffix string) {
	loc := inlineConstraintRe.FindStringIndex(value)
	if loc == nil {
		return value, ""
	}
	return value[:loc[0]], value[loc[0]+1:]
}

// buildConstraintsFromSuffix parses an inline constraint suffix
// ("key=v1,v2&key2=...") into a ResourceConstraints. Unknown keys are
// rejected by name, and the suffix must name a single constraint type.
func buildConstraintsFromSuffix(suffix string) (*types.ResourceConstraints, error) {
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
	// Every key is known and there is more than one. Known keys map to
	// mutually exclusive proto variants, which cannot be combined.
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

// validateWildcardValues rejects a "*" value. Neither implemented dimension
// reads "*" as a wildcard. MatcherFromConstraints would match it as a literal
// principal name, which is never what someone typing "*" intends.
func validateWildcardValues(key string, vals []string) error {
	if slices.Contains(vals, types.Wildcard) {
		return trace.BadParameter("constraint key %q does not accept a wildcard value", key)
	}
	return nil
}

// validateConstraintWildcards applies the inline form's wildcard rules to an
// already built ResourceConstraints (the --resource-file path).
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

// plannedConstraintKeys name the dimensions of constraint kinds that are
// designed but absent from this build. They get a distinct "not yet
// supported" error rather than "unknown constraint key".
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

// InlineEncodableConstraintKey reports whether the inline grammar understands
// the given constraint key. A newer cluster may report dimensions this build
// has no key for, and output meant to be passed back to --resource must leave
// those out rather than emit a value the parser will reject.
func InlineEncodableConstraintKey(key string) bool {
	switch key {
	case "logins", "role_arns":
		return true
	}
	return false
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
// commas. "\,", "\&", and "\\" escape their literal character; any other
// escape, or a trailing backslash, is rejected.
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
