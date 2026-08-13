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

package auth_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/label"
	"github.com/gravitational/teleport/lib/utils"
)

// A small value set, so generated selectors and app labels overlap often.
// Selector values may be globs, which services.MatchLabels matches as
// regular expressions. App labels are always literal, and no literal
// matches a glob, which pickMatchingAppLabels relies on.
var (
	appLabelKeys           = []string{"env", "vendor", "team", "region"}
	appLabelValues         = []string{"prod", "dev", "gitlab", "github", "blue", "green", "us", "eu"}
	appLabelSelectorValues = slices.Concat(appLabelValues, []string{"prod-*", "web-*"})
)

// genAppLabelSelector draws a wildcard, an empty selector, or a few keys
// with one or two values each. An empty selector matches no app. Sample
// return values:
//
//	types.Labels{"*": {"*"}}
//	types.Labels{}
//	types.Labels{"env": {"prod"}, "team": {"blue", "web-*"}}
func genAppLabelSelector(t *rapid.T) types.Labels {
	if rapid.Bool().Draw(t, "wildcard") {
		return types.Labels{types.Wildcard: []string{types.Wildcard}}
	}
	keyGen := rapid.SliceOfNDistinct(rapid.SampledFrom(appLabelKeys), 0, len(appLabelKeys), rapid.ID[string])
	valueGen := rapid.SliceOfNDistinct(rapid.SampledFrom(appLabelSelectorValues), 1, 2, rapid.ID[string])
	selector := types.Labels{}
	for _, key := range keyGen.Draw(t, "sel_keys") {
		selector[key] = valueGen.Draw(t, "sel_vals_"+key)
	}
	return selector
}

// genAppLabels draws the labels of one candidate app. Sample return values:
//
//	map[string]string{}
//	map[string]string{"env": "prod"}
//	map[string]string{"env": "dev", "region": "us"}
func genAppLabels(t *rapid.T) map[string]string {
	keyGen := rapid.SliceOfNDistinct(rapid.SampledFrom(appLabelKeys), 0, len(appLabelKeys), rapid.ID[string])
	labels := map[string]string{}
	for _, key := range keyGen.Draw(t, "app_keys") {
		labels[key] = rapid.SampledFrom(appLabelValues).Draw(t, "app_val_"+key)
	}
	return labels
}

// genAppLabelSets draws the labels of one to eight candidate apps. Sample
// return value:
//
//	[]map[string]string{{"env": "prod"}, {}, {"team": "blue"}}
func genAppLabelSets(t *rapid.T) []map[string]string {
	return rapid.SliceOfN(rapid.Custom(genAppLabels), 1, 8).Draw(t, "app_label_sets")
}

// pickMatchingAppLabels builds app labels that the selector matches and
// that also satisfy the label expression. A glob selector value is turned
// into a literal that matches it. It returns nil when no app can satisfy
// both, and for an empty selector with no expression.
//
//	{"env": {"prod", "dev"}}                  -> {"env": "prod"}
//	{"env": {"prod"}, "team": {"blue"}}       -> {"env": "prod", "team": "blue"}
//	{"env": {"prod-*"}}                       -> {"env": "prod-x"}
//	{"*": {"*"}} and region == "us"           -> {"region": "us"}
//	{"env": {"prod", "dev"}} and env == "dev" -> {"env": "dev"}
//	{"region": {"eu"}} and region == "us"     -> nil
func pickMatchingAppLabels(selector types.Labels, exprKey, exprValue string) map[string]string {
	labels := map[string]string{}
	for key, values := range selector {
		if key == types.Wildcard {
			continue
		}
		value := values[0]
		if strings.HasSuffix(value, "*") {
			value = strings.TrimSuffix(value, "*") + "x"
		}
		labels[key] = value
	}
	if exprKey == "" {
		if len(selector) == 0 {
			return nil
		}
		return labels
	}
	if values, ok := selector[exprKey]; ok {
		match, err := utils.SliceMatchesRegex(exprValue, values)
		if err != nil || !match {
			return nil
		}
	}
	labels[exprKey] = exprValue
	return labels
}

// genPreV19ClientVersion draws a version below minSupportedRoleV9Version.
// The version comparison drops any pre-release suffix, so a pre-release
// client downgrades like a release one.
func genPreV19ClientVersion(t *rapid.T) *semver.Version {
	major := rapid.Int64Range(1, 18).Draw(t, "major")
	minor := rapid.Int64Range(0, 20).Draw(t, "minor")
	patch := rapid.Int64Range(0, 20).Draw(t, "patch")
	preRelease := rapid.SampledFrom([]string{"", "dev.1", "beta.3"}).Draw(t, "pre_release")
	return &semver.Version{Major: major, Minor: minor, Patch: patch, PreRelease: semver.PreRelease(preRelease)}
}

// TestProperty_DowngradeV8_DeniesEveryPreviouslyAllowedApp asserts three
// properties:
//   - The downgraded copy denies every app the v9 role allowed.
//   - The downgraded copy still denies every app the v9 role denied.
//   - The input role is never modified (it is shared with other callers).
//
// The properties call the downgrade directly. TestRoleVersionV9Downgrade
// covers the wiring that reaches it.
func TestProperty_DowngradeV8_DeniesEveryPreviouslyAllowedApp(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		allowLabels := genAppLabelSelector(t)
		denyLabels := genAppLabelSelector(t)
		appLabelSets := genAppLabelSets(t)
		clientVersion := genPreV19ClientVersion(t)

		// Random apps rarely match a keyed selector, so add one that does
		// for each side.
		if match := pickMatchingAppLabels(allowLabels, "", ""); match != nil {
			appLabelSets = append(appLabelSets, match)
		}
		if match := pickMatchingAppLabels(denyLabels, "", ""); match != nil {
			appLabelSets = append(appLabelSets, match)
		}

		// None of these is a pure allow_all, so the downgrade strips the
		// allow app labels.
		allowRules := rapid.SampledFrom([][]types.AppResource{nil, {{}}, {{AllowAll: true}, {}}}).Draw(t, "allow_rules")
		denyRules := rapid.SampledFrom([][]types.AppResource{nil, {{}}}).Draw(t, "deny_rules")
		require.False(t, types.AppResourcesAllowAll(allowRules, denyRules), "generator produced a pure allow_all role")

		// The labels map must be non-empty. The downgrade writes into it in
		// place, which a shallow copy would leak back to the caller.
		metadata := types.Metadata{Name: "dev", Labels: map[string]string{"owner": "core"}}
		role := &types.RoleV6{
			Kind:     types.KindRole,
			Metadata: metadata,
			Version:  types.V9,
			Spec: types.RoleSpecV6{
				Allow: types.RoleConditions{AppLabels: allowLabels, AppResources: allowRules},
				Deny:  types.RoleConditions{AppLabels: denyLabels, AppResources: denyRules},
			},
		}

		before := apiutils.CloneProtoMsg(role)
		got := auth.MaybeDowngradeRoleVersionToV8(t.Context(), role, clientVersion)

		require.Equal(t, types.V8, got.GetVersion())
		require.Nil(t, got.Spec.Allow.AppResources)
		require.Nil(t, got.Spec.Deny.AppResources)
		require.Empty(t, got.Spec.Allow.AppLabels)
		require.NotEmpty(t, got.GetMetadata().Labels[types.TeleportDowngradedLabel])
		require.Equal(t, "core", got.GetMetadata().Labels["owner"])

		require.Equal(t, before, apiutils.CloneProtoMsg(role), "downgrade modified the input role")

		denyMatchers := types.LabelMatchers{Labels: got.Spec.Deny.AppLabels, Expression: got.Spec.Deny.AppLabelsExpression}
		for _, appLabels := range appLabelSets {
			allowedBefore, _, err := services.MatchLabels(allowLabels, appLabels)
			require.NoError(t, err)
			deniedBefore, _, err := services.MatchLabels(denyLabels, appLabels)
			require.NoError(t, err)
			if !allowedBefore && !deniedBefore {
				continue
			}
			deniedAfter, _, err := services.CheckLabelsMatch(types.Deny, denyMatchers, "", nil, label.MapLabelGetter(appLabels), false)
			require.NoError(t, err)
			require.True(t, deniedAfter,
				"app allowed or denied before downgrade must be denied after: app=%v allowLabels=%v denyLabels=%v resultDeny=%v",
				appLabels, allowLabels, denyLabels, got.Spec.Deny.AppLabels)
		}
	})
}

// TestProperty_DowngradeV8_FailsClosedWithBothSelectors asserts that the
// downgraded copy denies every app the v9 role allowed or denied, whether
// the role selects by app_labels, app_labels_expression, or both, and
// that the input role is never modified.
func TestProperty_DowngradeV8_FailsClosedWithBothSelectors(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		allowLabels := genAppLabelSelector(t)
		exprKey, exprValue, allowExpression := "", "", ""
		if rapid.Bool().Draw(t, "has_allow_expression") {
			exprKey = rapid.SampledFrom(appLabelKeys).Draw(t, "expr_key")
			exprValue = rapid.SampledFrom(appLabelValues).Draw(t, "expr_value")
			allowExpression = fmt.Sprintf("labels[%q] == %q", exprKey, exprValue)
		}
		denyLabels := rapid.SampledFrom([]types.Labels{nil, {"region": []string{"us"}}}).Draw(t, "deny_labels")
		denyExprKey, denyExprValue := "region", "eu"
		denyExprText := fmt.Sprintf("labels[%q] == %q", denyExprKey, denyExprValue)
		denyExpression := rapid.SampledFrom([]string{"", denyExprText}).Draw(t, "deny_expression")
		appLabelSets := genAppLabelSets(t)
		clientVersion := genPreV19ClientVersion(t)

		// Random apps rarely match a keyed selector, so add one for the
		// allow selector, the deny selector, and the deny expression.
		if match := pickMatchingAppLabels(allowLabels, exprKey, exprValue); match != nil {
			appLabelSets = append(appLabelSets, match)
		}
		if match := pickMatchingAppLabels(denyLabels, "", ""); match != nil {
			appLabelSets = append(appLabelSets, match)
		}
		if denyExpression != "" {
			appLabelSets = append(appLabelSets, map[string]string{denyExprKey: denyExprValue})
		}

		role := &types.RoleV6{
			Kind:     types.KindRole,
			Metadata: types.Metadata{Name: "dev"},
			Version:  types.V9,
			Spec: types.RoleSpecV6{
				Allow: types.RoleConditions{AppLabels: allowLabels, AppLabelsExpression: allowExpression, AppResources: []types.AppResource{{}}},
				Deny:  types.RoleConditions{AppLabels: denyLabels, AppLabelsExpression: denyExpression},
			},
		}

		before := apiutils.CloneProtoMsg(role)
		got := auth.MaybeDowngradeRoleVersionToV8(t.Context(), role, clientVersion)
		require.Equal(t, types.V8, got.GetVersion())
		require.Empty(t, got.Spec.Allow.AppLabels)
		require.Empty(t, got.Spec.Allow.AppLabelsExpression)
		require.Equal(t, before, apiutils.CloneProtoMsg(role), "downgrade modified the input role")

		allowMatchers := types.LabelMatchers{Labels: allowLabels, Expression: allowExpression}
		inputDenyMatchers := types.LabelMatchers{Labels: denyLabels, Expression: denyExpression}
		denyMatchers := types.LabelMatchers{Labels: got.Spec.Deny.AppLabels, Expression: got.Spec.Deny.AppLabelsExpression}
		for _, appLabels := range appLabelSets {
			app := label.MapLabelGetter(appLabels)
			allowed, _, err := services.CheckLabelsMatch(types.Allow, allowMatchers, "", nil, app, false)
			require.NoError(t, err)
			deniedBefore, _, err := services.CheckLabelsMatch(types.Deny, inputDenyMatchers, "", nil, app, false)
			require.NoError(t, err)
			if !allowed && !deniedBefore {
				continue
			}
			denied, _, err := services.CheckLabelsMatch(types.Deny, denyMatchers, "", nil, app, false)
			require.NoError(t, err)
			require.True(t, denied,
				"app allowed or denied before downgrade must be denied after: app=%v allowLabels=%v allowExpr=%q denyLabels=%v denyExpr=%q resultDeny=%v resultDenyExpr=%q",
				appLabels, allowLabels, allowExpression, denyLabels, denyExpression, got.Spec.Deny.AppLabels, got.Spec.Deny.AppLabelsExpression)
		}
	})
}

// TestDowngradeV8_ConservativeOverDeny asserts that the downgraded copy
// can also deny certain apps the v9 role never allowed.
//
// An allow rule needs both app_labels and app_labels_expression to match,
// while a deny rule needs only one, so moving them across cannot preserve
// the allow set exactly. Over-denying is the deliberate and documented
// choice.
func TestDowngradeV8_ConservativeOverDeny(t *testing.T) {
	role := &types.RoleV6{
		Kind:     types.KindRole,
		Metadata: types.Metadata{Name: "dev"},
		Version:  types.V9,
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				AppLabels:           types.Labels{types.Wildcard: []string{types.Wildcard}},
				AppLabelsExpression: `labels["env"] == "prod"`,
				AppResources:        []types.AppResource{{}},
			},
		},
	}
	got := auth.MaybeDowngradeRoleVersionToV8(t.Context(), role, &semver.Version{Major: 18, Minor: 1, Patch: 2})
	denyMatchers := types.LabelMatchers{Labels: got.Spec.Deny.AppLabels, Expression: got.Spec.Deny.AppLabelsExpression}

	assertDenied := func(t *testing.T, labels map[string]string) {
		t.Helper()
		app := label.MapLabelGetter(labels)
		denied, _, err := services.CheckLabelsMatch(types.Deny, denyMatchers, "", nil, app, false)
		require.NoError(t, err)
		require.True(t, denied)
	}

	// An app the role allowed.
	assertDenied(t, map[string]string{"env": "prod"})
	// An app the role never allowed, denied by the moved wildcard labels.
	assertDenied(t, map[string]string{"env": "dev"})
}
