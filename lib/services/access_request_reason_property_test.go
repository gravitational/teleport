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

package services

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/lib/tlsca"
)

// requestableRoleNames are the concrete roles a generated access request may ask
// for. They share prefixes and suffixes so that the generated patterns below
// match non-trivial subsets of them.
var requestableRoleNames = []string{
	"db-admin", "db-reader", "db-writer",
	"k8s-admin", "k8s-reader",
	"app-admin",
	"admin",
}

// drawRolePattern draws a value for spec.allow.request.roles: either a literal
// role name or a pattern matching some subset of requestableRoleNames.
func drawRolePattern(t *rapid.T) string {
	prefixes := rapid.SampledFrom([]string{"db", "k8s", "app"})
	suffixes := rapid.SampledFrom([]string{"admin", "reader", "writer"})

	switch rapid.IntRange(0, 3).Draw(t, "pattern_kind") {
	case 0:
		return rapid.SampledFrom(requestableRoleNames).Draw(t, "literal")
	case 1:
		return prefixes.Draw(t, "prefix") + "-*"
	case 2:
		return "*-" + suffixes.Draw(t, "suffix")
	default:
		return types.Wildcard
	}
}

// reasonOutcome is the observable effect of the request reason configuration on
// a single access request, as seen by both the request path and by clients doing
// a dry run.
type reasonOutcome struct {
	// validateResult is "ok", "reason-required", or the error the request failed
	// with. The exact reason-required message is normalized away because it names
	// the matched role, which legitimately differs between the two spellings of
	// an equivalent config.
	validateResult string
	dryRunMode     types.RequestReasonMode
	dryRunPrompts  []string
}

const reasonRequiredMessage = "request reason must be specified"

// observeReason builds a user holding a single requester role with the given
// allow.request conditions, and records what happens when that user requests
// requestedRole without a reason.
func observeReason(ctx context.Context, t require.TestingT, conditions *types.AccessRequestConditions, requestedRole string) reasonOutcome {
	const (
		requesterRoleName = "test-requester"
		userName          = "test-user"
	)

	g := &mockGetter{
		roles:       make(map[string]types.Role),
		userStates:  make(map[string]*userloginstate.UserLoginState),
		users:       make(map[string]types.User),
		nodes:       make(map[string]types.Server),
		kubeServers: make(map[string]types.KubeServer),
		dbServers:   make(map[string]types.DatabaseServer),
		appServers:  make(map[string]types.AppServer),
		desktops:    make(map[string]types.WindowsDesktop),
		clusterName: "my-test-cluster",
	}
	for _, name := range requestableRoleNames {
		role, err := types.NewRole(name, types.RoleSpecV6{})
		require.NoError(t, err)
		g.roles[name] = role
	}
	requesterRole, err := types.NewRole(requesterRoleName, types.RoleSpecV6{
		Allow: types.RoleConditions{Request: conditions},
	})
	require.NoError(t, err)
	g.roles[requesterRoleName] = requesterRole

	uls, err := userloginstate.New(header.Metadata{Name: userName}, userloginstate.Spec{
		Roles: []string{requesterRoleName},
		Traits: trait.Traits{
			"logins": []string{"abcd"},
		},
	})
	require.NoError(t, err)
	g.userStates[userName] = uls

	clock := clockwork.NewFakeClock()
	identity := tlsca.Identity{Expires: clock.Now().UTC().Add(8 * time.Hour)}

	newRequest := func() types.AccessRequest {
		req, err := types.NewAccessRequest("some-id", userName, requestedRole)
		require.NoError(t, err)
		return req
	}

	outcome := reasonOutcome{validateResult: "ok"}

	// The request path: submitting without a reason.
	validator, err := NewRequestValidator(ctx, clock, g, userName, WithExpandVars(true))
	require.NoError(t, err)
	switch err := validator.validate(ctx, newRequest(), identity); {
	case err == nil:
	case strings.Contains(err.Error(), reasonRequiredMessage):
		outcome.validateResult = "reason-required"
	default:
		outcome.validateResult = err.Error()
	}

	// The dry run path: what clients are told to prompt for.
	dryRunValidator, err := NewRequestValidator(ctx, clock, g, userName, WithExpandVars(true))
	require.NoError(t, err)
	dryRun := newRequest()
	dryRun.SetDryRun(true)
	if err := dryRunValidator.validate(ctx, dryRun, identity); err == nil {
		if enrichment := dryRun.GetDryRunEnrichment(); enrichment != nil {
			outcome.dryRunMode = enrichment.ReasonMode
			outcome.dryRunPrompts = slices.Clone(enrichment.ReasonPrompts)
			slices.Sort(outcome.dryRunPrompts)
		}
	}

	return outcome
}

// TestReasonConfigIsSpellingIndependent asserts that the request reason
// configuration does not depend on how spec.allow.request.roles is spelled:
// naming a set of roles with a pattern must behave exactly like listing those
// same roles literally.
//
// Listing the matching roles literally is the documented workaround for
// https://github.com/gravitational/teleport/issues/54397, so this property
// failing is that bug.
func TestReasonConfigIsSpellingIndependent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	rapid.Check(t, func(rt *rapid.T) {
		pattern := drawRolePattern(rt)
		mode := rapid.SampledFrom([]string{"required", "optional", ""}).Draw(rt, "mode")
		prompt := rapid.SampledFrom([]string{"", "why do you need this?"}).Draw(rt, "prompt")
		requestedRole := rapid.SampledFrom(requestableRoleNames).Draw(rt, "requested_role")

		reason := &types.AccessRequestConditionsReason{
			Mode:   types.RequestReasonMode(mode),
			Prompt: prompt,
		}

		// Ask the validator itself which roles the pattern covers, rather than
		// reimplementing match semantics here. This is the same matcher that
		// decides whether the request is allowed at all.
		patternConditions := &types.AccessRequestConditions{
			Roles:  []string{pattern},
			Reason: reason,
		}
		validator, err := NewRequestValidator(ctx, clockwork.NewFakeClock(), newReasonPropertyGetter(rt, patternConditions), "test-user", WithExpandVars(true))
		require.NoError(rt, err)

		matched := make([]string, 0, len(requestableRoleNames))
		for _, name := range requestableRoleNames {
			if validator.CanRequestRole(name) {
				matched = append(matched, name)
			}
		}
		if len(matched) == 0 {
			// Nothing to expand; the two configs would be identical.
			return
		}

		// The equivalent config, with the pattern expanded to the roles it
		// matches.
		literalConditions := &types.AccessRequestConditions{
			Roles:  matched,
			Reason: reason,
		}

		require.Equal(rt,
			observeReason(ctx, rt, literalConditions, requestedRole),
			observeReason(ctx, rt, patternConditions, requestedRole),
			"reason handling for role %q differs between roles: [%s] and roles: [%s]",
			requestedRole, pattern, strings.Join(matched, " "),
		)
	})
}

// newReasonPropertyGetter builds a getter holding requestableRoleNames plus a
// single requester role with the given conditions, for constructing a validator
// whose matchers can be queried.
func newReasonPropertyGetter(t require.TestingT, conditions *types.AccessRequestConditions) *mockGetter {
	g := &mockGetter{
		roles:       make(map[string]types.Role),
		userStates:  make(map[string]*userloginstate.UserLoginState),
		users:       make(map[string]types.User),
		nodes:       make(map[string]types.Server),
		kubeServers: make(map[string]types.KubeServer),
		dbServers:   make(map[string]types.DatabaseServer),
		appServers:  make(map[string]types.AppServer),
		desktops:    make(map[string]types.WindowsDesktop),
		clusterName: "my-test-cluster",
	}
	for _, name := range requestableRoleNames {
		role, err := types.NewRole(name, types.RoleSpecV6{})
		require.NoError(t, err)
		g.roles[name] = role
	}
	requesterRole, err := types.NewRole("test-requester", types.RoleSpecV6{
		Allow: types.RoleConditions{Request: conditions},
	})
	require.NoError(t, err)
	g.roles["test-requester"] = requesterRole

	uls, err := userloginstate.New(header.Metadata{Name: "test-user"}, userloginstate.Spec{
		Roles:  []string{"test-requester"},
		Traits: trait.Traits{"logins": []string{"abcd"}},
	})
	require.NoError(t, err)
	g.userStates["test-user"] = uls

	return g
}
