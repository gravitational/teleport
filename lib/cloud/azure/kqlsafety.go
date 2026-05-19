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

// KQL (Kusto Query Language) injection safety for Azure Resource Graph (ARG) queries.
//
// This file is the single source of truth for protecting the KQL query string
// from injection by caller-supplied filter values. Three layers, in order of
// strength:
//
//  1. Allowlist (primary). sanitizeQueryVMsParams rejects any non-wildcard
//     Region/ResourceGroup outside its per-field regex (azureRegionPattern,
//     azureResourceGroupPattern), and rejects any OSType outside the closed
//     set of canonical OSType constants -- all before any KQL is built.
//     Empty and untrimmed values are rejected too. Subscription IDs are
//     also validated here for non-empty, trimmed, canonical hyphenated UUID form.
//  2. Parameterization. SubscriptionIDs flow through the typed SDK field
//     (armresourcegraph.QueryRequest.Subscriptions in queryChunk), never the
//     KQL string. Zero KQL injection surface for subscription IDs.
//  3. Defense-in-depth. quoteKQL/escapeKQL double single quotes inside KQL
//     string literals. FuzzEscapeKQL pins the no-breakout invariant (every
//     single quote in the output is part of a doubled pair); FuzzQuoteKQL
//     pins semantic parity with an in-test port of Azure/azure-kusto-go's
//     QuoteString. Belt-and-suspenders on top of the allowlist.
//
// Contract for adding new KQL-bound inputs: every value that flows into KQL
// MUST pass through sanitizeQueryVMsParams (or an equivalent allowlist
// validator), and every value interpolated as a string literal MUST go
// through quoteKQL.
//
// No existing Teleport code or third-party dependency provides this; the
// hand-rolled escape is justified by (a) the strict allowlist excludes every
// escapable byte, and (b) FuzzQuoteKQL parity against an in-test port of the
// canonical Azure/azure-kusto-go QuoteString. Vendoring the full Kusto SDK
// for one ReplaceAll would add dependency surface for zero behavior change.

package azure

import (
	"regexp"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// sanitizedParams is QueryVMsParams after passing every check in sanitizeQueryVMsParams.
// The only intended constructor of this type is sanitizeQueryVMsParams. Outside the
// azure package, sanitizedParams cannot be manufactured at all (the type is unexported);
// inside the package, literal construction is technically possible but is the kind of
// thing code review must catch -- the type's role is to make the safe form the only
// form that compiles for any caller that doesn't already live alongside this file.
//
// Field order intentionally differs from QueryVMsParams so the underlying types are
// not identical: that prevents sanitizedParams(rawParams) direct conversion from
// compiling, which would otherwise be a one-line back door around sanitizeQueryVMsParams.
// Do not "fix" the order to match QueryVMsParams; the divergence is load-bearing.
type sanitizedParams struct {
	// KQL-bound fields come first. Regions and ResourceGroups are regex-allowlisted
	// via sanitizeKQLValues; OSTypes is closed-set validated via sanitizeOSTypes.
	Regions        []string
	ResourceGroups []string
	OSTypes        []OSType
	// SubscriptionIDs uses typed SDK parameterization, not KQL interpolation.
	SubscriptionIDs []string
}

// Defense-in-depth regex allowlists for caller-supplied string values that flow into KQL
// string literals. Each is a Teleport-supported safe subset of Azure's naming surface,
// narrow enough to reject any character that could break out of a single-quoted KQL string
// (quote, backslash, newline, null, etc.). OSType is validated separately by sanitizeOSTypes
// against the closed set of OSType constants and so has no regex here.
//
// types.Wildcard ("*") is whitelisted by sanitizeKQLValues; the predicate
// helpers absorb wildcards before any KQL is generated.
var (
	// azureRegionPattern is an injection-safety allowlist for region values: alphanumeric only,
	// matching the canonical form ARG stores in the `location` column (e.g. "eastus", "westeurope").
	// Display names like "East US" or "(US) East US" appear in Azure UI and sample responses
	// but ARG normalizes location to canonical for filtering.
	//
	// See ARG property normalization (the page documents that some properties are lowercased;
	// sample responses on the same page show `location` in canonical form like "eastus"):
	// https://learn.microsoft.com/en-us/azure/governance/resource-graph/concepts/explore-resources
	azureRegionPattern = regexp.MustCompile(`^[A-Za-z0-9]+$`)

	// azureResourceGroupPattern is a Teleport-defined ASCII subset of Azure's resource group
	// naming surface: letters, digits, underscore, hyphen, period, and parenthesis. Azure's
	// canonical pattern (^[-\w\._\(\)]+$) uses `\w` which includes Unicode word chars;
	// we narrow to ASCII for a well-bounded allowlist. Length (1-90) is enforced by Azure.
	//
	// See the Microsoft.Resources TypeSpec source. The `NamePattern` argument to `ResourceNameParameter`
	// on the ResourceGroup model declares the regex once at the source level; the generated swagger
	// `pattern` fields on every `resourceGroupName` parameter (Get, Create, Update, Delete, etc.) all
	// come from this one declaration:
	// https://github.com/Azure/azure-rest-api-specs/blob/d2765895f8a8ddf6c540fbf1ebfdf2ea7bfe490f/specification/resources/resource-manager/Microsoft.Resources/resources/ResourceGroup.tsp#L19-L24
	//
	// The trailing character class excludes period to match Azure's "cannot end with
	// period" rule for resource group names.
	azureResourceGroupPattern = regexp.MustCompile(`^[A-Za-z0-9_.()-]*[A-Za-z0-9_()-]$`)
)

// azureSubscriptionIDLen is the length of a canonical Azure subscription ID:
// the 36-char hyphenated UUID form (8-4-4-4-12) that the Azure SDK and Teleport
// elsewhere (e.g. lib/join/azurejoin generates them with uuid.NewString) treat
// as the subscription's identity. uuid.Validate accepts looser variants --
// urn:uuid:..., {...}, and unhyphenated 32-char hex -- that real Azure
// subscriptions never use; pairing the length check with uuid.Validate keeps
// this validator's accepted set identical to the prior regex
// (^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$)
// and surfaces malformed input as a clear caller-side BadParameter rather than
// pushing it downstream.
const azureSubscriptionIDLen = 36

// sanitizeQueryVMsParams enforces the input contract shared by QueryVMs and the ARMResourceGraphMock:
// a non-empty subscription list with no empty, untrimmed, or non-canonical-UUID entries, and per-field
// allowlist validation for the filter slices. Returns sanitizedParams: the only path to a value
// buildVMDiscoveryKQL accepts. Centralized so the mock cannot drift from production behavior.
func sanitizeQueryVMsParams(params QueryVMsParams) (sanitizedParams, error) {
	if len(params.SubscriptionIDs) == 0 {
		return sanitizedParams{}, trace.BadParameter("at least one subscription ID is required")
	}

	for _, id := range params.SubscriptionIDs {
		switch {
		case id == "":
			return sanitizedParams{}, trace.BadParameter("subscription ID must not be empty")
		case strings.TrimSpace(id) != id:
			return sanitizedParams{}, trace.BadParameter("subscription ID %q must not have leading or trailing whitespace", id)
		case len(id) != azureSubscriptionIDLen || uuid.Validate(id) != nil:
			return sanitizedParams{}, trace.BadParameter("subscription ID %q must be a canonical UUID (e.g. \"11111111-1111-1111-1111-111111111111\")", id)
		}
	}

	if err := sanitizeKQLValues(params.Regions, azureRegionPattern, "region"); err != nil {
		return sanitizedParams{}, trace.Wrap(err)
	}
	if err := sanitizeKQLValues(params.ResourceGroups, azureResourceGroupPattern, "resource group"); err != nil {
		return sanitizedParams{}, trace.Wrap(err)
	}
	if err := sanitizeOSTypes(params.OSTypes); err != nil {
		return sanitizedParams{}, trace.Wrap(err)
	}

	// Clone every slice so the validated value cannot be mutated by the caller after
	// validation. Without this, params.Regions[0] = "evil" after sanitize would silently
	// alias into the sanitized result, undermining the "validated once, safe thereafter"
	// invariant.
	return sanitizedParams{
		SubscriptionIDs: slices.Clone(params.SubscriptionIDs),
		Regions:         slices.Clone(params.Regions),
		ResourceGroups:  slices.Clone(params.ResourceGroups),
		OSTypes:         slices.Clone(params.OSTypes),
	}, nil
}

// sanitizeKQLValues rejects any non-wildcard entry that is empty, untrimmed, or doesn't
// match the supplied pattern. kind is a human-readable label used in error messages.
// types.Wildcard is whitelisted because predicate helpers absorb it before KQL is built.
func sanitizeKQLValues(values []string, pattern *regexp.Regexp, kind string) error {
	for _, v := range values {
		switch {
		case v == types.Wildcard:
			continue
		case v == "":
			return trace.BadParameter("%s must not be empty", kind)
		case strings.TrimSpace(v) != v:
			return trace.BadParameter("%s %q must not have leading or trailing whitespace", kind, v)
		case !pattern.MatchString(v):
			return trace.BadParameter("%s %q contains invalid characters; allowed pattern: %s", kind, v, pattern.String())
		}
	}

	return nil
}

// sanitizeOSTypes validates the closed set of canonical Azure VM OS type values
// against the OSType* constants. The OSType named type guides callers through
// the constants at compile time; this runtime check rejects values that bypass
// the type (e.g. literal OSType("linux") conversions, zero values, or untrimmed
// entries from caller-side config parsing). Strict canonical case per Azure's
// documented enum -- no whitespace tolerance, no case folding.
//
// Branches are ordered to give the caller the most actionable error for their
// specific mistake (empty, untrimmed, unknown value) rather than a single
// catch-all "must be Linux or Windows" for every failure mode.
//
// types.Wildcard is whitelisted because predicate helpers absorb it before KQL is built.
func sanitizeOSTypes(values []OSType) error {
	for _, v := range values {
		switch {
		case string(v) == types.Wildcard:
			continue
		case v == "":
			return trace.BadParameter("OS type must not be empty")
		case strings.TrimSpace(string(v)) != string(v):
			return trace.BadParameter("OS type %q must not have leading or trailing whitespace", v)
		case v != OSTypeLinux && v != OSTypeWindows:
			return trace.BadParameter("OS type %q must be %q or %q", v, OSTypeLinux, OSTypeWindows)
		}
	}

	return nil
}

// quoteKQL escapes single quotes in s and wraps the result in single quotes to produce a KQL string literal.
//
// PRECONDITION: s must have already passed sanitizeKQLValues, sanitizeOSTypes,
// or an equivalent per-field allowlist. quoteKQL is not a general-purpose
// arbitrary-string KQL encoder; it is the defense-in-depth layer that runs
// only after the allowlist has excluded every escapable byte except single quotes.
func quoteKQL(s string) string {
	return "'" + escapeKQL(s) + "'"
}

// escapeKQL escapes single quotes in a KQL string literal by doubling them.
//
// This is the minimum subset of Azure Kusto's string quoting behavior that suffices
// for single-quoted literals; allowlisted input (sanitizeKQLValues) guarantees no other
// escapable byte ever reaches this function. FuzzQuoteKQL compares its output against an
// in-test port of Azure/azure-kusto-go's QuoteString to keep that invariant honest. Vendoring
// the full SDK escape lib would add dependency surface for behavior the allowlist already excludes.
func escapeKQL(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
