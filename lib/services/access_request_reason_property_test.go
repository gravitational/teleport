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

const (
	// reasonPropertyClaim is the trait the generated claims_to_roles mappings
	// read from.
	reasonPropertyClaim = "requestable_roles"
	// reasonPropertyRequester is the name of the single requester role the
	// generated user holds.
	reasonPropertyRequester = "test-requester"
	// reasonPropertyUser is the name of the generated user.
	reasonPropertyUser = "test-user"
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

// rolesSpelling is one way of expressing a set of requestable roles in
// spec.allow.request, together with the user traits it needs to resolve.
type rolesSpelling struct {
	roles         []string
	claimsToRoles []types.ClaimMapping
	traits        trait.Traits
}

func (s rolesSpelling) String() string {
	var parts []string
	if len(s.roles) > 0 {
		parts = append(parts, "roles: ["+strings.Join(s.roles, " ")+"]")
	}
	for _, cm := range s.claimsToRoles {
		parts = append(parts, "claims_to_roles: {claim: "+cm.Claim+", value: "+cm.Value+
			", roles: ["+strings.Join(cm.Roles, " ")+"]}")
	}
	return strings.Join(parts, ", ")
}

// conditions renders the spelling as access request conditions carrying the
// given reason configuration.
func (s rolesSpelling) conditions(reason *types.AccessRequestConditionsReason) *types.AccessRequestConditions {
	return &types.AccessRequestConditions{
		Roles:         s.roles,
		ClaimsToRoles: s.claimsToRoles,
		Reason:        reason,
	}
}

// drawRolesSpelling draws a way of naming some subset of requestableRoleNames:
// literally, with a glob or wildcard, or indirectly through claims_to_roles.
func drawRolesSpelling(t *rapid.T) rolesSpelling {
	claimValues := rapid.SliceOfNDistinct(rapid.SampledFrom(requestableRoleNames), 1, 3, func(s string) string { return s }).
		Draw(t, "claim_values")
	traits := trait.Traits{
		"logins":            []string{"abcd"},
		reasonPropertyClaim: claimValues,
	}

	switch rapid.IntRange(0, 5).Draw(t, "spelling_kind") {
	case 0:
		return rolesSpelling{
			roles:  []string{rapid.SampledFrom(requestableRoleNames).Draw(t, "literal")},
			traits: traits,
		}
	case 1:
		return rolesSpelling{
			roles:  []string{rapid.SampledFrom([]string{"db", "k8s", "app"}).Draw(t, "prefix") + "-*"},
			traits: traits,
		}
	case 2:
		return rolesSpelling{
			roles:  []string{"*-" + rapid.SampledFrom([]string{"admin", "reader", "writer"}).Draw(t, "suffix")},
			traits: traits,
		}
	case 3:
		return rolesSpelling{roles: []string{types.Wildcard}, traits: traits}
	case 4:
		// The reported shape: every value of the claim names a requestable role.
		return rolesSpelling{
			claimsToRoles: []types.ClaimMapping{{
				Claim: reasonPropertyClaim,
				Value: "^(.*)$",
				Roles: []string{"$1"},
			}},
			traits: traits,
		}
	default:
		// A claim mapping with no regex and no wildcard anywhere.
		return rolesSpelling{
			claimsToRoles: []types.ClaimMapping{{
				Claim: reasonPropertyClaim,
				Value: claimValues[0],
				Roles: []string{claimValues[0]},
			}},
			traits: traits,
		}
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

// newReasonPropertyGetter builds a getter holding requestableRoleNames, a single
// requester role carrying the given conditions, and a user holding that role.
func newReasonPropertyGetter(t require.TestingT, conditions *types.AccessRequestConditions, traits trait.Traits) *mockGetter {
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
	requesterRole, err := types.NewRole(reasonPropertyRequester, types.RoleSpecV6{
		Allow: types.RoleConditions{Request: conditions},
	})
	require.NoError(t, err)
	g.roles[reasonPropertyRequester] = requesterRole

	uls, err := userloginstate.New(header.Metadata{Name: reasonPropertyUser}, userloginstate.Spec{
		Roles:  []string{reasonPropertyRequester},
		Traits: traits,
	})
	require.NoError(t, err)
	g.userStates[reasonPropertyUser] = uls

	return g
}

// observeReason records what happens when a user whose requester role carries
// the given spelling and reason configuration requests requestedRole without
// supplying a reason.
func observeReason(
	ctx context.Context,
	t require.TestingT,
	spelling rolesSpelling,
	reason *types.AccessRequestConditionsReason,
	requestedRole string,
) reasonOutcome {
	g := newReasonPropertyGetter(t, spelling.conditions(reason), spelling.traits)

	clock := clockwork.NewFakeClock()
	identity := tlsca.Identity{Expires: clock.Now().UTC().Add(8 * time.Hour)}

	newRequest := func() types.AccessRequest {
		req, err := types.NewAccessRequest("some-id", reasonPropertyUser, requestedRole)
		require.NoError(t, err)
		return req
	}

	outcome := reasonOutcome{validateResult: "ok"}

	// The request path: submitting without a reason.
	validator, err := NewRequestValidator(ctx, clock, g, reasonPropertyUser, WithExpandVars(true))
	require.NoError(t, err)
	switch err := validator.validate(ctx, newRequest(), identity); {
	case err == nil:
	case strings.Contains(err.Error(), reasonRequiredMessage):
		outcome.validateResult = "reason-required"
	default:
		outcome.validateResult = err.Error()
	}

	// The dry run path: what clients are told to prompt for.
	dryRunValidator, err := NewRequestValidator(ctx, clock, g, reasonPropertyUser, WithExpandVars(true))
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
// configuration does not depend on how the roles it applies to are named: any
// way of naming a set of roles in spec.allow.request -- a glob, a wildcard, or a
// claims_to_roles mapping -- must behave exactly like listing those same roles
// literally in spec.allow.request.roles.
//
// Listing the matching roles literally is the documented workaround for
// https://github.com/gravitational/teleport/issues/54397, so this property
// failing is that bug.
func TestReasonConfigIsSpellingIndependent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	rapid.Check(t, func(rt *rapid.T) {
		spelling := drawRolesSpelling(rt)
		mode := rapid.SampledFrom([]string{"required", "optional", ""}).Draw(rt, "mode")
		prompt := rapid.SampledFrom([]string{"", "why do you need this?"}).Draw(rt, "prompt")
		requestedRole := rapid.SampledFrom(requestableRoleNames).Draw(rt, "requested_role")

		reason := &types.AccessRequestConditionsReason{
			Mode:   types.RequestReasonMode(mode),
			Prompt: prompt,
		}

		// Ask the validator itself which roles the spelling covers, rather than
		// reimplementing match semantics here. This is the same matcher that
		// decides whether the request is allowed at all.
		validator, err := NewRequestValidator(ctx, clockwork.NewFakeClock(),
			newReasonPropertyGetter(rt, spelling.conditions(reason), spelling.traits),
			reasonPropertyUser, WithExpandVars(true))
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

		// The equivalent config, naming the same roles literally. The traits are
		// carried over so that the two users differ only in the requester role.
		literal := rolesSpelling{roles: matched, traits: spelling.traits}

		require.Equal(rt,
			observeReason(ctx, rt, literal, reason, requestedRole),
			observeReason(ctx, rt, spelling, reason, requestedRole),
			"reason handling for role %q differs between %s and %s",
			requestedRole, spelling, literal,
		)
	})
}
