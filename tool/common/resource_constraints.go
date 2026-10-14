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
	"bytes"
	"encoding/json"
	"io"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/utils"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// Flag descriptions shared by the tsh and tctl request create commands.
const (
	ResourceFlagDescription     = `Resource ID to request, with optional inline constraints ("/main/node/<node-uuid>?logins=root"). Repeatable. Escape literal commas in constraint values as \,. Use --resource-file for JSON input.`
	ResourceFileFlagDescription = `Path to a JSON ResourceAccessIDList file. Use --resource-file=- to read stdin.`
)

// One variant per ResourceConstraints oneof member. Keys of the same variant
// can be combined on one resource.
const (
	variantSSH        = "ssh"
	variantAWSConsole = "aws_console"
)

// constraintKeySpec is the resource kinds an inline constraint key applies to
// and the ResourceConstraints oneof member it sets.
type constraintKeySpec struct {
	kinds   []string
	variant string
}

var constraintKeySpecs = map[string]constraintKeySpec{
	types.PrincipalTypeLogins:   {kinds: []string{types.KindNode}, variant: variantSSH},
	types.PrincipalTypeRoleARNs: {kinds: []string{types.KindApp}, variant: variantAWSConsole},
}

// Sorted for error messages.
var supportedConstraintKeys = slices.Sorted(maps.Keys(constraintKeySpecs))

type constraintDimension struct {
	key    string
	values []string
}

// constraintDimensions returns each dimension of rc; nil for an unknown variant.
func constraintDimensions(rc *types.ResourceConstraints) []constraintDimension {
	var dims []constraintDimension
	switch d := rc.GetDetails().(type) {
	case *types.ResourceConstraints_Ssh:
		if d.Ssh != nil {
			dims = append(dims, constraintDimension{key: types.PrincipalTypeLogins, values: d.Ssh.Logins})
		}
	case *types.ResourceConstraints_AwsConsole:
		if d.AwsConsole != nil {
			dims = append(dims, constraintDimension{key: types.PrincipalTypeRoleARNs, values: d.AwsConsole.RoleArns})
		}
	}
	// Keep dimensions of an all-empty variant. If omitted it appears as unconstrained.
	filled := func(d constraintDimension) bool { return len(d.values) > 0 }
	if slices.ContainsFunc(dims, filled) {
		dims = slices.DeleteFunc(dims, func(d constraintDimension) bool { return !filled(d) })
	}
	return dims
}

// CollectRequestedResources parses the inline values and the JSON
// ResourceAccessIDList at listPath ("-" for stdin, "" for none), and
// deduplicates the result.
func CollectRequestedResources(values []string, listPath string, stdin io.Reader) ([]types.ResourceAccessID, error) {
	raids, err := ParseResourceValues(values)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if listPath != "" {
		list := stdin
		if listPath != "-" {
			path, err := utils.NormalizePath(listPath, false)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			f, err := os.Open(path)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			defer f.Close()
			list = f
		}
		fromList, err := ParseResourceAccessIDList(list)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		raids = append(raids, fromList...)
	}
	deduplicated, err := deduplicateResourceAccessIDs(raids)
	return deduplicated, trace.Wrap(err)
}

// ParseResourceValues parses inline resource values, each a ResourceID with
// an optional constraint suffix ("/cluster/node/<node-uuid>?logins=root,admin").
func ParseResourceValues(values []string) ([]types.ResourceAccessID, error) {
	out := make([]types.ResourceAccessID, 0, len(values))
	for _, v := range values {
		if strings.HasPrefix(strings.TrimSpace(v), "{") {
			return nil, trace.WithField(trace.BadParameter("resource %q is JSON. Provide a resource ID with optional inline constraints", v), "resource_input_format", "json")
		}
		raid, err := parseInlineResource(v)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, raid)
	}
	return out, nil
}

// ParseResourceAccessIDList reads a JSON ResourceAccessIDList from r.
func ParseResourceAccessIDList(r io.Reader) ([]types.ResourceAccessID, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var list types.ResourceAccessIDList
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&list); err != nil {
		return nil, trace.BadParameter("invalid ResourceAccessIDList JSON (%v). Provide a valid JSON resource list", err)
	}
	if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
		return nil, trace.BadParameter("resource list contains trailing data. Provide a single JSON ResourceAccessIDList")
	}
	for i := range list.Resources {
		if err := validateResourceAccessID(&list.Resources[i]); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return list.Resources, nil
}

// MinConstraintsVersion is the first Teleport release that stores resource
// constraints on an Access Request.
const MinConstraintsVersion = "18.7.6"

// CheckConstraintsSupported rejects constrained requests unless Auth reports
// a supported version.
func CheckConstraintsSupported(req types.AccessRequest, clusterVersion func() (string, error)) error {
	if len(req.GetRequestedResourceAccessIDs()) == 0 {
		return nil
	}
	version, err := clusterVersion()
	if err != nil {
		return trace.Wrap(err)
	}
	if supported, err := utils.MinVerWithoutPreRelease(version, MinConstraintsVersion); err == nil && supported {
		return nil
	}
	return trace.NotImplemented("cannot verify constraint support with Auth version %q. Use Auth %s or later, or request without constraints",
		version, MinConstraintsVersion)
}

// validateResourceAccessID rejects unenforceable or invalid constraints.
// Such constraints are tolerated at Identity decoding but should be rejected
// at request creation.
func validateResourceAccessID(raid *types.ResourceAccessID) error {
	if err := raid.Id.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	rc := raid.GetConstraints()
	if rc == nil {
		return nil
	}
	idStr := types.ResourceIDToString(raid.Id)
	if rc.Unenforceable() {
		return trace.BadParameter("unrecognized constraints on resource %q. Request the resource without constraints", idStr)
	}
	if err := rc.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "invalid constraints on resource %q. Correct the constraint values", idStr)
	}
	if err := validateConstraintKind(*raid); err != nil {
		return trace.Wrap(err)
	}
	for _, d := range constraintDimensions(rc) {
		if err := validateWildcardValues(d.key, d.values); err != nil {
			return trace.Wrap(err, "resource %q", idStr)
		}
		if d.key == types.PrincipalTypeRoleARNs {
			for _, value := range d.values {
				if _, err := awsutils.ParseRoleARN(value); err != nil {
					return trace.BadParameter("invalid AWS role ARN %q on resource %q. Specify a full IAM role ARN", value, idStr)
				}
			}
		}
	}
	switch d := rc.GetDetails().(type) {
	case *types.ResourceConstraints_Ssh:
		d.Ssh.Logins = apiutils.Deduplicate(d.Ssh.Logins)
	case *types.ResourceConstraints_AwsConsole:
		d.AwsConsole.RoleArns = apiutils.Deduplicate(d.AwsConsole.RoleArns)
	}
	return nil
}

// deduplicateResourceAccessIDs removes identical duplicates and rejects conflicts.
// Auth keeps only the first entry per resource.
func deduplicateResourceAccessIDs(raids []types.ResourceAccessID) ([]types.ResourceAccessID, error) {
	out := make([]types.ResourceAccessID, 0, len(raids))
	seen := make(map[string]types.ResourceAccessID, len(raids))
	for _, raid := range raids {
		idStr := types.ResourceIDToString(raid.GetResourceID())
		prev, ok := seen[idStr]
		if !ok {
			seen[idStr] = raid
			out = append(out, raid)
			continue
		}
		if !constraintsEqual(prev.GetConstraints(), raid.GetConstraints()) {
			return nil, trace.BadParameter("resource %q has conflicting constraints (%q and %q). Request the resource once",
				idStr, FormatResourceAccessIDInline(prev), FormatResourceAccessIDInline(raid))
		}
	}
	return out, nil
}

func constraintsEqual(a, b *types.ResourceConstraints) bool {
	return slices.EqualFunc(constraintDimensions(a), constraintDimensions(b), func(a, b constraintDimension) bool {
		return a.key == b.key && apiutils.ContainSameUniqueElements(a.values, b.values)
	})
}

// inlineConstraintRe matches the "?key=" that starts a constraint suffix.
var inlineConstraintRe = regexp.MustCompile(`\?[A-Za-z_][A-Za-z0-9_]*=`)

func parseInlineResource(value string) (types.ResourceAccessID, error) {
	idStr, suffix := value, ""
	if loc := inlineConstraintRe.FindStringIndex(value); loc != nil {
		idStr, suffix = value[:loc[0]], value[loc[0]+1:]
	}
	id, err := types.ResourceIDFromString(idStr)
	if err != nil {
		return types.ResourceAccessID{}, trace.Wrap(err)
	}
	raid := types.ResourceAccessID{Id: id}
	if suffix != "" {
		rc, err := parseConstraintSuffix(suffix)
		if err != nil {
			return types.ResourceAccessID{}, trace.Wrap(err, "resource %q", value)
		}
		raid.Constraints = rc
	}
	return raid, trace.Wrap(validateResourceAccessID(&raid))
}

// parseConstraintSuffix parses "key=v1,v2&key2=v3". All keys must belong to
// one variant. The result is validated by validateResourceAccessID.
func parseConstraintSuffix(suffix string) (*types.ResourceConstraints, error) {
	values := make(map[string][]string)
	var variant, variantKey string
	for _, pair := range splitConstraintPairs(suffix) {
		key, raw, _ := strings.Cut(pair, "=")
		spec, ok := constraintKeySpecs[key]
		if !ok {
			return nil, trace.BadParameter("unrecognized constraint key %q. Use one of %s", key, strings.Join(supportedConstraintKeys, ", "))
		}
		if variant != "" && spec.variant != variant {
			return nil, trace.BadParameter("constraint keys %q and %q cannot be combined on one resource. Request each resource with matching constraints", variantKey, key)
		}
		variant, variantKey = spec.variant, key
		vals, err := splitConstraintValues(key, raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		values[key] = append(values[key], vals...)
	}

	rc := &types.ResourceConstraints{Version: types.ResourceConstraintVersionV1}
	switch variant {
	case variantSSH:
		rc.Details = &types.ResourceConstraints_Ssh{Ssh: &types.SSHResourceConstraints{Logins: values[types.PrincipalTypeLogins]}}
	case variantAWSConsole:
		rc.Details = &types.ResourceConstraints_AwsConsole{AwsConsole: &types.AWSConsoleResourceConstraints{RoleArns: values[types.PrincipalTypeRoleARNs]}}
	}
	return rc, nil
}

var pairKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// splitConstraintPairs splits on unescaped "&" followed by "key=". Any other
// "&" is part of the value.
func splitConstraintPairs(suffix string) []string {
	var pairs []string
	start := 0
	for i := 0; i < len(suffix); i++ {
		switch {
		case suffix[i] == '\\':
			i++
		case suffix[i] == '&' && pairKeyRe.MatchString(suffix[i+1:]):
			pairs = append(pairs, suffix[start:i])
			start = i + 1
		}
	}
	return append(pairs, suffix[start:])
}

// validateWildcardValues rejects "*", a value reserved for a later release to
// define as a wildcard.
func validateWildcardValues(key string, vals []string) error {
	if slices.Contains(vals, types.Wildcard) {
		return trace.BadParameter("constraint %q does not accept wildcard values. Specify explicit values", key)
	}
	return nil
}

// EscapeConstraintValue escapes "\", "," and "&" for the inline constraint grammar.
func EscapeConstraintValue(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, ",", `\,`)
	v = strings.ReplaceAll(v, "&", `\&`)
	return v
}

// splitConstraintValues splits raw on unescaped commas. Only "\,", "\&" and
// "\\" are valid escapes.
func splitConstraintValues(key, raw string) ([]string, error) {
	var vals []string
	var b strings.Builder
	flush := func() error {
		v := strings.TrimSpace(b.String())
		if v == "" {
			return trace.BadParameter("constraint %q contains an empty value. Specify a nonempty value", key)
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
				return nil, trace.BadParameter(`constraint %q contains unsupported escape sequence "\%c". Escape only commas, ampersands, and backslashes`, key, r)
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
		return nil, trace.BadParameter("constraint %q ends with an incomplete escape sequence. Escape the backslash or remove it", key)
	}
	if err := flush(); err != nil {
		return nil, trace.Wrap(err)
	}
	return vals, nil
}

func validateConstraintKind(raid types.ResourceAccessID) error {
	for _, d := range constraintDimensions(raid.GetConstraints()) {
		kinds := constraintKeySpecs[d.key].kinds
		if !slices.Contains(kinds, raid.Id.Kind) {
			return trace.BadParameter("constraint %q is not supported for resource %q of kind %q. Use a resource of kind %q",
				d.key, types.ResourceIDToString(raid.Id), raid.Id.Kind, strings.Join(kinds, ", "))
		}
	}
	return nil
}
