/*
Copyright 2026 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gravitational/trace"
)

const (
	// maxAppRulesPerRole caps the app_resources and app_resources_expressions
	// entries a single role may define.
	maxAppRulesPerRole = 64
	// maxAppWhereBytes caps one app_resources where clause.
	maxAppWhereBytes = 1 << 10 // 1 KiB
	// maxAppExpressionBytes caps one app_resources_expressions entry.
	maxAppExpressionBytes = 4 << 10 // 4 KiB
	// maxAppPathsPerRule caps the path patterns one app_resources rule may set.
	maxAppPathsPerRule = 64
	// maxAppPathBytes caps one app_resources path pattern.
	maxAppPathBytes = 4 << 10 // 4 KiB
	// maxAppMethodsPerRule caps the HTTP methods one app_resources rule may set.
	maxAppMethodsPerRule = 16
	// maxAppReasonBytes caps an app_resources allow_reason or deny_reason_hint.
	maxAppReasonBytes = 256
	// maxAppCodeBytes caps an app_resources allow_code or deny_code_hint.
	maxAppCodeBytes = 64
)

// checkAppResources validates the app_resources and app_resources_expressions
// fields. Both require role version v9 and are allowed only under allow, so a
// pre-v9 role that sets either is rejected, as is any use under deny.
func (r *RoleV6) checkAppResources() error {
	allow := r.Spec.Allow
	deny := r.Spec.Deny
	if r.Version != V9 {
		if len(allow.AppResources) > 0 || len(allow.AppResourcesExpressions) > 0 ||
			len(deny.AppResources) > 0 || len(deny.AppResourcesExpressions) > 0 {
			return trace.BadParameter("app_resources and app_resources_expressions require role version %q, got %q", V9, r.Version)
		}
		return nil
	}
	if len(deny.AppResources) > 0 || len(deny.AppResourcesExpressions) > 0 {
		return trace.BadParameter("app_resources and app_resources_expressions are not allowed under deny")
	}
	if err := checkAppResourcesSupported(allow.AppResources, allow.AppResourcesExpressions); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(validateAppResources(allow.AppResources, allow.AppResourcesExpressions))
}

// checkAppResourcesSupported rejects the app_resources rule fields that this
// release does not yet enforce. Only the escape hatch unsafe_allow_all is
// honored; accepting the other fields would silently deny every request the
// role author meant to allow. The check is removed as enforcement for the
// remaining fields lands.
func checkAppResourcesSupported(rules []AppResource, expressions []string) error {
	if len(expressions) > 0 {
		return trace.BadParameter("app_resources_expressions is not yet supported in this release; only app_resources with unsafe_allow_all is honored")
	}
	for i, rule := range rules {
		if !rule.UnsafeAllowAll || rule.setsFieldsBesidesUnsafeAllowAll() {
			return trace.BadParameter("app_resources[%d]: only unsafe_allow_all is supported in this release; paths, methods, where, and audit codes land in a later release", i)
		}
	}
	return nil
}

// validateAppResources checks the per-role cap and the structural constraints
// of every app_resources rule and app_resources_expressions entry.
func validateAppResources(rules []AppResource, expressions []string) error {
	if total := len(rules) + len(expressions); total > maxAppRulesPerRole {
		return trace.BadParameter("a role may define at most %d app_resources rules, got %d", maxAppRulesPerRole, total)
	}
	for i, rule := range rules {
		if err := rule.check(); err != nil {
			return trace.Wrap(err, "app_resources[%d]", i)
		}
	}
	for i, expr := range expressions {
		if len(expr) > maxAppExpressionBytes {
			return trace.BadParameter("app_resources_expressions[%d] is %d bytes, over the %d byte cap", i, len(expr), maxAppExpressionBytes)
		}
	}
	return nil
}

// check validates the structural constraints of one app_resources rule. It
// leaves path parsing and predicate compilation to later evaluation stages.
func (a AppResource) check() error {
	if a.UnsafeAllowAll {
		return a.checkUnsafeAllowAllStandsAlone()
	}
	if len(a.Paths) == 0 {
		return trace.BadParameter("a rule must set paths or unsafe_allow_all")
	}
	if len(a.Paths) > maxAppPathsPerRule {
		return trace.BadParameter("a rule sets %d paths, over the %d path cap", len(a.Paths), maxAppPathsPerRule)
	}
	for i, path := range a.Paths {
		if len(path) > maxAppPathBytes {
			return trace.BadParameter("paths[%d] is %d bytes, over the %d byte cap", i, len(path), maxAppPathBytes)
		}
	}
	if len(a.Methods) > maxAppMethodsPerRule {
		return trace.BadParameter("a rule sets %d methods, over the %d method cap", len(a.Methods), maxAppMethodsPerRule)
	}
	if err := a.checkAllowEncoded(); err != nil {
		return trace.Wrap(err)
	}
	if len(a.Where) > maxAppWhereBytes {
		return trace.BadParameter("where clause is %d bytes, over the %d byte cap", len(a.Where), maxAppWhereBytes)
	}
	if a.AllowReason != "" && a.AllowCode == "" {
		return trace.BadParameter("allow_reason set without allow_code")
	}
	if err := validateAppReason("allow_reason", a.AllowReason); err != nil {
		return trace.Wrap(err)
	}
	if err := validateAppCode(a.AllowCode); err != nil {
		return trace.Wrap(err, "invalid allow_code")
	}
	if a.DenyReasonHint != "" && a.DenyCodeHint == "" {
		return trace.BadParameter("deny_reason_hint set without deny_code_hint")
	}
	if err := validateAppReason("deny_reason_hint", a.DenyReasonHint); err != nil {
		return trace.Wrap(err)
	}
	if err := validateAppCode(a.DenyCodeHint); err != nil {
		return trace.Wrap(err, "invalid deny_code_hint")
	}
	if a.DenyCodeHint != "" && a.Where == "" {
		return trace.BadParameter("deny_code_hint set without a where clause for the hint to qualify")
	}
	return nil
}

// checkAllowEncoded rejects an allow_encoded list that names anything other
// than the encoded slash "/" or repeats it. The encoded slash is the only
// character this release opts a path match into.
func (a AppResource) checkAllowEncoded() error {
	seen := false
	for _, encoded := range a.AllowEncoded {
		if encoded != "/" {
			return trace.BadParameter("allow_encoded only supports the encoded slash %q, got %q", "/", encoded)
		}
		if seen {
			return trace.BadParameter("allow_encoded lists the encoded slash %q more than once", "/")
		}
		seen = true
	}
	return nil
}

// checkUnsafeAllowAllStandsAlone rejects an unsafe_allow_all rule that also
// sets another field. unsafe_allow_all grants everything, so any companion
// field is redundant or contradictory.
func (a AppResource) checkUnsafeAllowAllStandsAlone() error {
	if a.setsFieldsBesidesUnsafeAllowAll() {
		return trace.BadParameter("unsafe_allow_all cannot be combined with any other field")
	}
	return nil
}

// IsPureUnsafeAllowAll reports whether the rule grants unrestricted access
// and sets no other field. Enforcement and the client downgrade honor the
// escape hatch only for a pure rule, so a rule that pairs unsafe_allow_all
// with a companion field written by a later release fails closed instead of
// widening access.
func (a AppResource) IsPureUnsafeAllowAll() bool {
	return a.UnsafeAllowAll && !a.setsFieldsBesidesUnsafeAllowAll()
}

// setsFieldsBesidesUnsafeAllowAll reports whether the rule sets any field
// other than unsafe_allow_all.
func (a AppResource) setsFieldsBesidesUnsafeAllowAll() bool {
	return len(a.Paths) > 0 || len(a.Methods) > 0 || a.Where != "" ||
		len(a.AllowEncoded) > 0 || a.AllowCode != "" || a.AllowReason != "" ||
		a.DenyCodeHint != "" || a.DenyReasonHint != ""
}

// validateAppCode rejects an allow or deny code that is too long, contains a
// character outside [a-z0-9_], or uses the reserved teleport_ prefix. The
// charset keeps codes safe for audit events and deny responses and makes the
// reserved-prefix check case-exact. An empty code is valid, since codes are
// optional.
func validateAppCode(code string) error {
	if len(code) > maxAppCodeBytes {
		return trace.BadParameter("code %q is %d bytes, over the %d byte cap", code, len(code), maxAppCodeBytes)
	}
	for _, r := range code {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return trace.BadParameter("code %q may only contain lowercase letters, digits, and underscores", code)
		}
	}
	if strings.HasPrefix(code, "teleport_") {
		return trace.BadParameter("code %q must not start with the reserved teleport_ prefix", code)
	}
	return nil
}

// validateAppReason rejects an allow_reason or deny_reason_hint that is too
// long, is not valid UTF-8, or contains a control, format, or line or
// paragraph separator character. Those would otherwise flow unescaped into
// audit events and deny responses, where a bidi override could spoof text or
// a line separator could break a JSON consumer. An empty reason is valid.
func validateAppReason(name, reason string) error {
	if len(reason) > maxAppReasonBytes {
		return trace.BadParameter("%s is %d bytes, over the %d byte cap", name, len(reason), maxAppReasonBytes)
	}
	if !utf8.ValidString(reason) {
		return trace.BadParameter("%s must be valid UTF-8", name)
	}
	if strings.ContainsFunc(reason, func(r rune) bool {
		return unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp)
	}) {
		return trace.BadParameter("%s must not contain control, format, or separator characters", name)
	}
	return nil
}
