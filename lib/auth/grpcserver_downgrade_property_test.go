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
	"context"
	"fmt"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
)

// labelKeys and labelVals seed a small shared vocabulary so generated
// selectors and app labels overlap often enough to make the match-set
// assertions non-vacuous.
var (
	labelKeys = []string{"env", "vendor", "team", "region"}
	labelVals = []string{"prod", "dev", "gitlab", "github", "blue", "green", "us", "eu"}
)

// genAppLabelSelector draws a role-side app label selector: a wildcard, an
// empty selector (which matches nothing), or one to a few keys each mapped to
// one or two values from the shared vocabulary.
func genAppLabelSelector(t *rapid.T) types.Labels {
	if rapid.Bool().Draw(t, "wildcard") {
		return types.Labels{types.Wildcard: []string{types.Wildcard}}
	}
	keys := rapid.SliceOfNDistinct(rapid.SampledFrom(labelKeys), 0, len(labelKeys), func(k string) string { return k }).Draw(t, "sel_keys")
	selector := types.Labels{}
	for _, key := range keys {
		selector[key] = rapid.SliceOfNDistinct(rapid.SampledFrom(labelVals), 1, 2, func(v string) string { return v }).Draw(t, "sel_vals_"+key)
	}
	return selector
}

// genAppLabels draws the label set of a candidate app: zero to a few keys each
// mapped to a single value from the shared vocabulary.
func genAppLabels(t *rapid.T) map[string]string {
	keys := rapid.SliceOfNDistinct(rapid.SampledFrom(labelKeys), 0, len(labelKeys), func(k string) string { return k }).Draw(t, "app_keys")
	labels := map[string]string{}
	for _, key := range keys {
		labels[key] = rapid.SampledFrom(labelVals).Draw(t, "app_val_"+key)
	}
	return labels
}

// genAppPool draws a handful of candidate apps to test selector match sets
// against.
func genAppPool(t *rapid.T) []map[string]string {
	return rapid.SliceOfN(rapid.Custom(genAppLabels), 1, 8).Draw(t, "apps")
}

// genPreV9ClientVersion draws a client version below minSupportedRoleV9Version
// (v19), the range that forces a downgrade.
func genPreV9ClientVersion(t *rapid.T) *semver.Version {
	major := rapid.IntRange(1, 18).Draw(t, "major")
	minor := rapid.IntRange(0, 20).Draw(t, "minor")
	patch := rapid.IntRange(0, 20).Draw(t, "patch")
	return semver.New(fmt.Sprintf("%d.%d.%d", major, minor, patch))
}

// TestProperty_DowngradeV8_DeniesEveryPreviouslyAllowedApp is the fail-closed
// security property: when a pre-v19 agent cannot enforce a v9 role's app
// restriction, the downgraded v8 copy must not leave any of the role's own
// apps reachable. For any v9 role whose app_resources are not a pure allow_all,
// every app the role's allow selector matched must be matched by the
// downgraded role's deny selector, so a pre-v19 agent denies it.
func TestProperty_DowngradeV8_DeniesEveryPreviouslyAllowedApp(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		allowLabels := genAppLabelSelector(t)
		denyLabels := genAppLabelSelector(t)
		apps := genAppPool(t)
		clientVersion := genPreV9ClientVersion(t)

		// Every combination below is non-pure-allow_all, so the downgrade
		// takes the strip path rather than the "access unchanged" path.
		allowRules := rapid.SampledFrom([][]types.AppResource{nil, {{}}, {{AllowAll: true}, {}}}).Draw(t, "allow_rules")
		denyRules := rapid.SampledFrom([][]types.AppResource{nil, {{}}}).Draw(t, "deny_rules")
		require.False(t, types.AppResourcesAllowAll(allowRules, denyRules), "generator produced a pure allow_all role")

		role := &types.RoleV6{
			Kind:     types.KindRole,
			Metadata: types.Metadata{Name: "dev"},
			Version:  types.V9,
			Spec: types.RoleSpecV6{
				Allow: types.RoleConditions{AppLabels: allowLabels, AppResources: allowRules},
				Deny:  types.RoleConditions{AppLabels: denyLabels, AppResources: denyRules},
			},
		}

		got := auth.MaybeDowngradeRoleVersionToV8(context.Background(), role, clientVersion)

		require.Equal(t, types.V8, got.GetVersion())
		require.Nil(t, got.Spec.Allow.AppResources)
		require.Nil(t, got.Spec.Deny.AppResources)
		require.Empty(t, got.Spec.Allow.AppLabels)
		require.Empty(t, got.Spec.Allow.AppLabelsExpression)
		require.NotEmpty(t, got.GetMetadata().Labels[types.TeleportDowngradedLabel])

		for _, app := range apps {
			allowedBefore, _, err := services.MatchLabels(allowLabels, app)
			require.NoError(t, err)
			if !allowedBefore {
				continue
			}
			deniedAfter, _, err := services.MatchLabels(got.Spec.Deny.AppLabels, app)
			require.NoError(t, err)
			require.True(t, deniedAfter,
				"app allowed before downgrade must be denied after: app=%v allowLabels=%v resultDeny=%v",
				app, allowLabels, got.Spec.Deny.AppLabels)
		}
	})
}

// TestProperty_DowngradeV8_CarriesAllowExpressionIntoDeny pins the expression
// half of the strip path structurally, since the where predicate language has
// no cheap match-set oracle. For any non-pure-allow_all v9 role with a
// non-empty allow app labels expression, the downgrade must clear the allow
// expression and carry the original expression into the deny expression.
func TestProperty_DowngradeV8_CarriesAllowExpressionIntoDeny(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		allowExpression := rapid.SampledFrom([]string{
			`labels["vendor"] == "gitlab"`,
			`labels["env"] == "prod"`,
			`equals(labels["team"], "blue")`,
		}).Draw(t, "allow_expression")
		denyExpression := rapid.SampledFrom([]string{"", `labels["region"] == "us"`}).Draw(t, "deny_expression")
		clientVersion := genPreV9ClientVersion(t)

		role := &types.RoleV6{
			Kind:     types.KindRole,
			Metadata: types.Metadata{Name: "dev"},
			Version:  types.V9,
			Spec: types.RoleSpecV6{
				Allow: types.RoleConditions{AppLabelsExpression: allowExpression, AppResources: []types.AppResource{{}}},
				Deny:  types.RoleConditions{AppLabelsExpression: denyExpression},
			},
		}

		got := auth.MaybeDowngradeRoleVersionToV8(context.Background(), role, clientVersion)

		require.Equal(t, types.V8, got.GetVersion())
		require.Empty(t, got.Spec.Allow.AppLabelsExpression)
		require.Contains(t, got.Spec.Deny.AppLabelsExpression, allowExpression,
			"allow expression must survive on the deny side: allow=%q deny=%q result=%q",
			allowExpression, denyExpression, got.Spec.Deny.AppLabelsExpression)
	})
}

// TestProperty_DowngradeV8_AllowAllPreservesAppAccess is the fail-open
// complement: a v9 role that grants unrestricted app access through a pure
// allow_all rule must keep the exact same app access after downgrade, since a
// plain v8 role already grants it. For any such role, the allow selector is
// unchanged, so every app matches the allow selector the same way before and
// after.
func TestProperty_DowngradeV8_AllowAllPreservesAppAccess(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		allowLabels := genAppLabelSelector(t)
		allowExpression := rapid.SampledFrom([]string{"", `labels["vendor"] == "gitlab"`}).Draw(t, "allow_expression")
		apps := genAppPool(t)
		clientVersion := genPreV9ClientVersion(t)

		allowRules := []types.AppResource{{AllowAll: true}}
		require.True(t, types.AppResourcesAllowAll(allowRules, nil), "generator did not produce a pure allow_all role")

		role := &types.RoleV6{
			Kind:     types.KindRole,
			Metadata: types.Metadata{Name: "dev"},
			Version:  types.V9,
			Spec: types.RoleSpecV6{
				Allow: types.RoleConditions{AppLabels: allowLabels, AppLabelsExpression: allowExpression, AppResources: allowRules},
			},
		}

		got := auth.MaybeDowngradeRoleVersionToV8(context.Background(), role, clientVersion)

		require.Equal(t, types.V8, got.GetVersion())
		require.Nil(t, got.Spec.Allow.AppResources)
		require.Equal(t, allowExpression, got.Spec.Allow.AppLabelsExpression)
		require.Contains(t, got.GetMetadata().Labels[types.TeleportDowngradedLabel], "app access is unchanged")

		for _, app := range apps {
			allowedBefore, _, err := services.MatchLabels(allowLabels, app)
			require.NoError(t, err)
			allowedAfter, _, err := services.MatchLabels(got.Spec.Allow.AppLabels, app)
			require.NoError(t, err)
			require.Equal(t, allowedBefore, allowedAfter,
				"allow_all downgrade changed app access: app=%v allowLabels=%v resultAllow=%v",
				app, allowLabels, got.Spec.Allow.AppLabels)
		}
	})
}

// genApps draws a handful of candidate apps as concrete resources, so
// CheckLabelsMatch can evaluate both the label selector and the label
// expression against them.
func genApps(t *rapid.T) []*types.AppV3 {
	labelSets := rapid.SliceOfN(rapid.Custom(genAppLabels), 1, 8).Draw(t, "app_label_sets")
	apps := make([]*types.AppV3, 0, len(labelSets))
	for i, labels := range labelSets {
		app, err := types.NewAppV3(
			types.Metadata{Name: fmt.Sprintf("app-%d", i), Labels: labels},
			types.AppSpecV3{URI: "http://localhost"},
		)
		require.NoError(t, err)
		apps = append(apps, app)
	}
	return apps
}

// TestProperty_DowngradeV8_FailsClosedWithBothSelectors covers the conservative
// AND-to-OR downgrade. An allow rule matches when both app_labels and
// app_labels_expression match, while a deny rule matches when either does. The
// downgrade moves the two selectors to the deny side separately, so the deny
// evaluates as an OR of what the allow evaluated as an AND. That over-denies,
// which is accepted because it fails closed: for any v9 role that sets both
// selectors and is not a pure allow_all, every app the allow rule matched must
// still be denied after the downgrade. This exercises the wildcard-fallback and
// expression-combine branches that the single-selector properties do not.
func TestProperty_DowngradeV8_FailsClosedWithBothSelectors(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		allowLabels := genAppLabelSelector(t)
		allowExpression := rapid.SampledFrom([]string{
			`labels["vendor"] == "gitlab"`,
			`labels["env"] == "prod"`,
			`labels["team"] == "blue"`,
		}).Draw(t, "allow_expression")
		denyLabels := rapid.SampledFrom([]types.Labels{nil, {"region": []string{"us"}}}).Draw(t, "deny_labels")
		denyExpression := rapid.SampledFrom([]string{"", `labels["region"] == "eu"`}).Draw(t, "deny_expression")
		apps := genApps(t)
		clientVersion := genPreV9ClientVersion(t)

		role := &types.RoleV6{
			Kind:     types.KindRole,
			Metadata: types.Metadata{Name: "dev"},
			Version:  types.V9,
			Spec: types.RoleSpecV6{
				Allow: types.RoleConditions{AppLabels: allowLabels, AppLabelsExpression: allowExpression, AppResources: []types.AppResource{{}}},
				Deny:  types.RoleConditions{AppLabels: denyLabels, AppLabelsExpression: denyExpression},
			},
		}

		got := auth.MaybeDowngradeRoleVersionToV8(context.Background(), role, clientVersion)
		require.Equal(t, types.V8, got.GetVersion())
		require.Empty(t, got.Spec.Allow.AppLabels)
		require.Empty(t, got.Spec.Allow.AppLabelsExpression)

		allowMatchers := types.LabelMatchers{Labels: allowLabels, Expression: allowExpression}
		denyMatchers := types.LabelMatchers{Labels: got.Spec.Deny.AppLabels, Expression: got.Spec.Deny.AppLabelsExpression}
		for _, app := range apps {
			allowed, _, err := services.CheckLabelsMatch(types.Allow, allowMatchers, "", nil, app, false)
			require.NoError(t, err)
			if !allowed {
				continue
			}
			denied, _, err := services.CheckLabelsMatch(types.Deny, denyMatchers, "", nil, app, false)
			require.NoError(t, err)
			require.True(t, denied,
				"app allowed by (labels AND expression) must be denied after downgrade: app=%v allowLabels=%v allowExpr=%q resultDeny=%v resultDenyExpr=%q",
				app.GetAllLabels(), allowLabels, allowExpression, got.Spec.Deny.AppLabels, got.Spec.Deny.AppLabelsExpression)
		}
	})
}

// TestDowngradeV8_ConservativeOverDeny pins the accepted over-deny in the
// AND-to-OR downgrade. A v9 role that allows apps matching a wildcard label set
// narrowed by an expression grants only the apps the expression admits.
// Downgrading moves the wildcard labels and the expression to the deny side
// separately, and a deny rule is greedy, so the wildcard labels alone deny
// every app, including apps outside the allow set. This is intentional: the
// downgrade fails closed and never grants access. The roles.mdx mixed-version
// note documents it for pre-v19 agents.
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
	got := auth.MaybeDowngradeRoleVersionToV8(context.Background(), role, semver.New("18.1.2"))
	denyMatchers := types.LabelMatchers{Labels: got.Spec.Deny.AppLabels, Expression: got.Spec.Deny.AppLabelsExpression}

	assertDenied := func(t *testing.T, labels map[string]string) {
		t.Helper()
		app, err := types.NewAppV3(types.Metadata{Name: "app", Labels: labels}, types.AppSpecV3{URI: "http://localhost"})
		require.NoError(t, err)
		denied, _, err := services.CheckLabelsMatch(types.Deny, denyMatchers, "", nil, app, false)
		require.NoError(t, err)
		require.True(t, denied)
	}

	// An app the role allowed is denied, so the downgrade fails closed on its
	// own apps.
	assertDenied(t, map[string]string{"env": "prod"})
	// An app the role never allowed is also denied, because the wildcard labels
	// on the deny side match it. This is the accepted conservative over-deny.
	assertDenied(t, map[string]string{"env": "dev"})
}
