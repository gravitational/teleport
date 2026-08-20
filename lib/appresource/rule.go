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

package appresource

import (
	"slices"
	"strings"

	"github.com/gravitational/trace"
)

// maxWhereBytes is the maximum length in bytes of one where clause, the sugared
// form.
const maxWhereBytes = 1 << 10 // 1 KiB

// maxReasonBytes is the maximum length in bytes of an allow_reason or
// deny_reason_hint.
const maxReasonBytes = 1 << 10 // 1 KiB

// maxAuditCodeBytes is the maximum length in bytes of an allow_code or
// deny_code_hint.
const maxAuditCodeBytes = 256

// maxPathBytes is the maximum length in bytes of one path pattern.
const maxPathBytes = 1 << 10 // 1 KiB

// maxPaths is the maximum number of path patterns in one rule.
const maxPaths = 64

// Rule is one app_resources entry, the sugared form. A request matches when its
// path matches Paths, its method matches Methods, and its Where clause evaluates to
// true.
type Rule struct {
	// Paths are the path patterns the rule matches. The {project} segment in
	// "/api/projects/{project}/**" is captured, and Where reads it as
	// vars.project. A rule sets either Paths or AllowAll.
	Paths []string `yaml:"paths,omitempty"`
	// Methods is a list of GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS, or
	// TRACE, matched case-insensitively. A request method is not folded, so
	// it must be upper case. Unset, Methods allows all eight.
	Methods []string `yaml:"methods,omitempty"`
	// Where is a predicate over the caller identity and the rule's path
	// captures, such as contains(user.traits["projects"], vars.project). If
	// set, it must evaluate to true for the rule to match.
	Where string `yaml:"where,omitempty"`
	// AllowEncoded lists the characters a request path may carry in
	// percent-encoded form for the rule to match. The only supported value is
	// "/", which allows the encoded slash, %2F or %2f.
	AllowEncoded []string `yaml:"allow_encoded,omitempty"`
	// AllowCode is the code recorded on the allow audit event when the rule
	// matches. If it is not set, no allow audit event is recorded. A code may
	// not start with the reserved "teleport_" prefix.
	AllowCode string `yaml:"allow_code,omitempty"`
	// AllowReason is the explanation recorded alongside AllowCode. A rule sets
	// it only together with AllowCode.
	AllowReason string `yaml:"allow_reason,omitempty"`
	// DenyCodeHint is the code added to the deny decision when the rule's path
	// and method match but the Where predicate does not. A denied request
	// collects a code from every such rule, so one decision can record several
	// codes. A code may not start with the reserved "teleport_" prefix.
	DenyCodeHint string `yaml:"deny_code_hint,omitempty"`
	// DenyReasonHint is the explanation recorded alongside DenyCodeHint. A rule
	// sets it only together with DenyCodeHint.
	DenyReasonHint string `yaml:"deny_reason_hint,omitempty"`
	// AllowAll grants unrestricted access to every path and method. It cannot
	// be combined with any other field.
	AllowAll bool `yaml:"allow_all,omitempty"`
}

// validateAuditCode checks an allow or deny code. A valid code is 1 to 256 bytes of
// [a-z0-9_] and does not start with the reserved teleport_ prefix.
func validateAuditCode(code string) error {
	if len(code) < 1 || len(code) > maxAuditCodeBytes {
		return trace.BadParameter("code %q must be 1 to %d bytes", code, maxAuditCodeBytes)
	}
	for _, r := range code {
		legal := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_'
		if !legal {
			return trace.BadParameter("code %q must contain only [a-z0-9_]", code)
		}
	}
	if strings.HasPrefix(code, "teleport_") {
		return trace.BadParameter("code %q must not start with the reserved teleport_ prefix", code)
	}
	return nil
}

// validate checks a rule's structural constraints, e.g. that AllowAll cannot be
// combined with another field. Path pattern checks are left to compile time.
func (r Rule) validate() error {
	if r.AllowAll {
		return r.validateAllowAllStandsAlone()
	}
	if len(r.Paths) == 0 {
		return trace.BadParameter("a rule must set paths or allow_all")
	}
	if err := validatePaths(r.Paths); err != nil {
		return trace.Wrap(err)
	}
	if err := validateMethods(r.Methods); err != nil {
		return trace.Wrap(err)
	}
	if err := validateWhere(r.Where); err != nil {
		return trace.Wrap(err)
	}
	for _, e := range r.AllowEncoded {
		if e != "/" {
			return trace.BadParameter("allow_encoded allows only the separator %q, got %q", "/", e)
		}
	}
	if r.AllowReason != "" && r.AllowCode == "" {
		return trace.BadParameter("allow_reason set without allow_code")
	}
	if r.AllowCode != "" {
		if err := validateAuditCode(r.AllowCode); err != nil {
			return trace.Wrap(err, "invalid allow_code")
		}
	}
	if len(r.AllowReason) > maxReasonBytes {
		return trace.BadParameter("allow_reason is %d bytes, over the %d byte cap", len(r.AllowReason), maxReasonBytes)
	}
	if r.DenyReasonHint != "" && r.DenyCodeHint == "" {
		return trace.BadParameter("deny_reason_hint set without deny_code_hint")
	}
	if r.DenyCodeHint != "" {
		if err := validateAuditCode(r.DenyCodeHint); err != nil {
			return trace.Wrap(err, "invalid deny_code_hint")
		}
		if r.Where == "" {
			return trace.BadParameter("deny_code_hint set without a where clause")
		}
	}
	if len(r.DenyReasonHint) > maxReasonBytes {
		return trace.BadParameter("deny_reason_hint is %d bytes, over the %d byte cap", len(r.DenyReasonHint), maxReasonBytes)
	}
	return nil
}

// validateAllowAllStandsAlone rejects an allow_all rule that also sets another
// field.
func (r Rule) validateAllowAllStandsAlone() error {
	if len(r.Paths) > 0 || len(r.Methods) > 0 || r.Where != "" ||
		len(r.AllowEncoded) > 0 || r.AllowCode != "" || r.AllowReason != "" ||
		r.DenyCodeHint != "" || r.DenyReasonHint != "" {
		return trace.BadParameter("allow_all cannot be combined with any other field")
	}
	return nil
}

// validatePaths checks the count and byte caps on a rule's path patterns. The
// pattern syntax is checked when the rule compiles.
func validatePaths(paths []string) error {
	if len(paths) > maxPaths {
		return trace.BadParameter("a rule holds %d paths, over the cap of %d", len(paths), maxPaths)
	}
	for _, p := range paths {
		if len(p) > maxPathBytes {
			return trace.BadParameter("path is %d bytes, over the %d byte cap", len(p), maxPathBytes)
		}
	}
	return nil
}

// validateMethods rejects a name outside validMethods, folded to upper case, so
// "get" passes and "GTE" fails.
func validateMethods(methods []string) error {
	for _, m := range methods {
		if !slices.Contains(validMethods, strings.ToUpper(m)) {
			return trace.BadParameter("method %q is not one of %s", m, strings.Join(validMethods, ", "))
		}
	}
	return nil
}

// validateWhere checks the byte cap on a sugared rule's where clause. The where
// language itself is checked when the clause compiles.
func validateWhere(where string) error {
	if where == "" {
		return nil
	}
	if len(where) > maxWhereBytes {
		return trace.BadParameter("where clause is %d bytes, over the %d byte cap", len(where), maxWhereBytes)
	}
	return nil
}
