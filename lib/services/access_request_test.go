/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/tlsca"
)

// mockGetter mocks the UserAndRoleGetter interface.
type mockGetter struct {
	userStates  map[string]*userloginstate.UserLoginState
	users       map[string]types.User
	roles       map[string]types.Role
	nodes       map[string]types.Server
	kubeServers map[string]types.KubeServer
	dbServers   map[string]types.DatabaseServer
	appServers  map[string]types.AppServer
	desktops    map[string]types.WindowsDesktop
	clusterName string
}

// user inserts a new user with the specified roles and returns the username.
func (m *mockGetter) user(t *testing.T, roles ...string) string {
	name := uuid.New().String()
	uls, err := userloginstate.New(header.Metadata{
		Name: name,
	}, userloginstate.Spec{
		Roles: roles,
	})
	require.NoError(t, err)

	m.userStates[name] = uls
	return name
}

func (m *mockGetter) GetUserLoginStates(context.Context) ([]*userloginstate.UserLoginState, error) {
	return nil, trace.NotImplemented("GetUserLoginStates is not implemented")
}

func (m *mockGetter) GetUserLoginState(ctx context.Context, name string) (*userloginstate.UserLoginState, error) {
	uls, ok := m.userStates[name]
	if !ok {
		return nil, trace.NotFound("no such user login state: %q", name)
	}
	return uls, nil
}

func (m *mockGetter) GetRole(ctx context.Context, name string) (types.Role, error) {
	role, ok := m.roles[name]
	if !ok {
		return nil, trace.NotFound("no such role: %q", name)
	}
	return role, nil
}

func (m *mockGetter) GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error) {
	if withSecrets {
		return nil, trace.NotImplemented("")
	}

	user, ok := m.users[name]
	if !ok {
		return nil, trace.NotFound("no such user: %q", name)
	}
	return user, nil
}

func (m *mockGetter) GetRoles(ctx context.Context) ([]types.Role, error) {
	roles := make([]types.Role, 0, len(m.roles))
	for _, r := range m.roles {
		roles = append(roles, r)
	}
	return roles, nil
}

// ListResources is a very dumb implementation for the mockGetter that just
// returns all resources which have names matching the request
// PredicateExpression.
func (m *mockGetter) ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	resp := &types.ListResourcesResponse{}
	for nodeName, node := range m.nodes {
		if strings.Contains(req.PredicateExpression, nodeName) {
			resp.Resources = append(resp.Resources, types.ResourceWithLabels(node))
		}
	}
	for kubeName, kubeService := range m.kubeServers {
		if strings.Contains(req.PredicateExpression, kubeName) {
			resp.Resources = append(resp.Resources, types.ResourceWithLabels(kubeService))
		}
	}
	for dbName, dbServer := range m.dbServers {
		if strings.Contains(req.PredicateExpression, dbName) {
			resp.Resources = append(resp.Resources, dbServer)
		}
	}
	for appName, appServer := range m.appServers {
		if strings.Contains(req.PredicateExpression, appName) {
			resp.Resources = append(resp.Resources, appServer)
		}
	}
	for desktopName, desktop := range m.desktops {
		if strings.Contains(req.PredicateExpression, desktopName) {
			resp.Resources = append(resp.Resources, desktop)
		}
	}
	return resp, nil
}

func (m *mockGetter) GetClusterName(opts ...MarshalOption) (types.ClusterName, error) {
	return types.NewClusterName(types.ClusterNameSpecV2{
		ClusterName: m.clusterName,
		ClusterID:   "testid",
	})
}

// TestReviewThresholds tests various review threshold scenarios
func TestReviewThresholds(t *testing.T) {
	ctx := context.Background()

	// describes a collection of roles with various approval/review
	// permissions.
	roleDesc := map[string]types.RoleConditions{
		// dictator is the role that will be requested
		"dictator": {
			// ...
		},
		// populists can achieve dictatorship via uprising or consensus, but only
		// be conclusively denied via uprising.
		"populist": {
			Request: &types.AccessRequestConditions{
				Roles: []string{"dictator"},
				Annotations: map[string][]string{
					"mechanism": {"uprising", "consensus"},
				},
				Thresholds: []types.AccessReviewThreshold{
					{
						Name:    "overwhelming consensus",
						Approve: 3,
					},
					{
						Name:    "popular uprising",
						Filter:  `contains(reviewer.roles,"proletariat")`,
						Approve: 2,
						Deny:    2,
					},
				},
			},
		},
		// generals can achieve dictatorship via coup or consensus, but only
		// be conclusively denied via coup.
		"general": {
			Request: &types.AccessRequestConditions{
				Roles: []string{"dictator"},
				Annotations: map[string][]string{
					"mechanism": {"coup", "consensus"},
				},
				Thresholds: []types.AccessReviewThreshold{
					{
						Name:    "overwhelming consensus",
						Approve: 3,
					},
					{
						Name:    "bloodless coup",
						Filter:  `contains(reviewer.roles,"military")`,
						Approve: 2,
						Deny:    2,
					},
				},
			},
		},
		// conquerors can achieve, or be conclusively denied, dictatorship
		// via treachery.
		"conqueror": {
			Request: &types.AccessRequestConditions{
				Roles: []string{"dictator"},
				Annotations: map[string][]string{
					"mechanism": {"treachery"},
				},
				// no explicit thresholds defaults to single approval/denial
			},
		},
		// rationalists can achieve dictatorship via consensus with a single
		// approval if there is a good reason.
		"rationalist": {
			Request: &types.AccessRequestConditions{
				Roles: []string{"dictator"},
				Annotations: map[string][]string{
					"mechanism": {"consensus"},
				},
				Thresholds: []types.AccessReviewThreshold{
					{
						Name:    "rational consensus",
						Filter:  `regexp.match(request.reason, "*good*") && regexp.match(review.reason, "*good*")`,
						Approve: 1,
					},
				},
			},
		},
		// idealists have an directive for requesting a role which
		// does not exist, and no threshold which permit it to be assumed.
		"idealist": {
			Request: &types.AccessRequestConditions{
				Roles: []string{"never"},
				Thresholds: []types.AccessReviewThreshold{
					{
						Name: "reality check",
						Deny: 1,
					},
				},
			},
		},
		// the proletariat can put dictators into power via uprising
		"proletariat": {
			ReviewRequests: &types.AccessReviewConditions{
				Roles: []string{"dictator"},
				Where: `contains(request.system_annotations["mechanism"],"uprising")`,
			},
		},
		// the intelligentsia can put dictators into power via consensus, with a
		// good reason.
		"intelligentsia": {
			ReviewRequests: &types.AccessReviewConditions{
				Roles: []string{"dictator", "never"},
				Where: `contains(request.system_annotations["mechanism"],"consensus") && regexp.match(request.reason, "*good*")`,
			},
		},
		// the military can put dictators into power via a coup our treachery
		"military": {
			ReviewRequests: &types.AccessReviewConditions{
				Roles: []string{"dictator"},
				Where: `contains(request.system_annotations["mechanism"],"coup") || contains(request.system_annotations["mechanism"],"treachery")`,
			},
		},
		// never is the role that will never be requested
		"never": {
			// ...
		},
	}

	roles := make(map[string]types.Role)

	for name, conditions := range roleDesc {
		role, err := types.NewRole(name, types.RoleSpecV6{
			Allow: conditions,
		})
		require.NoError(t, err)

		roles[name] = role
	}

	// describes a collection of users with various roles
	ulsDesc := map[string][]string{
		"alice": {"populist", "proletariat", "intelligentsia", "military"},
		"carol": {"conqueror", "proletariat", "intelligentsia", "military"},
		"erika": {"populist", "idealist"},
	}

	userStates := make(map[string]*userloginstate.UserLoginState)
	for name, roles := range ulsDesc {
		uls, err := userloginstate.New(header.Metadata{
			Name: name,
		}, userloginstate.Spec{
			Roles: roles,
		})
		require.NoError(t, err)
		userStates[name] = uls
	}

	users := make(map[string]types.User)
	userDesc := map[string][]string{
		"bob":   {"general", "proletariat", "intelligentsia", "military"},
		"dave":  {"populist", "general", "conqueror"},
		"frank": {"rationalist"},
	}

	for name, roles := range userDesc {
		user, err := types.NewUser(name)
		require.NoError(t, err)

		user.SetRoles(roles)
		users[name] = user
	}

	g := &mockGetter{
		roles:      roles,
		userStates: userStates,
		users:      users,
	}

	const (
		pending = types.RequestState_PENDING
		approve = types.RequestState_APPROVED
		deny    = types.RequestState_DENIED
		promote = types.RequestState_PROMOTED
	)

	type review struct {
		// author is the name of the review author
		author string
		// noReview indicates that author will not be allowed to review
		noReview bool
		// propose is the state proposed by the review
		propose types.RequestState
		// expect is the expected post-review state of the request (defaults to pending)
		expect types.RequestState
		// assumeStartTime to apply to review
		assumeStartTime time.Time
		// reason for the review
		reason string

		errCheck require.ErrorAssertionFunc
	}

	clock := clockwork.NewFakeClock()
	tts := []struct {
		// desc is a short description of the test scenario (should be unique)
		desc string
		// requestor is the name of the requesting user
		requestor string
		// the roles to be requested (defaults to "dictator")
		roles []string
		// the reason for the request
		reason  string
		reviews []review
		expiry  time.Time
	}{
		{
			desc:      "populist approval via multi-threshold match",
			requestor: "alice", // permitted by role populist
			reason:    "some very good reason",
			reviews: []review{
				{ // cannot review own requests
					author:   "alice",
					noReview: true,
				},
				{ // no matching allow directives
					author:   g.user(t, "military"),
					noReview: true,
				},
				{ // adds one approval to all thresholds
					author:  g.user(t, "proletariat", "intelligentsia", "military"),
					propose: approve,
				},
				{ // adds one denial to all thresholds
					author:  g.user(t, "proletariat", "intelligentsia", "military"),
					propose: deny,
				},
				{ // adds second approval to all thresholds, triggers "uprising".
					author:  g.user(t, "proletariat", "intelligentsia", "military"),
					propose: approve,
					expect:  approve,
				},
			},
		},
		{
			desc:      "trying to deny an already approved request",
			requestor: "alice", // permitted by role populist
			reason:    "some very good reason",
			reviews: []review{
				{ // cannot review own requests
					author:   "alice",
					noReview: true,
				},
				{ // no matching allow directives
					author:   g.user(t, "military"),
					noReview: true,
				},
				{ // adds one approval to all thresholds
					author:  g.user(t, "proletariat", "intelligentsia", "military"),
					propose: approve,
				},
				{ // adds one denial to all thresholds
					author:  g.user(t, "proletariat", "intelligentsia", "military"),
					propose: deny,
				},
				{ // adds second approval to all thresholds, triggers "uprising".
					author:  g.user(t, "proletariat", "intelligentsia", "military"),
					propose: approve,
					expect:  approve,
				},
				{ // adds second denial but request was already approved.
					author:  g.user(t, "proletariat", "intelligentsia", "military"),
					propose: deny,
					errCheck: func(tt require.TestingT, err error, i ...interface{}) {
						require.ErrorIs(tt, err, trace.AccessDenied("the access request has been already approved"), i...)
					},
				},
			},
		},
		{
			desc:      "trying to approve an already denied request",
			requestor: "bob", // permitted by role general
			reviews: []review{
				{ // 1 of 2 required denials
					author:  g.user(t, "military"),
					propose: deny,
				},
				{ // 2 of 2 required denials
					author:  g.user(t, "military"),
					propose: deny,
					expect:  deny,
				},
				{ // tries to approve but it was already denied
					author:  g.user(t, "military"),
					propose: approve,
					errCheck: func(tt require.TestingT, err error, i ...interface{}) {
						require.ErrorIs(tt, err, trace.AccessDenied("the access request has been already denied"), i...)
					},
				},
			},
		},
		{
			desc:      "intelligentsia cannot review without reason",
			requestor: "alice",
			reviews: []review{
				{
					author:   g.user(t, "intelligentsia"),
					propose:  approve,
					noReview: true,
				},
			},
		},
		{
			desc:      "populist approval via consensus threshold",
			requestor: "alice", // permitted by role populist
			reason:    "some very good reason",
			reviews: []review{
				{ // matches "consensus" threshold
					author:  g.user(t, "intelligentsia"),
					propose: approve,
				},
				{ // matches "uprising" and "consensus" thresholds
					author:  g.user(t, "proletariat"),
					propose: approve,
				},
				{ // 1 of 2 required denials (does not trigger a threshold)
					author:  g.user(t, "proletariat"),
					propose: deny,
				},
				{ // review is permitted but has no effect since "consensus" has no denial condition
					author:  g.user(t, "intelligentsia"),
					propose: deny,
				},
				{ // triggers the "consensus" approval threshold
					author:  g.user(t, "intelligentsia"),
					propose: approve,
					expect:  approve,
				},
			},
		},
		{
			desc:      "general denial via coup threshold",
			requestor: "bob", // permitted by role general
			reason:    "some very good reason",
			reviews: []review{
				{ // cannot review own requests
					author:   "bob",
					noReview: true,
				},
				{ // matches "consensus" threshold
					author:  g.user(t, "intelligentsia"),
					propose: approve,
				},
				{ // matches "coup" threshold
					author:  g.user(t, "military"),
					propose: approve,
				},
				{ // no matching allow directives
					author:   g.user(t, "proletariat"),
					noReview: true,
				},
				{ // 1 of 2 required denials for "coup" (does not trigger a threshold)
					author:  g.user(t, "military"),
					propose: deny,
				},
				{ // review is permitted but matches no thresholds
					author:  g.user(t, "intelligentsia"),
					propose: deny,
				},
				{ // tirggers the "coup" denial threshold
					author:  g.user(t, "military"),
					propose: deny,
					expect:  deny,
				},
			},
		},
		{
			desc:      "conqueror approval via default threshold",
			requestor: "carol", // permitted by role conqueror
			reason:    "some very good reason",
			reviews: []review{
				{ // cannot review own requests
					author:   "carol",
					noReview: true,
				},
				{ // no matching allow directives
					author:   g.user(t, "proletariat"),
					noReview: true,
				},
				{ // no matching allow directives
					author:   g.user(t, "intelligentsia"),
					noReview: true,
				},
				{ // triggers "default" threshold for immediate approval
					author:  g.user(t, "military"),
					propose: approve,
					expect:  approve,
				},
			},
		},
		{
			desc:      "conqueror denial via default threshold",
			requestor: "carol", // permitted by role conqueror
			reviews: []review{
				{ // triggers "default" threshold for immediate denial
					author:  g.user(t, "military"),
					propose: deny,
					expect:  deny,
				},
			},
		},
		{
			// this test case covers a scenario where multiple roles contributed
			// to requestor's permissions.  Current behavior is to require one threshold
			// to pass from each contributing role, but future iterations may become
			// "smart" about which thresholds really need to pass.
			desc:      "multi-role requestor approval via separate threshold matches",
			requestor: "dave", // permitted by conqueror, general, *and* populist
			reason:    "some very good reason",
			reviews: []review{
				{ // matches "default", "coup", and "consensus" thresholds
					author:  g.user(t, "military"),
					propose: approve,
				},

				{ // matches "default", "uprising", and "consensus" thresholds
					author:  g.user(t, "proletariat"),
					propose: approve,
				},
				{ // matches "default" and "consensus" thresholds.
					author:  g.user(t, "intelligentsia"),
					propose: approve,
					expect:  approve,
				},
			},
		},
		{
			// this test case covers a scenario where multiple roles contributed
			// to requestor's permissions.  since the reviewing users hold multiple
			// roles which match various threshold filters, we reach approval with
			// the minimum number reviews necessary s.t. one threshold passes from each
			// requestor role.
			desc:      "multi-role requestor approval via multi-threshold match",
			requestor: "dave", // permitted by conqueror, general, *and* populist
			reviews: []review{
				{ // matches all thresholds
					author:  g.user(t, "military", "proletariat"),
					propose: approve,
				},

				{ // matches all thresholds
					author:  g.user(t, "military", "proletariat"),
					propose: approve,
					expect:  approve,
				},
			},
		},
		{
			// this test case covers a scenario where multiple roles contributed
			// to requestor's permissions.  the interaction here is a bit unintuitive.
			// review *submission* is allowed because of the system annotations applied
			// by "general"/"populist", *but* we end up triggering a threshold which originates
			// from "conqueror".  this may be unexpected behavior for some people since it
			// effectively allows a third role to open up the ability for the reviewer to
			// trigger a threshold which they would not otherwise be able to trigger.  while
			// unintuitive, this effect only manifests as teleport being overly-eager
			// about denying requests (i.e. erring on the side of caution).
			desc:      "multi-role requestor denial via short-circuit on default",
			requestor: "dave", // permitted by conqueror, general, *and* populist
			reason:    "some very good reason",
			reviews: []review{
				{ // ...
					author:  g.user(t, "intelligentsia"),
					propose: deny,
					expect:  deny,
				},
			},
		},
		{
			// this test case is just a sanity-check to make sure that the next
			// case is testing what we think its testing (that thresholds associated
			// with unrequested roles are not added to the request).
			desc:      "threshold omission related sanity-check",
			requestor: "erika", // permitted by combination of populist and idealist
			roles:     []string{"dictator", "never"},
			reason:    "some very good reason",
			reviews: []review{
				{ // matches default threshold from idealist
					author:  g.user(t, "intelligentsia"),
					propose: deny,
					expect:  deny,
				},
			},
		},
		{
			// this test case verifies that thresholds associated with unrelated roles
			// are not added to an access request.  erika holds a role with the default
			// threshold (idealist) but she is only requesting dictator (which does not
			// match any of the allow directives on idealist) so that threshold should
			// be omitted.
			desc:      "threshold omission check",
			requestor: "erika", // permitted by populist, but also holds idealist
			reason:    "some very good reason",
			reviews: []review{
				{ // review is permitted but matches no thresholds
					author:  g.user(t, "intelligentsia"),
					propose: deny,
				},
				{ // matches "consensus" threshold
					author:  g.user(t, "intelligentsia"),
					propose: approve,
				},
				{ // matches "consensus" and "uprising" thresholds
					author:  g.user(t, "proletariat"),
					propose: deny,
				},
				{ // matches "consensus" threshold
					author:  g.user(t, "proletariat"),
					propose: approve,
				},
			},
		},
		{
			desc:      "promoted skips the threshold check",
			requestor: "bob",
			reason:    "some very good reason",
			reviews: []review{
				{ // status should be set to promoted despite the approval threshold not being met
					author:  g.user(t, "intelligentsia"),
					propose: promote,
					expect:  promote,
				},
			},
		},
		{
			desc:      "trying to approve a request with assumeStartTime past expiry",
			requestor: "bob", // permitted by role general
			expiry:    clock.Now().UTC().Add(8 * time.Hour),
			reviews: []review{
				{ // 1 of 2 required approvals
					author:  g.user(t, "military"),
					propose: deny,
				},
				{ // tries to approve but assumeStartTime is after expiry
					author:          g.user(t, "military"),
					propose:         approve,
					assumeStartTime: clock.Now().UTC().Add(10000 * time.Hour),
					errCheck: func(tt require.TestingT, err error, i ...interface{}) {
						require.ErrorContains(tt, err, "assume start time must be prior to access expiry time", i...)
					},
				},
			},
		},
		{
			desc:      "rationalist approval with valid reasons",
			requestor: "frank",
			roles:     []string{"dictator"},
			reason:    "some very good reason",
			reviews: []review{
				{
					author:  g.user(t, "intelligentsia"),
					propose: approve,
					reason:  "frank is just okay",
					expect:  pending,
				},
				{
					author:  g.user(t, "intelligentsia"),
					propose: approve,
					reason:  "frank is pretty good",
					expect:  approve,
				},
			},
		},
	}

	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			if len(tt.roles) == 0 {
				tt.roles = []string{"dictator"}
			}

			// create a request for the specified author
			req, err := types.NewAccessRequest("some-id", tt.requestor, tt.roles...)
			require.NoError(t, err, "scenario=%q", tt.desc)
			req.SetRequestReason(tt.reason)

			clock := clockwork.NewFakeClock()
			identity := tlsca.Identity{
				Expires: clock.Now().UTC().Add(8 * time.Hour),
			}

			if !tt.expiry.IsZero() {
				req.SetExpiry(tt.expiry)
			}

			// perform request validation (necessary in order to initialize internal
			// request variables like annotations and thresholds).
			validator, err := NewRequestValidator(ctx, clock, g, tt.requestor, ExpandVars(true))
			require.NoError(t, err, "scenario=%q", tt.desc)

			require.NoError(t, validator.Validate(ctx, req, identity), "scenario=%q", tt.desc)

		Inner:
			for ri, rt := range tt.reviews {
				if rt.expect.IsNone() {
					rt.expect = types.RequestState_PENDING
				}

				checker, err := NewReviewPermissionChecker(ctx, g, rt.author, nil)
				require.NoError(t, err, "scenario=%q, rev=%d", tt.desc, ri)

				canReview, err := checker.CanReviewRequest(req)
				require.NoError(t, err, "scenario=%q, rev=%d", tt.desc, ri)

				if rt.noReview {
					require.False(t, canReview, "scenario=%q, rev=%d", tt.desc, ri)
					continue Inner
				} else {
					require.True(t, canReview, "scenario=%q, rev=%d", tt.desc, ri)
				}

				rev := types.AccessReview{
					Author:          rt.author,
					ProposedState:   rt.propose,
					Reason:          rt.reason,
					AssumeStartTime: &rt.assumeStartTime,
				}

				author, ok := userStates[rt.author]
				require.True(t, ok, "scenario=%q, rev=%d", tt.desc, ri)

				err = ApplyAccessReview(req, rev, author)
				if rt.errCheck != nil {
					rt.errCheck(t, err)
					continue
				}

				require.NoError(t, err, "scenario=%q, rev=%d", tt.desc, ri)
				require.Equal(t, rt.expect.String(), req.GetState().String(), "scenario=%q, rev=%d", tt.desc, ri)
			}
		})
	}
}

// TestMaxLength tests that we reject too large access requests.
func TestMaxLength(t *testing.T) {
	req, err := types.NewAccessRequest("some-id", "dave", "dictator", "never")
	require.NoError(t, err)

	req.SetRequestReason(strings.Repeat("a", maxAccessRequestReasonSize))
	require.Error(t, ValidateAccessRequest(req))

	resourceIDs, err := types.ResourceIDsFromString(`["/cluster/node/some-node", "/cluster/node/some-other-node" ]`)
	require.NoError(t, err)

	req.SetRequestReason("")
	req.SetRequestedResourceIDs(slices.Repeat(resourceIDs, maxResourcesPerRequest/2+1))
	require.Error(t, ValidateAccessRequest(req))
}

// TestThresholdReviewFilter verifies basic filter syntax.
func TestThresholdReviewFilter(t *testing.T) {
	// test cases consist of a context, and various filter expressions
	// which should or should not match the supplied context.
	tts := []struct {
		ctx       thresholdFilterContext
		willMatch []string
		wontMatch []string
		wontParse []string
	}{
		{ // test expected matching behavior against a basic example context
			ctx: thresholdFilterContext{
				reviewer: reviewAuthorContext{
					roles: []string{"dev"},
					traits: map[string][]string{
						"teams": {"staging-admin"},
					},
				},
				review: reviewParamsContext{
					reason: "ok",
					annotations: map[string][]string{
						"constraints": {"no-admin"},
					},
				},
				request: reviewRequestContext{
					roles:  []string{"dev"},
					reason: "Ticket 123",
					systemAnnotations: map[string][]string{
						"teams": {"staging-dev"},
					},
				},
			},
			willMatch: []string{
				`contains(reviewer.roles,"dev")`,
				`contains(reviewer.traits["teams"],"staging-admin") && contains(request.system_annotations["teams"],"staging-dev")`,
				`!contains(review.annotations["constraints"],"no-admin") || !contains(request.roles,"admin")`,
				`equals(request.reason,"Ticket 123") && equals(review.reason,"ok")`,
				`contains(reviewer.roles,"admin") || contains(reviewer.roles,"dev")`,
				`!(contains(reviewer.roles,"foo") || contains(reviewer.roles,"bar"))`,
				`regexp.match(request.roles, "^dev(elopers)?$")`,
				`regexp.match(request.reason, "^Ticket [0-9]+.*$") && !equals(review.reason, "")`,
			},
			wontMatch: []string{
				`contains(reviewer.roles, "admin")`,
				`equals(request.reason,review.reason)`,
				`!contains(reviewer.traits["teams"],"staging-admin")`,
				`contains(reviewer.roles,"admin") && contains(reviewer.roles,"dev")`,
			},
		},
		{ // test expected matching behavior against zero values
			willMatch: []string{
				`equals(request.reason,review.reason)`,
				`!contains(reviewer.traits["teams"],"staging-admin")`,
				`!contains(review.annotations["constraints"],"no-admin") || !contains(request.roles,"admin")`,
				`!(contains(reviewer.roles,"foo") || contains(reviewer.roles,"bar"))`,
			},
			wontMatch: []string{
				`contains(reviewer.roles, "admin")`,
				`contains(reviewer.roles,"admin") && contains(reviewer.roles,"dev")`,
				`contains(reviewer.roles,"dev")`,
				`contains(reviewer.traits["teams"],"staging-admin") && contains(request.system_annotations["teams"],"staging-dev")`,
				`equals(request.reason,"plz") && equals(review.reason,"ok")`,
				`contains(reviewer.roles,"admin") || contains(reviewer.roles,"dev")`,
				`regexp.match(request.roles, "^dev(elopers)?$")`,
				`regexp.match(request.reason, "^Ticket [0-9]+.*$") && !equals(review.reason, "")`,
			},
			// confirm that an empty context can be used to catch syntax errors
			wontParse: []string{
				`equals(fully.fake.path,"should-fail")`,
				`fakefunc(reviewer.roles,"some-role")`,
				`equals("too","many","params")`,
				`contains("missing-param")`,
				`contains(reviewer.partially.fake.path,"also fails")`,
				`!`,
				`&& missing-left`,
			},
		},
	}

	for _, tt := range tts {
		for _, expr := range tt.willMatch {
			parsed, err := parseThresholdFilterExpression(expr)
			require.NoError(t, err)
			result, err := parsed.Evaluate(tt.ctx)
			require.NoError(t, err)
			require.True(t, result)
		}

		for _, expr := range tt.wontMatch {
			parsed, err := parseThresholdFilterExpression(expr)
			require.NoError(t, err)
			result, err := parsed.Evaluate(tt.ctx)
			require.NoError(t, err)
			require.False(t, result)
		}

		for _, expr := range tt.wontParse {
			_, err := parseThresholdFilterExpression(expr)
			require.Error(t, err)
		}
	}
}

// TestAccessRequestMarshaling verifies that marshaling/unmarshaling access requests
// works as expected (failures likely indicate a problem with json schema).
func TestAccessRequestMarshaling(t *testing.T) {
	req1, err := NewAccessRequest("some-user", "role-1", "role-2")
	require.NoError(t, err)

	marshaled, err := MarshalAccessRequest(req1)
	require.NoError(t, err)

	req2, err := UnmarshalAccessRequest(marshaled)
	require.NoError(t, err)

	require.Equal(t, req1, req2)
}

// TestPluginDataExpectations verifies the correct behavior of the `Expect` mapping.
// Update operations which include an `Expect` mapping should not succeed unless
// all expectations match (e.g. `{"foo":"bar","spam":""}` matches the state where
// key `foo` has value `bar` and key `spam` does not exist).
func TestPluginDataExpectations(t *testing.T) {
	const rname = "my-resource"
	const pname = "my-plugin"
	data, err := types.NewPluginData(rname, types.KindAccessRequest)
	require.NoError(t, err)

	// Set two keys, expecting them to be unset.
	err = data.Update(types.PluginDataUpdateParams{
		Kind:     types.KindAccessRequest,
		Resource: rname,
		Plugin:   pname,
		Set: map[string]string{
			"hello": "world",
			"spam":  "eggs",
		},
		Expect: map[string]string{
			"hello": "",
			"spam":  "",
		},
	})
	require.NoError(t, err)

	// Expect a value which does not exist.
	err = data.Update(types.PluginDataUpdateParams{
		Kind:     types.KindAccessRequest,
		Resource: rname,
		Plugin:   pname,
		Set: map[string]string{
			"should": "fail",
		},
		Expect: map[string]string{
			"missing": "key",
		},
	})
	fixtures.AssertCompareFailed(t, err)

	// Expect a value to not exist when it does exist.
	err = data.Update(types.PluginDataUpdateParams{
		Kind:     types.KindAccessRequest,
		Resource: rname,
		Plugin:   pname,
		Set: map[string]string{
			"should": "fail",
		},
		Expect: map[string]string{
			"hello": "world",
			"spam":  "",
		},
	})
	fixtures.AssertCompareFailed(t, err)

	// Expect the correct state, updating one key and removing another.
	err = data.Update(types.PluginDataUpdateParams{
		Kind:     types.KindAccessRequest,
		Resource: rname,
		Plugin:   pname,
		Set: map[string]string{
			"hello": "there",
			"spam":  "",
		},
		Expect: map[string]string{
			"hello": "world",
			"spam":  "eggs",
		},
	})
	require.NoError(t, err)

	// Expect the new updated state.
	err = data.Update(types.PluginDataUpdateParams{
		Kind:     types.KindAccessRequest,
		Resource: rname,
		Plugin:   pname,
		Set: map[string]string{
			"should": "succeed",
		},
		Expect: map[string]string{
			"hello": "there",
			"spam":  "",
		},
	})
	require.NoError(t, err)
}

// TestPluginDataFilterMatching verifies the expected matching behavior for PluginDataFilter
func TestPluginDataFilterMatching(t *testing.T) {
	const rname = "my-resource"
	const pname = "my-plugin"
	data, err := types.NewPluginData(rname, types.KindAccessRequest)
	require.NoError(t, err)

	var f types.PluginDataFilter

	// Filter for a different resource
	f.Resource = "other-resource"
	require.False(t, f.Match(data))

	// Filter for the same resource
	f.Resource = rname
	require.True(t, f.Match(data))

	// Filter for a plugin which does not have data yet
	f.Plugin = pname
	require.False(t, f.Match(data))

	// Add some data
	err = data.Update(types.PluginDataUpdateParams{
		Kind:     types.KindAccessRequest,
		Resource: rname,
		Plugin:   pname,
		Set: map[string]string{
			"spam": "eggs",
		},
	})
	// Filter again now that data exists
	require.NoError(t, err)
	require.True(t, f.Match(data))
}

// TestRequestFilterMatching verifies expected matching behavior for AccessRequestFilter.
func TestRequestFilterMatching(t *testing.T) {
	reqA, err := NewAccessRequest("alice", "role-a")
	require.NoError(t, err)

	reqB, err := NewAccessRequest("bob", "role-b")
	require.NoError(t, err)

	testCases := []struct {
		user   string
		id     string
		matchA bool
		matchB bool
	}{
		{"", "", true, true},
		{"alice", "", true, false},
		{"", reqA.GetName(), true, false},
		{"bob", reqA.GetName(), false, false},
		{"carol", "", false, false},
	}
	for _, tc := range testCases {
		m := types.AccessRequestFilter{
			User: tc.user,
			ID:   tc.id,
		}
		if m.Match(reqA) != tc.matchA {
			t.Errorf("bad filter behavior (a) %+v", tc)
		}
		if m.Match(reqB) != tc.matchB {
			t.Errorf("bad filter behavior (b) %+v", tc)
		}
	}
}

// TestRequestFilterConversion verifies that filters convert to and from
// maps correctly.
func TestRequestFilterConversion(t *testing.T) {
	testCases := []struct {
		f types.AccessRequestFilter
		m map[string]string
	}{
		{
			types.AccessRequestFilter{User: "alice", ID: "foo", State: types.RequestState_PENDING},
			map[string]string{"user": "alice", "id": "foo", "state": "PENDING"},
		},
		{
			types.AccessRequestFilter{User: "bob"},
			map[string]string{"user": "bob"},
		},
		{
			types.AccessRequestFilter{},
			map[string]string{},
		},
	}
	for _, tc := range testCases {
		m := tc.f.IntoMap()
		require.Empty(t, cmp.Diff(m, tc.m))
		var f types.AccessRequestFilter
		err := f.FromMap(tc.m)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(f, tc.f))
	}
	badMaps := []map[string]string{
		{"food": "carrots"},
		{"state": "homesick"},
	}
	for _, m := range badMaps {
		var f types.AccessRequestFilter
		require.Error(t, f.FromMap(m))
	}
}

// TestRolesForResourceRequest tests that the correct roles are automatically
// determined for resource access requests
func TestRolesForResourceRequest(t *testing.T) {
	// set up test roles
	roleDesc := map[string]types.RoleSpecV6{
		"db-admins": {
			Allow: types.RoleConditions{
				NodeLabels: types.Labels{
					"owner": {"db-admins"},
				},
			},
		},
		"db-response-team": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{"db-admins"},
				},
			},
		},
		"deny-db-request": {
			Deny: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles: []string{"db-admins"},
				},
			},
		},
		"deny-db-search": {
			Deny: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{"db-admins"},
				},
			},
		},
		"splunk-admins": {
			Allow: types.RoleConditions{
				NodeLabels: types.Labels{
					"owner": {"splunk-admins"},
				},
			},
		},
		"splunk-response-team": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{"splunk-admins", "splunk-super-admins"},
				},
			},
		},
		// splunk-super-admins is a role that will never be requested
		"splunk-super-admins": {
			// ...
		},
	}
	roles := make(map[string]types.Role)
	for name, spec := range roleDesc {
		role, err := types.NewRole(name, spec)
		require.NoError(t, err)
		roles[name] = role
	}

	resourceIDs := []types.ResourceID{{
		ClusterName: "one",
		Kind:        "node",
		Name:        "some-uuid",
	}}

	testCases := []struct {
		desc                 string
		currentRoles         []string
		requestRoles         []string
		requestResourceIDs   []types.ResourceID
		expectError          error
		expectRequestedRoles []string
	}{
		{
			desc:                 "basic case",
			currentRoles:         []string{"db-response-team"},
			requestResourceIDs:   resourceIDs,
			expectRequestedRoles: []string{"db-admins"},
		},
		{
			desc:               "deny request without resources",
			currentRoles:       []string{"db-response-team"},
			requestRoles:       []string{"db-admins"},
			requestResourceIDs: nil,
			expectError:        trace.BadParameter(`user "test-user" can not request role "db-admins"`),
		},
		{
			desc:               "deny search",
			currentRoles:       []string{"db-response-team", "deny-db-search"},
			requestResourceIDs: resourceIDs,
			expectError:        trace.AccessDenied(`Resource Access Requests require usable "search_as_roles", none found for user "test-user"`),
		},
		{
			desc:               "deny request",
			currentRoles:       []string{"db-response-team", "deny-db-request"},
			requestResourceIDs: resourceIDs,
			expectError:        trace.AccessDenied(`Resource Access Requests require usable "search_as_roles", none found for user "test-user"`),
		},
		{
			desc:                 "multi allowed roles",
			currentRoles:         []string{"db-response-team", "splunk-response-team"},
			requestResourceIDs:   resourceIDs,
			expectRequestedRoles: []string{"db-admins", "splunk-admins", "splunk-super-admins"},
		},
		{
			desc:                 "multi allowed roles with denial",
			currentRoles:         []string{"db-response-team", "splunk-response-team", "deny-db-search"},
			requestResourceIDs:   resourceIDs,
			expectRequestedRoles: []string{"splunk-admins", "splunk-super-admins"},
		},
		{
			desc:                 "explicit roles request",
			currentRoles:         []string{"db-response-team", "splunk-response-team"},
			requestResourceIDs:   resourceIDs,
			requestRoles:         []string{"splunk-admins"},
			expectRequestedRoles: []string{"splunk-admins"},
		},
		{
			desc:               "invalid explicit roles request",
			currentRoles:       []string{"db-response-team"},
			requestResourceIDs: resourceIDs,
			requestRoles:       []string{"splunk-admins"},
			expectError:        trace.BadParameter(`user "test-user" can not request role "splunk-admins"`),
		},
		{
			desc:               "no allowed roles",
			currentRoles:       nil,
			requestResourceIDs: resourceIDs,
			expectError:        trace.AccessDenied(`Resource Access Requests require usable "search_as_roles", none found for user "test-user"`),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			uls, err := userloginstate.New(header.Metadata{
				Name: "test-user",
			}, userloginstate.Spec{
				Roles: tc.currentRoles,
			})
			require.NoError(t, err)
			userStates := map[string]*userloginstate.UserLoginState{
				uls.GetName(): uls,
			}

			g := &mockGetter{
				roles:       roles,
				userStates:  userStates,
				clusterName: "my-cluster",
			}

			req, err := types.NewAccessRequestWithResources(
				"some-id", uls.GetName(), tc.requestRoles, tc.requestResourceIDs)
			require.NoError(t, err)

			clock := clockwork.NewFakeClock()
			identity := tlsca.Identity{
				Expires: clock.Now().UTC().Add(8 * time.Hour),
			}

			validator, err := NewRequestValidator(context.Background(), clock, g, uls.GetName(), ExpandVars(true))
			require.NoError(t, err)

			err = validator.Validate(context.Background(), req, identity)
			require.ErrorIs(t, err, tc.expectError)
			if err != nil {
				return
			}

			require.Equal(t, tc.expectRequestedRoles, req.GetRoles())
		})
	}
}

func TestPruneRequestRoles(t *testing.T) {
	ctx := context.Background()

	clusterName := "my-cluster"

	g := &mockGetter{
		roles:       make(map[string]types.Role),
		userStates:  make(map[string]*userloginstate.UserLoginState),
		users:       make(map[string]types.User),
		nodes:       make(map[string]types.Server),
		kubeServers: make(map[string]types.KubeServer),
		dbServers:   make(map[string]types.DatabaseServer),
		appServers:  make(map[string]types.AppServer),
		desktops:    make(map[string]types.WindowsDesktop),
		clusterName: clusterName,
	}

	// set up test roles
	roleDesc := map[string]types.RoleSpecV6{
		"response-team": {
			// By default has access to nothing, but can request many types of
			// resources.
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{
						"node-admins",
						"node-access",
						"node-team",
						"kube-admins",
						"db-admins",
						"app-admins",
						"windows-admins",
						"empty",
					},
				},
			},
		},
		"node-access": {
			// Grants access with user's own login
			Allow: types.RoleConditions{
				NodeLabels: types.Labels{
					"*": {"*"},
				},
				Logins: []string{"{{internal.logins}}"},
			},
		},
		"node-admins": {
			// Grants root access to specific nodes.
			Allow: types.RoleConditions{
				NodeLabels: types.Labels{
					"owner": {"node-admins"},
				},
				Logins: []string{"{{internal.logins}}", "root"},
			},
		},
		"node-team": {
			// Grants root access to nodes owned by user's team via label
			// expression.
			Allow: types.RoleConditions{
				NodeLabelsExpression: `contains(user.spec.traits["team"], labels["owner"])`,
				Logins:               []string{"{{internal.logins}}", "root"},
			},
		},
		"kube-admins": {
			Allow: types.RoleConditions{
				KubernetesLabels: types.Labels{
					"*": {"*"},
				},
			},
		},
		"db-admins": {
			Allow: types.RoleConditions{
				DatabaseLabels: types.Labels{
					"*": {"*"},
				},
			},
		},
		"app-admins": {
			Allow: types.RoleConditions{
				AppLabels: types.Labels{
					"*": {"*"},
				},
			},
		},
		"windows-admins": {
			Allow: types.RoleConditions{
				WindowsDesktopLabels: types.Labels{
					"*": {"*"},
				},
			},
		},
		"empty": {
			// Grants access to nothing, should never be requested.
		},
	}
	for name, spec := range roleDesc {
		role, err := types.NewRole(name, spec)
		require.NoError(t, err)
		g.roles[name] = role
	}

	user := g.user(t, "response-team")
	g.userStates[user].Spec.Traits = map[string][]string{
		"logins": {"responder"},
		"team":   {"response-team"},
	}

	nodeDesc := []struct {
		name   string
		labels map[string]string
	}{
		{
			name: "admins-node",
			labels: map[string]string{
				"owner": "node-admins",
			},
		},
		{
			name: "admins-node-2",
			labels: map[string]string{
				"owner": "node-admins",
			},
		},
		{
			name: "responders-node",
			labels: map[string]string{
				"owner": "response-team",
			},
		},
		{
			name: "denied-node",
		},
	}
	for _, desc := range nodeDesc {
		node, err := types.NewServerWithLabels(desc.name, types.KindNode, types.ServerSpecV2{}, desc.labels)
		require.NoError(t, err)
		g.nodes[desc.name] = node
	}

	kube, err := types.NewKubernetesClusterV3(types.Metadata{
		Name: "kube",
	},
		types.KubernetesClusterSpecV3{},
	)
	require.NoError(t, err)

	kubeServer, err := types.NewKubernetesServerV3FromCluster(kube, "_", "_")
	require.NoError(t, err)
	g.kubeServers[kube.GetName()] = kubeServer

	db, err := types.NewDatabaseV3(types.Metadata{
		Name: "db",
	}, types.DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "example.com:3000",
	})
	require.NoError(t, err)
	dbServer, err := types.NewDatabaseServerV3(types.Metadata{
		Name: db.GetName(),
	}, types.DatabaseServerSpecV3{
		HostID:   "db-server",
		Hostname: "db-server",
		Database: db,
	})
	require.NoError(t, err)
	g.dbServers[dbServer.GetName()] = dbServer

	app, err := types.NewAppV3(types.Metadata{
		Name: "app",
	}, types.AppSpecV3{
		URI: "example.com:3000",
	})
	require.NoError(t, err)
	appServer, err := types.NewAppServerV3FromApp(app, "app-server", "app-server")
	require.NoError(t, err)
	g.appServers[app.GetName()] = appServer

	desktop, err := types.NewWindowsDesktopV3("windows", nil, types.WindowsDesktopSpecV3{
		Addr: "example.com:3001",
	})
	require.NoError(t, err)
	g.desktops[desktop.GetName()] = desktop

	testCases := []struct {
		desc               string
		requestResourceIDs []types.ResourceID
		loginHint          string
		expectRoles        []string
		expectError        bool
	}{
		{
			desc: "without login hint",
			requestResourceIDs: []types.ResourceID{
				{
					ClusterName: clusterName,
					Kind:        types.KindNode,
					Name:        "admins-node",
				},
			},
			// Without the login hint, all roles granting access will be
			// requested.
			expectRoles: []string{"node-admins", "node-access"},
		},
		{
			desc: "label expression role",
			requestResourceIDs: []types.ResourceID{
				{
					ClusterName: clusterName,
					Kind:        types.KindNode,
					Name:        "responders-node",
				},
			},
			expectRoles: []string{"node-team", "node-access"},
		},
		{
			desc: "user login hint",
			requestResourceIDs: []types.ResourceID{
				{
					ClusterName: clusterName,
					Kind:        types.KindNode,
					Name:        "admins-node",
				},
			},
			loginHint: "responder",
			// With "responder" login hint, only request node-access.
			expectRoles: []string{"node-access"},
		},
		{
			desc: "multiple nodes",
			requestResourceIDs: []types.ResourceID{
				{
					ClusterName: clusterName,
					Kind:        types.KindNode,
					Name:        "admins-node",
				},
				{
					ClusterName: clusterName,
					Kind:        types.KindNode,
					Name:        "admins-node-2",
				},
			},
			loginHint: "responder",
			// With "responder" login hint, only request node-access.
			expectRoles: []string{"node-access"},
		},
		{
			desc: "root login hint",
			requestResourceIDs: []types.ResourceID{
				{
					ClusterName: clusterName,
					Kind:        types.KindNode,
					Name:        "admins-node",
				},
			},
			loginHint: "root",
			// With "root" login hint, request node-admins.
			expectRoles: []string{"node-admins"},
		},
		{
			desc: "root login unavailable",
			requestResourceIDs: []types.ResourceID{
				{
					ClusterName: clusterName,
					Kind:        types.KindNode,
					Name:        "denied-node",
				},
			},
			loginHint: "root",
			// No roles grant access with the desired login, return an error.
			expectError: true,
		},
		{
			desc: "kube request",
			requestResourceIDs: []types.ResourceID{
				{
					ClusterName: clusterName,
					Kind:        types.KindKubernetesCluster,
					Name:        "kube",
				},
			},
			// Request for kube cluster should only request kube-admins
			expectRoles: []string{"kube-admins"},
		},
		{
			desc: "db request",
			requestResourceIDs: []types.ResourceID{
				{
					ClusterName: clusterName,
					Kind:        types.KindDatabase,
					Name:        "db",
				},
			},
			// Request for db should only request db-admins
			expectRoles: []string{"db-admins"},
		},
		{
			desc: "app request",
			requestResourceIDs: []types.ResourceID{
				{
					ClusterName: clusterName,
					Kind:        types.KindApp,
					Name:        "app",
				},
			},
			// Request for app should only request app-admins
			expectRoles: []string{"app-admins"},
		},
		{
			desc: "windows request",
			requestResourceIDs: []types.ResourceID{
				{
					ClusterName: clusterName,
					Kind:        types.KindWindowsDesktop,
					Name:        "windows",
				},
			},
			// Request for windows should only request windows-admins
			expectRoles: []string{"windows-admins"},
		},
		{
			desc: "mixed request",
			requestResourceIDs: []types.ResourceID{
				{
					ClusterName: clusterName,
					Kind:        types.KindNode,
					Name:        "admins-node",
				},
				{
					ClusterName: clusterName,
					Kind:        types.KindKubernetesCluster,
					Name:        "kube",
				},
				{
					ClusterName: clusterName,
					Kind:        types.KindDatabase,
					Name:        "db",
				},
				{
					ClusterName: clusterName,
					Kind:        types.KindApp,
					Name:        "app",
				},
				{
					ClusterName: clusterName,
					Kind:        types.KindWindowsDesktop,
					Name:        "windows",
				},
			},
			// Request for different kinds should request all necessary roles
			expectRoles: []string{"node-access", "node-admins", "kube-admins", "db-admins", "app-admins", "windows-admins"},
		},
		{
			desc: "foreign resource",
			requestResourceIDs: []types.ResourceID{
				{
					ClusterName: "leaf",
					Kind:        types.KindNode,
					Name:        "admins-node",
				},
			},
			// Request for foreign resource should request all available roles,
			// we don't know which one is necessary
			expectRoles: []string{"node-access", "node-admins", "node-team", "kube-admins", "db-admins", "app-admins", "windows-admins", "empty"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			req, err := NewAccessRequestWithResources(user, nil, tc.requestResourceIDs)
			require.NoError(t, err)

			req.SetLoginHint(tc.loginHint)

			clock := clockwork.NewFakeClock()
			identity := tlsca.Identity{
				Expires: clock.Now().UTC().Add(8 * time.Hour),
			}

			accessCaps, err := CalculateAccessCapabilities(ctx, clock, g, tlsca.Identity{}, types.AccessCapabilitiesRequest{User: user, ResourceIDs: tc.requestResourceIDs})
			require.NoError(t, err)

			err = ValidateAccessRequestForUser(ctx, clock, g, req, identity, ExpandVars(true))
			if tc.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			if tc.loginHint == "" {
				require.ElementsMatch(t, tc.expectRoles, accessCaps.ApplicableRolesForResources)
			}

			require.ElementsMatch(t, tc.expectRoles, req.GetRoles(),
				"Pruned roles %v don't match expected roles %v", req.GetRoles(), tc.expectRoles)
			require.Len(t, req.GetRoleThresholdMapping(), len(req.GetRoles()),
				"Length of rtm does not match number of roles. rtm: %v roles %v",
				req.GetRoleThresholdMapping(), req.GetRoles())
		})
	}
}

func TestGetRequestableRoles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	clusterName := "my-cluster"

	g := &mockGetter{
		roles:       make(map[string]types.Role),
		userStates:  make(map[string]*userloginstate.UserLoginState),
		nodes:       make(map[string]types.Server),
		clusterName: clusterName,
	}

	for i := 0; i < 10; i++ {
		node, err := types.NewServerWithLabels(
			fmt.Sprintf("node-%d", i),
			types.KindNode,
			types.ServerSpecV2{},
			map[string]string{"index": strconv.Itoa(i)})
		require.NoError(t, err)
		g.nodes[node.GetName()] = node
	}

	getResourceID := func(i int) types.ResourceID {
		return types.ResourceID{
			ClusterName: clusterName,
			Kind:        types.KindNode,
			Name:        fmt.Sprintf("node-%d", i),
		}
	}

	roleDesc := map[string]types.RoleSpecV6{
		"partial-access": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles:         []string{"full-access", "full-search"},
					SearchAsRoles: []string{"full-access"},
				},
				NodeLabels: types.Labels{
					"index": {"0", "1", "2", "3", "4"},
				},
				Logins: []string{"{{internal.logins}}"},
			},
		},
		"full-access": {
			Allow: types.RoleConditions{
				NodeLabels: types.Labels{
					"index": {"*"},
				},
				Logins: []string{"{{internal.logins}}"},
			},
		},
		"full-search": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles:         []string{"partial-access", "full-access"},
					SearchAsRoles: []string{"full-access"},
				},
			},
		},
		"partial-search": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles:         []string{"partial-access", "full-access"},
					SearchAsRoles: []string{"partial-access"},
				},
			},
		},
		"partial-roles": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles:         []string{"partial-access"},
					SearchAsRoles: []string{"full-access"},
				},
			},
		},
	}

	for name, spec := range roleDesc {
		role, err := types.NewRole(name, spec)
		require.NoError(t, err)
		g.roles[name] = role
	}

	user := g.user(t)

	tests := []struct {
		name               string
		userRole           string
		requestedResources []types.ResourceID
		disableFilter      bool
		allowedResourceIDs []types.ResourceID
		expectedRoles      []string
	}{
		{
			name:          "no resources to filter by",
			userRole:      "full-search",
			expectedRoles: []string{"partial-access", "full-access"},
		},
		{
			name:               "filtering disabled",
			userRole:           "full-search",
			requestedResources: []types.ResourceID{getResourceID(9)},
			disableFilter:      true,
			expectedRoles:      []string{"partial-access", "full-access"},
		},
		{
			name:               "filter by resources",
			userRole:           "full-search",
			requestedResources: []types.ResourceID{getResourceID(9)},
			expectedRoles:      []string{"full-access"},
		},
		{
			name:     "resource in another cluster",
			userRole: "full-search",
			requestedResources: []types.ResourceID{
				getResourceID(9),
				{
					ClusterName: "some-other-cluster",
					Kind:        types.KindNode,
					Name:        "node-9",
				},
			},
			expectedRoles: []string{"partial-access", "full-access"},
		},
		{
			name:               "resource user shouldn't know about",
			userRole:           "partial-search",
			requestedResources: []types.ResourceID{getResourceID(9)},
			expectedRoles:      []string{"partial-access", "full-access"},
		},
		{
			name:               "can view resource but not assume role",
			userRole:           "partial-roles",
			requestedResources: []types.ResourceID{getResourceID(9)},
		},
		{
			name:               "prevent transitive access",
			userRole:           "partial-access",
			requestedResources: []types.ResourceID{getResourceID(9)},
			allowedResourceIDs: []types.ResourceID{getResourceID(0), getResourceID(1), getResourceID(2), getResourceID(3), getResourceID(4)},
			expectedRoles:      []string{"full-access", "full-search"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g.userStates[user].Spec.Roles = []string{tc.userRole}
			accessCaps, err := CalculateAccessCapabilities(ctx, clockwork.NewFakeClock(), g,
				tlsca.Identity{
					AllowedResourceIDs: tc.allowedResourceIDs,
				},
				types.AccessCapabilitiesRequest{
					User:                             user,
					RequestableRoles:                 true,
					ResourceIDs:                      tc.requestedResources,
					FilterRequestableRolesByResource: !tc.disableFilter,
				})
			require.NoError(t, err)
			require.ElementsMatch(t, tc.expectedRoles, accessCaps.RequestableRoles)
		})
	}
}

// TestCalculatePendingRequestTTL verifies that the TTL for the Access Request is capped to the
// request's access expiry or capped to the default const requestTTL, whichever is smaller.
func TestCalculatePendingRequestTTL(t *testing.T) {
	clock := clockwork.NewFakeClock()
	now := clock.Now().UTC()

	tests := []struct {
		desc string
		// accessExpiryTTL == max access duration.
		accessExpiryTTL time.Duration
		// when the access request expires in the PENDING state.
		requestPendingExpiryTTL time.Time
		assertion               require.ErrorAssertionFunc
		expectedDuration        time.Duration
	}{
		{
			desc:                    "valid: requested ttl < access expiry",
			accessExpiryTTL:         requestTTL - (3 * day),
			requestPendingExpiryTTL: now.Add(requestTTL - (4 * day)),
			expectedDuration:        requestTTL - (4 * day),
			assertion:               require.NoError,
		},
		{
			desc:                    "valid: requested ttl == access expiry",
			accessExpiryTTL:         requestTTL - (3 * day),
			requestPendingExpiryTTL: now.Add(requestTTL - (3 * day)),
			expectedDuration:        requestTTL - (3 * day),
			assertion:               require.NoError,
		},
		{
			desc:                    "valid: requested ttl == default request ttl",
			accessExpiryTTL:         requestTTL,
			requestPendingExpiryTTL: now.Add(requestTTL),
			expectedDuration:        requestTTL,
			assertion:               require.NoError,
		},
		{
			desc:             "valid: no TTL request defaults to the const requestTTL if access expiry is larger",
			accessExpiryTTL:  requestTTL + (3 * day),
			expectedDuration: requestTTL,
			assertion:        require.NoError,
		},
		{
			desc:             "valid: no TTL request defaults to accessExpiry if const requestTTL is larger",
			accessExpiryTTL:  requestTTL - (3 * day),
			expectedDuration: requestTTL - (3 * day),
			assertion:        require.NoError,
		},
		{
			desc:                    "invalid: requested ttl > access expiry",
			accessExpiryTTL:         requestTTL - (3 * day),
			requestPendingExpiryTTL: now.Add(requestTTL - (2 * day)),
			assertion:               require.Error,
		},
		{
			desc:                    "invalid: requested ttl > default request TTL",
			accessExpiryTTL:         requestTTL + (1 * day),
			requestPendingExpiryTTL: now.Add(requestTTL + (1 * day)),
			assertion:               require.Error,
		},
		{
			desc:                    "invalid: requested ttl < now",
			accessExpiryTTL:         requestTTL - (3 * day),
			requestPendingExpiryTTL: now.Add(-(3 * day)),
			assertion:               require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// Setup test user "foo" and "bar" and the mock auth server that
			// will return users and roles.
			uls, err := userloginstate.New(header.Metadata{
				Name: "foo",
			}, userloginstate.Spec{
				Roles: []string{"bar"},
			})
			require.NoError(t, err)

			role, err := types.NewRole("bar", types.RoleSpecV6{})
			require.NoError(t, err)

			getter := &mockGetter{
				userStates: map[string]*userloginstate.UserLoginState{"foo": uls},
				roles:      map[string]types.Role{"bar": role},
			}

			validator, err := NewRequestValidator(context.Background(), clock, getter, "foo", ExpandVars(true))
			require.NoError(t, err)

			request, err := types.NewAccessRequest("some-id", "foo", "bar")
			require.NoError(t, err)
			request.SetExpiry(tt.requestPendingExpiryTTL)
			request.SetAccessExpiry(now.Add(tt.accessExpiryTTL))

			ttl, err := validator.calculatePendingRequestTTL(request, now)
			tt.assertion(t, err)
			if err == nil {
				require.Equal(t, tt.expectedDuration, ttl)
			}
		})
	}
}

// TestSessionTTL verifies that the TTL for elevated access gets reduced by
// requested access time, lifetime of certificate, and strictest session TTL on
// any role.
func TestSessionTTL(t *testing.T) {
	clock := clockwork.NewFakeClock()
	now := clock.Now().UTC()

	tests := []struct {
		desc          string
		accessExpiry  time.Time
		identity      tlsca.Identity
		maxSessionTTL time.Duration
		expectedTTL   time.Duration
		assertion     require.ErrorAssertionFunc
	}{
		{
			desc:          "less than identity expiration and role session ttl allowed",
			accessExpiry:  now.Add(13 * time.Minute),
			identity:      tlsca.Identity{Expires: now.Add(defaults.MaxAccessDuration)},
			maxSessionTTL: defaults.MaxAccessDuration,
			expectedTTL:   13 * time.Minute,
			assertion:     require.NoError,
		},
		{
			desc:          "greater than identity expiration and role session ttl not allowed",
			accessExpiry:  now.Add(14 * time.Minute),
			identity:      tlsca.Identity{Expires: now.Add(13 * time.Minute)},
			maxSessionTTL: 13 * time.Minute,
			assertion:     require.Error,
		},
		{
			desc:          "greater than certificate duration not allowed",
			accessExpiry:  now.Add(defaults.MaxAccessDuration).Add(1 * time.Minute),
			identity:      tlsca.Identity{Expires: now.Add(defaults.MaxAccessDuration)},
			maxSessionTTL: defaults.MaxAccessDuration,
			assertion:     require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// Setup test user "foo" and "bar" and the mock auth server that
			// will return users and roles.
			user, err := types.NewUser("foo")
			require.NoError(t, err)

			role, err := types.NewRole("bar", types.RoleSpecV6{
				Options: types.RoleOptions{
					MaxSessionTTL: types.NewDuration(tt.maxSessionTTL),
				},
			})
			require.NoError(t, err)

			getter := &mockGetter{
				users: map[string]types.User{"foo": user},
				roles: map[string]types.Role{"bar": role},
			}

			validator, err := NewRequestValidator(context.Background(), clock, getter, "foo", ExpandVars(true))
			require.NoError(t, err)

			request, err := types.NewAccessRequest("some-id", "foo", "bar")
			request.SetAccessExpiry(tt.accessExpiry)
			require.NoError(t, err)

			ttl, err := validator.sessionTTL(context.Background(), tt.identity, request, now)
			tt.assertion(t, err)
			if err == nil {
				require.Equal(t, tt.expectedTTL, ttl)
			}
		})
	}
}

func TestAutoRequest(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()

	empty, err := types.NewRole("empty", types.RoleSpecV6{})
	require.NoError(t, err)

	promptRole, err := types.NewRole("prompt", types.RoleSpecV6{
		Options: types.RoleOptions{
			RequestPrompt: "test prompt",
		},
	})
	require.NoError(t, err)

	optionalRole, err := types.NewRole("optional", types.RoleSpecV6{
		Options: types.RoleOptions{
			RequestAccess: types.RequestStrategyOptional,
		},
	})
	require.NoError(t, err)

	reasonRole, err := types.NewRole("reason", types.RoleSpecV6{
		Options: types.RoleOptions{
			RequestAccess: types.RequestStrategyReason,
		},
	})
	require.NoError(t, err)

	alwaysRole, err := types.NewRole("always", types.RoleSpecV6{
		Options: types.RoleOptions{
			RequestAccess: types.RequestStrategyAlways,
		},
	})
	require.NoError(t, err)

	cases := []struct {
		name      string
		roles     []types.Role
		assertion func(t *testing.T, validator *RequestValidator, accessCaps *types.AccessCapabilities)
	}{
		{
			name: "no roles",
			assertion: func(t *testing.T, validator *RequestValidator, accessCaps *types.AccessCapabilities) {
				require.False(t, validator.requireReasonForAllRoles)
				require.False(t, validator.autoRequest)
				require.Empty(t, validator.prompt)

				require.False(t, accessCaps.RequireReason)
				require.False(t, accessCaps.AutoRequest)
				require.Empty(t, accessCaps.RequestPrompt)
			},
		},
		{
			name:  "with prompt",
			roles: []types.Role{empty, optionalRole, promptRole},
			assertion: func(t *testing.T, validator *RequestValidator, accessCaps *types.AccessCapabilities) {
				require.False(t, validator.requireReasonForAllRoles)
				require.False(t, validator.autoRequest)
				require.Equal(t, "test prompt", validator.prompt)

				require.False(t, accessCaps.RequireReason)
				require.False(t, accessCaps.AutoRequest)
				require.Equal(t, "test prompt", accessCaps.RequestPrompt)
			},
		},
		{
			name:  "with auto request",
			roles: []types.Role{alwaysRole},
			assertion: func(t *testing.T, validator *RequestValidator, accessCaps *types.AccessCapabilities) {
				require.False(t, validator.requireReasonForAllRoles)
				require.True(t, validator.autoRequest)
				require.Empty(t, validator.prompt)

				require.False(t, accessCaps.RequireReason)
				require.True(t, accessCaps.AutoRequest)
				require.Empty(t, accessCaps.RequestPrompt)
			},
		},
		{
			name:  "with prompt and auto request",
			roles: []types.Role{promptRole, alwaysRole},
			assertion: func(t *testing.T, validator *RequestValidator, accessCaps *types.AccessCapabilities) {
				require.False(t, validator.requireReasonForAllRoles)
				require.True(t, validator.autoRequest)
				require.Equal(t, "test prompt", validator.prompt)

				require.False(t, accessCaps.RequireReason)
				require.True(t, accessCaps.AutoRequest)
				require.Equal(t, "test prompt", accessCaps.RequestPrompt)
			},
		},
		{
			name:  "with reason and auto prompt",
			roles: []types.Role{reasonRole},
			assertion: func(t *testing.T, validator *RequestValidator, accessCaps *types.AccessCapabilities) {
				require.True(t, validator.requireReasonForAllRoles)
				require.True(t, validator.autoRequest)
				require.Empty(t, validator.prompt)

				require.True(t, accessCaps.RequireReason)
				require.True(t, accessCaps.AutoRequest)
				require.Empty(t, accessCaps.RequestPrompt)
			},
		},
	}

	for _, test := range cases {
		ctx := context.Background()

		uls, err := userloginstate.New(header.Metadata{
			Name: "foo",
		}, userloginstate.Spec{})
		require.NoError(t, err)

		getter := &mockGetter{
			userStates:  make(map[string]*userloginstate.UserLoginState),
			roles:       make(map[string]types.Role),
			clusterName: "test-cluster",
		}

		for _, r := range test.roles {
			getter.roles[r.GetName()] = r
			uls.Spec.Roles = append(uls.Spec.Roles, r.GetName())
		}

		getter.userStates[uls.GetName()] = uls

		validator, err := NewRequestValidator(ctx, clock, getter, uls.GetName(), ExpandVars(true))
		require.NoError(t, err)

		accessCapabilities, err := CalculateAccessCapabilities(ctx, clock, getter, tlsca.Identity{}, types.AccessCapabilitiesRequest{
			User:             uls.GetName(),
			RequestableRoles: true,
		})
		require.NoError(t, err)

		test.assertion(t, &validator, accessCapabilities)
	}

}

func TestReasonRequired(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	clusterName := "my-test-cluster"

	g := &mockGetter{
		roles:       make(map[string]types.Role),
		userStates:  make(map[string]*userloginstate.UserLoginState),
		users:       make(map[string]types.User),
		nodes:       make(map[string]types.Server),
		kubeServers: make(map[string]types.KubeServer),
		dbServers:   make(map[string]types.DatabaseServer),
		appServers:  make(map[string]types.AppServer),
		desktops:    make(map[string]types.WindowsDesktop),
		clusterName: clusterName,
	}

	nodeDesc := []struct {
		name   string
		labels map[string]string
	}{
		{
			name: "fork-node",
			labels: map[string]string{
				"cutlery": "fork",
			},
		},
		{
			name: "spoon-node",
			labels: map[string]string{
				"cutlery": "spoon",
			},
		},
	}
	for _, desc := range nodeDesc {
		node, err := types.NewServerWithLabels(desc.name, types.KindNode, types.ServerSpecV2{}, desc.labels)
		require.NoError(t, err)
		g.nodes[desc.name] = node
	}

	roleDesc := map[string]types.RoleSpecV6{
		"cutlery-access": {
			Allow: types.RoleConditions{
				NodeLabels: types.Labels{
					"cutlery": []string{types.Wildcard},
				},
			},
		},
		"fork-access": {
			Allow: types.RoleConditions{
				NodeLabels: types.Labels{
					"cutlery": []string{"fork"},
				},
			},
		},

		"cutlery-access-requester": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles: []string{"cutlery-access"},
					// "optional" is the default
					// Reason: &types.AccessRequestConditionsReason{
					// 	Mode: "optional",
					// },
				},
			},
		},
		"cutlery-node-requester": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{"cutlery-access"},
					// set "optional" explicitly
					Reason: &types.AccessRequestConditionsReason{
						Mode: "optional",
					},
				},
			},
		},

		"fork-node-requester": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{"fork-access"},
					// everything not-"required" is basically "optional"
					Reason: &types.AccessRequestConditionsReason{
						Mode: "not-recognized-is-optional",
					},
				},
			},
		},
		"fork-node-requester-with-reason": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{"fork-access"},
					Reason: &types.AccessRequestConditionsReason{
						Mode: "required",
					},
				},
			},
		},
		"fork-access-requester": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles: []string{"fork-access"},
				},
			},
		},
		"fork-access-requester-with-reason": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles: []string{"fork-access"},
					Reason: &types.AccessRequestConditionsReason{
						Mode: "required",
					},
				},
			},
		},
	}
	for name, spec := range roleDesc {
		role, err := types.NewRole(name, spec)
		require.NoError(t, err)
		g.roles[name] = role
	}

	testCases := []struct {
		name               string
		currentRoles       []string
		requestRoles       []string
		requestResourceIDs []types.ResourceID
		expectError        error
	}{
		{
			name:         "role request: require reason when role has reason.required",
			currentRoles: []string{"fork-access-requester-with-reason"},
			requestRoles: []string{"fork-access"},
			expectError:  trace.BadParameter(`request reason must be specified (required for role "fork-access")`),
		},
		{
			name:         "resource request: require reason when role has reason.required",
			currentRoles: []string{"fork-node-requester-with-reason"},
			requestResourceIDs: []types.ResourceID{
				{ClusterName: clusterName, Kind: types.KindNode, Name: "fork-node"},
			},
			expectError: trace.BadParameter(`request reason must be specified (required for role "fork-access")`),
		},
		{
			name:         "role request: do not require reason when role does not have reason.required",
			currentRoles: []string{"fork-access-requester"},
			requestRoles: []string{"fork-access"},
			expectError:  nil,
		},
		{
			name:         "resource request: do not require reason when role does not have reason.required",
			currentRoles: []string{"fork-node-requester"},
			requestResourceIDs: []types.ResourceID{
				{ClusterName: clusterName, Kind: types.KindNode, Name: "fork-node"},
			},
			expectError: nil,
		},
		{
			name:         "resource request: but require reason when another role allowing _role_ access requires reason for the role",
			currentRoles: []string{"fork-node-requester", "fork-access-requester-with-reason"},
			requestResourceIDs: []types.ResourceID{
				{ClusterName: clusterName, Kind: types.KindNode, Name: "fork-node"},
			},
			expectError: trace.BadParameter(`request reason must be specified (required for role "fork-access")`),
		},
		{
			name:         "role request: require reason when _any_ role has reason.required",
			currentRoles: []string{"fork-access-requester", "fork-access-requester-with-reason", "cutlery-access-requester"},
			requestRoles: []string{"fork-access"},
			expectError:  trace.BadParameter(`request reason must be specified (required for role "fork-access")`),
		},
		{
			name:         "resource request: require reason when _any_ role has reason.required",
			currentRoles: []string{"fork-node-requester", "fork-node-requester-with-reason", "cutlery-node-requester"},
			requestResourceIDs: []types.ResourceID{
				{ClusterName: clusterName, Kind: types.KindNode, Name: "fork-node"},
			},
			expectError: trace.BadParameter(`request reason must be specified (required for role "fork-access")`),
		},
		{
			name:         "role request: do not require reason when all roles don't have reason.required",
			currentRoles: []string{"fork-access-requester", "cutlery-access-requester"},
			requestRoles: []string{"fork-access"},
			expectError:  nil,
		},
		{
			name:         "resource request: do not require reason when all roles don't have reason.required",
			currentRoles: []string{"fork-node-requester", "cutlery-node-requester"},
			requestResourceIDs: []types.ResourceID{
				{ClusterName: clusterName, Kind: types.KindNode, Name: "fork-node"},
			},
			expectError: nil,
		},
		{
			name:         "role request: require reason when _any_ role with reason.required matches _any_ roles",
			currentRoles: []string{"fork-access-requester-with-reason", "cutlery-access-requester"},
			requestRoles: []string{"fork-access", "cutlery-access"},
			expectError:  trace.BadParameter(`request reason must be specified (required for role "fork-access")`),
		},
		{
			name:         "resource request: require reason when _any_ role with reason.required matches _any_ resource",
			currentRoles: []string{"fork-node-requester-with-reason", "cutlery-node-requester"},
			requestResourceIDs: []types.ResourceID{
				{ClusterName: clusterName, Kind: types.KindNode, Name: "fork-node"},
				{ClusterName: clusterName, Kind: types.KindNode, Name: "spoon-node"},
			},
			expectError: trace.BadParameter(`request reason must be specified (required for role "fork-access")`),
		},
		{
			name:         "role request: do not require reason when _all_ roles do not require reason for _all_ roles",
			currentRoles: []string{"fork-access-requester", "cutlery-access-requester"},
			requestRoles: []string{"fork-access", "cutlery-access"},
			expectError:  nil,
		},
		{
			name:         "role request: handle wildcard",
			currentRoles: []string{"fork-access-requester-with-reason"},
			requestRoles: []string{"*"},
			expectError:  trace.BadParameter(`request reason must be specified (required for role "fork-access")`),
		},
		{
			name:         "resource request: do not require reason when _all_ roles do not require reason for _all_ resources",
			currentRoles: []string{"fork-node-requester", "cutlery-node-requester"},
			requestResourceIDs: []types.ResourceID{
				{ClusterName: clusterName, Kind: types.KindNode, Name: "fork-node"},
				{ClusterName: clusterName, Kind: types.KindNode, Name: "spoon-node"},
			},
			expectError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uls, err := userloginstate.New(header.Metadata{
				Name: "test-user",
			}, userloginstate.Spec{
				Roles: tc.currentRoles,
				Traits: trait.Traits{
					"logins": []string{"abcd"},
				},
			})
			require.NoError(t, err)
			g.userStates[uls.GetName()] = uls

			clock := clockwork.NewFakeClock()
			identity := tlsca.Identity{
				Expires: clock.Now().UTC().Add(8 * time.Hour),
			}

			// test RequestValidator.Validate
			{
				validator, err := NewRequestValidator(ctx, clock, g, uls.GetName(), ExpandVars(true))
				require.NoError(t, err)

				req, err := types.NewAccessRequestWithResources(
					"some-id", uls.GetName(), tc.requestRoles, tc.requestResourceIDs)
				require.NoError(t, err)

				// No reason in the request.
				err = validator.Validate(ctx, req.Copy(), identity)
				require.ErrorIs(t, err, tc.expectError)

				// White-space reason should be treated as no reason.
				req.SetRequestReason("  \t \n  ")
				err = validator.Validate(ctx, req.Copy(), identity)
				require.ErrorIs(t, err, tc.expectError)

				// When non-empty reason is provided then validation should pass.
				req.SetRequestReason("good reason")
				err = validator.Validate(ctx, req.Copy(), identity)
				require.NoError(t, err)
			}

			// test CalculateAccessCapabilities
			{
				req := types.AccessCapabilitiesRequest{
					User:             uls.GetName(),
					ResourceIDs:      tc.requestResourceIDs,
					RequestableRoles: len(tc.requestResourceIDs) == 0,
				}

				res, err := CalculateAccessCapabilities(ctx, clock, g, identity, req)
				require.NoError(t, err)
				if tc.expectError != nil {
					require.True(t, res.RequireReason)
				} else {
					require.False(t, res.RequireReason)
				}
			}
		})
	}
}

type mockClusterGetter struct {
	localCluster   types.ClusterName
	remoteClusters map[string]types.RemoteCluster
}

func (mcg mockClusterGetter) GetClusterName(opts ...MarshalOption) (types.ClusterName, error) {
	return mcg.localCluster, nil
}

func (mcg mockClusterGetter) GetRemoteCluster(ctx context.Context, clusterName string) (types.RemoteCluster, error) {
	if cluster, ok := mcg.remoteClusters[clusterName]; ok {
		return cluster, nil
	}
	return nil, trace.NotFound("remote cluster %q was not found", clusterName)
}

func TestValidateResourceRequestSizeLimits(t *testing.T) {
	g := &mockGetter{
		roles:       make(map[string]types.Role),
		userStates:  make(map[string]*userloginstate.UserLoginState),
		users:       make(map[string]types.User),
		nodes:       make(map[string]types.Server),
		kubeServers: make(map[string]types.KubeServer),
		dbServers:   make(map[string]types.DatabaseServer),
		appServers:  make(map[string]types.AppServer),
		desktops:    make(map[string]types.WindowsDesktop),
		clusterName: "someCluster",
	}

	for i := 1; i < 3; i++ {
		node, err := types.NewServerWithLabels(
			fmt.Sprintf("resource%d", i),
			types.KindNode,
			types.ServerSpecV2{},
			map[string]string{"foo": "bar"},
		)
		require.NoError(t, err)
		g.nodes[node.GetName()] = node
	}

	searchAsRole, err := types.NewRole("searchAs", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins:     []string{"root"},
			NodeLabels: types.Labels{"*": []string{"*"}},
		},
	})
	require.NoError(t, err)
	g.roles[searchAsRole.GetName()] = searchAsRole

	testRole, err := types.NewRole("testRole", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				SearchAsRoles: []string{searchAsRole.GetName()},
			},
		},
	})
	require.NoError(t, err)
	g.roles[testRole.GetName()] = testRole

	user := g.user(t, testRole.GetName())

	clock := clockwork.NewFakeClock()
	identity := tlsca.Identity{
		Expires: clock.Now().UTC().Add(8 * time.Hour),
	}

	req, err := types.NewAccessRequestWithResources("name", user, nil, /* roles */
		[]types.ResourceID{
			{ClusterName: "someCluster", Kind: "node", Name: "resource1"},
			{ClusterName: "someCluster", Kind: "node", Name: "resource1"}, // a  duplicate
			{ClusterName: "someCluster", Kind: "node", Name: "resource2"}, // not a duplicate
		})
	require.NoError(t, err)

	require.NoError(t, ValidateAccessRequestForUser(context.Background(), clock, g, req, identity, ExpandVars(true)))
	require.Len(t, req.GetRequestedResourceIDs(), 2)
	require.Equal(t, "/someCluster/node/resource1", types.ResourceIDToString(req.GetRequestedResourceIDs()[0]))
	require.Equal(t, "/someCluster/node/resource2", types.ResourceIDToString(req.GetRequestedResourceIDs()[1]))

	var requestedResourceIDs []types.ResourceID
	for i := 0; i < 200; i++ {
		requestedResourceIDs = append(requestedResourceIDs, types.ResourceID{
			ClusterName: "someCluster",
			Kind:        "node",
			Name:        "resource" + strconv.Itoa(i),
		})
	}
	req.SetRequestedResourceIDs(requestedResourceIDs)
	require.ErrorContains(t, ValidateAccessRequestForUser(context.Background(), clock, g, req, identity, ExpandVars(true)), "access request exceeds maximum length")
}

func TestValidateAccessRequestClusterNames(t *testing.T) {
	for _, tc := range []struct {
		name               string
		localClusterName   string
		remoteClusterNames []string
		expectedInErr      string
	}{
		{
			name:               "local cluster is requested",
			localClusterName:   "someCluster",
			remoteClusterNames: []string{},
			expectedInErr:      "",
		}, {
			name:               "remote cluster is requested",
			localClusterName:   "notTheCorrectName",
			remoteClusterNames: []string{"someCluster"},
			expectedInErr:      "",
		}, {
			name:               "unknown cluster requested",
			localClusterName:   "notTheCorrectName",
			remoteClusterNames: []string{"notTheCorrectClusterEither"},
			expectedInErr:      "invalid or unknown cluster names",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			localCluster, err := types.NewClusterName(types.ClusterNameSpecV2{
				ClusterName: tc.localClusterName,
				ClusterID:   "someClusterID",
			})
			require.NoError(t, err)

			remoteClusters := map[string]types.RemoteCluster{}
			for _, remoteCluster := range tc.remoteClusterNames {
				remoteClusters[remoteCluster], err = types.NewRemoteCluster(remoteCluster)
				require.NoError(t, err)
			}

			mcg := mockClusterGetter{
				localCluster:   localCluster,
				remoteClusters: remoteClusters,
			}
			req, err := types.NewAccessRequestWithResources("name", "user", []string{}, []types.ResourceID{
				{ClusterName: "someCluster"},
			})
			require.NoError(t, err)

			err = ValidateAccessRequestClusterNames(mcg, req)
			if tc.expectedInErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedInErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestValidate_RequestedMaxDuration tests requested max duration
// and the default values for session and pending TTL as a result
// of requested max duration.
func TestValidate_RequestedMaxDuration(t *testing.T) {
	// describes a collection of roles and their conditions
	roleDesc := roleTestSet{
		"requestedRole": {
			// ...
		},
		"requestedRole2": {
			// ...
		},
		"setMaxTTLRole": {
			options: types.RoleOptions{
				MaxSessionTTL: types.Duration(6 * time.Hour),
			},
		},
		"defaultRole": {
			condition: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles: []string{"requestedRole", "setMaxTTLRole"},
				},
			},
		},
		"defaultShortRole": {
			condition: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles: []string{"requestedRole", "setMaxTTLRole"},
				},
			},
		},
		"maxDurationReqRole": {
			condition: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles:       []string{"requestedRole", "setMaxTTLRole"},
					MaxDuration: types.Duration(7 * day),
				},
			},
		},
		"shortMaxDurationReqRole": {
			condition: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles:       []string{"requestedRole"},
					MaxDuration: types.Duration(3 * day),
				},
			},
		},
		"shortMaxDurationReqRole2": {
			condition: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles:       []string{"requestedRole2"},
					MaxDuration: types.Duration(day),
				},
			},
		},
	}

	// describes a collection of users with various roles
	userDesc := map[string][]string{
		"alice": {"shortMaxDurationReqRole"},
		"bob":   {"defaultRole"},
		"carol": {"shortMaxDurationReqRole", "shortMaxDurationReqRole2"},
		"david": {"maxDurationReqRole"},
	}

	defaultSessionTTL := 8 * time.Hour

	g := getMockGetter(t, roleDesc, userDesc)

	tts := []struct {
		// desc is a short description of the test scenario (should be unique)
		desc string
		// requestor is the name of the requesting user
		requestor string
		// the roles to be requested (defaults to "dictator")
		roles []string
		// requestedMaxDuration is the requested requestedMaxDuration duration
		requestedMaxDuration time.Duration
		// expectedAccessDuration is the expected access duration
		expectedAccessDuration time.Duration
		// expectedSessionTTL is the expected session TTL
		expectedSessionTTL time.Duration
		// expectedPendingTTL is the time when request expires in PENDING state
		expectedPendingTTL time.Duration
		// DryRun is true if the request is a dry run
		dryRun bool
	}{
		{
			desc:                   "role max_duration is respected and sessionTTL does not exceed the calculated max duration",
			requestor:              "alice",
			roles:                  []string{"requestedRole"}, // role max_duration capped to 3 days
			requestedMaxDuration:   7 * day,                   // ignored b/c it's > role max_duration
			expectedAccessDuration: 3 * day,
			expectedSessionTTL:     8 * time.Hour, // caps to defaultSessionTTL b/c it's < than the expectedAccessDuration
			expectedPendingTTL:     3 * day,       // caps to expectedAccessDuration b/c it's < than the const default TTL
		},
		{
			desc:                   "role max_duration is still respected even with dry run (which requests for longer maxDuration)",
			requestor:              "alice",
			roles:                  []string{"requestedRole"}, // role max_duration capped to 3 days
			requestedMaxDuration:   10 * day,                  // ignored b/c it's > role max_duration
			expectedAccessDuration: 3 * day,
			expectedPendingTTL:     3 * day,
			expectedSessionTTL:     8 * time.Hour,
			dryRun:                 true,
		},
		{
			desc:                   "role max_duration is ignored when requestedMaxDuration is not set",
			requestor:              "alice",
			roles:                  []string{"requestedRole"}, // role max_duration capped to 3 days
			expectedAccessDuration: 8 * time.Hour,             // caps to defaultSessionTTL since requestedMaxDuration was not set
			expectedPendingTTL:     8 * time.Hour,
			expectedSessionTTL:     8 * time.Hour,
		},
		{
			desc:                   "when role max_duration is not set: default to defaultSessionTTL when requestedMaxDuration is not set",
			requestor:              "bob",
			roles:                  []string{"requestedRole"}, // role max_duration is not set (0)
			expectedAccessDuration: 8 * time.Hour,             // caps to defaultSessionTTL since requestedMaxDuration was not set
			expectedPendingTTL:     8 * time.Hour,
			expectedSessionTTL:     8 * time.Hour,
		},
		{
			desc:                   "when role max_duration is not set: requestedMaxDuration is respected when < defaultSessionTTL",
			requestor:              "bob",
			roles:                  []string{"requestedRole"}, // role max_duration is not set (0)
			requestedMaxDuration:   5 * time.Hour,
			expectedAccessDuration: 5 * time.Hour,
			expectedPendingTTL:     5 * time.Hour,
			expectedSessionTTL:     5 * time.Hour, // capped to expectedAccessDuration because it's < defaultSessionTTL (8h)
		},
		{
			desc:                   "when role max_duration is not set: requestedMaxDuration is ignored if > defaultSessionTTL",
			requestor:              "bob",
			roles:                  []string{"requestedRole"}, // role max_duration is not set (0)
			requestedMaxDuration:   10 * time.Hour,
			expectedAccessDuration: 8 * time.Hour, // caps to defaultSessionTTL (8h) which is < requestedMaxDuration
			expectedPendingTTL:     8 * time.Hour,
			expectedSessionTTL:     8 * time.Hour,
		},
		{
			desc:                   "when role max_duration is not set: requestedMaxDuration is ignored if > role defined sesssionTTL (6h)",
			requestor:              "bob",
			roles:                  []string{"setMaxTTLRole"}, // role max_duration is not set (0), caps sessionTTL to 6 hours
			requestedMaxDuration:   day,
			expectedAccessDuration: 6 * time.Hour, // capped to the lowest sessionTTL found in role (6h) which is < requestedMaxDuration
			expectedPendingTTL:     6 * time.Hour,
			expectedSessionTTL:     6 * time.Hour,
		},
		{
			desc:                   "when role max_duration is not set: requestedMaxDuration is respected when < role defined sessionTTL (6h)",
			requestor:              "bob",
			roles:                  []string{"setMaxTTLRole"}, // role max_duration is not set (0), caps sessionTTL to 6 hours
			requestedMaxDuration:   5 * time.Hour,
			expectedAccessDuration: 5 * time.Hour, // caps to requestedMaxDuration which is < role defined sessionTTL (6h)
			expectedPendingTTL:     5 * time.Hour,
			expectedSessionTTL:     5 * time.Hour,
		},
		{
			desc:                   "requestedMaxDuration is respected if it's < the max_duration set in role",
			requestor:              "david",
			roles:                  []string{"setMaxTTLRole"}, // role max_duration capped to default MaxAccessDuration, caps sessionTTL to 6 hours
			requestedMaxDuration:   day,                       // respected because it's < default const MaxAccessDuration
			expectedAccessDuration: day,
			expectedPendingTTL:     day,
			expectedSessionTTL:     6 * time.Hour, // capped to the lowest sessionTTL found in role which is < requestedMaxDuration
		},
		{
			desc:                   "expectedSessionTTL does not exceed requestedMaxDuration",
			requestor:              "david",
			roles:                  []string{"setMaxTTLRole"}, // caps max_duration to default MaxAccessDuration, caps sessionTTL to 6 hours
			requestedMaxDuration:   2 * time.Hour,             // respected because it's < default const MaxAccessDuration
			expectedAccessDuration: 2 * time.Hour,
			expectedPendingTTL:     2 * time.Hour,
			expectedSessionTTL:     2 * time.Hour, // capped to requestedMaxDuration because it's < role defined sessionTTL (6h)
		},
		{
			desc:                   "only the assigned role that allows the requested roles are considered for maxDuration",
			requestor:              "carol",                   // has multiple roles assigned
			roles:                  []string{"requestedRole"}, // caps max_duration to 3 days
			requestedMaxDuration:   5 * day,
			expectedAccessDuration: 3 * day,
			expectedPendingTTL:     3 * day,
			expectedSessionTTL:     8 * time.Hour,
		},
		{
			desc:                   "only the assigned role that allows the requested roles are considered for maxDuration #2",
			requestor:              "carol",                    // has multiple roles assigned
			roles:                  []string{"requestedRole2"}, // caps max_duration to 1 day
			requestedMaxDuration:   6 * day,
			expectedAccessDuration: day,
			expectedPendingTTL:     day,
			expectedSessionTTL:     8 * time.Hour,
		},
	}

	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			require.NotEmpty(t, tt.roles, "at least one role must be specified")

			// create a request for the specified author
			req, err := types.NewAccessRequest("some-id", tt.requestor, tt.roles...)
			require.NoError(t, err)

			clock := clockwork.NewFakeClock()
			now := clock.Now().UTC()
			identity := tlsca.Identity{
				Expires: now.Add(defaultSessionTTL),
			}

			validator, err := NewRequestValidator(context.Background(), clock, g, tt.requestor, ExpandVars(true))
			require.NoError(t, err)

			req.SetCreationTime(now)
			req.SetMaxDuration(now.Add(tt.requestedMaxDuration))
			req.SetDryRun(tt.dryRun)

			require.NoError(t, validator.Validate(context.Background(), req, identity))
			require.Equal(t, now.Add(tt.expectedAccessDuration), req.GetAccessExpiry())
			require.Equal(t, now.Add(tt.expectedAccessDuration), req.GetMaxDuration())
			require.Equal(t, now.Add(tt.expectedSessionTTL), req.GetSessionTLL())
			require.Equal(t, now.Add(tt.expectedPendingTTL), req.Expiry())
		})
	}
}

// TestValidate_RequestedPendingTTLAndMaxDuration tests that both requested
// max duration and pending TTL is respected (given within limits).
func TestValidate_RequestedPendingTTLAndMaxDuration(t *testing.T) {
	// describes a collection of roles and their conditions
	roleDesc := roleTestSet{
		"requestRole": {
			condition: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles:       []string{"requestRole"},
					MaxDuration: types.Duration(5 * day),
				},
			},
		},
	}

	// describes a collection of users with various roles
	userDesc := map[string][]string{
		"alice": {"requestRole"},
	}

	g := getMockGetter(t, roleDesc, userDesc)
	req, err := types.NewAccessRequest("some-id", "alice", []string{"requestRole"}...)
	require.NoError(t, err)

	clock := clockwork.NewFakeClock()
	now := clock.Now().UTC()
	defaultSessionTTL := 8 * time.Hour
	identity := tlsca.Identity{
		Expires: now.Add(defaultSessionTTL),
	}

	validator, err := NewRequestValidator(context.Background(), clock, g, "alice", ExpandVars(true))
	require.NoError(t, err)

	requestedMaxDuration := 4 * day
	requestedPendingTTL := 2 * day

	req.SetCreationTime(now)
	req.SetMaxDuration(now.Add(requestedMaxDuration))
	req.SetExpiry(now.Add(requestedPendingTTL))

	require.NoError(t, validator.Validate(context.Background(), req, identity))
	require.Equal(t, now.Add(requestedMaxDuration), req.GetAccessExpiry())
	require.Equal(t, now.Add(requestedMaxDuration), req.GetMaxDuration())
	require.Equal(t, now.Add(defaultSessionTTL), req.GetSessionTLL())
	require.Equal(t, now.Add(requestedPendingTTL), req.Expiry())
}

// TestValidate_WithAllowRequestKubernetesResources tests that requests containing
// kubernetes resources, the kinds are enforced defined by users static role
// field `request.kubernetes_resources`
func TestValidate_WithAllowRequestKubernetesResources(t *testing.T) {
	myClusterName := "teleport-cluster"

	// set up test roles
	roleDesc := map[string]types.RoleSpecV6{
		"kube-access-wildcard": {
			Allow: types.RoleConditions{
				KubernetesLabels: types.Labels{
					"*": {"*"},
				},
				KubernetesResources: []types.KubernetesResource{
					{Kind: "*", Namespace: "*", Name: "*", Verbs: []string{"*"}},
				},
			},
		},
		"kube-no-access": {
			Allow: types.RoleConditions{},
		},
		"kube-access-namespace": {
			Allow: types.RoleConditions{
				KubernetesLabels: types.Labels{
					"*": {"*"},
				},
				KubernetesResources: []types.KubernetesResource{
					{Kind: types.KindNamespace, Namespace: "*", Name: "*", Verbs: []string{"*"}},
				},
			},
		},
		"kube-access-pod": {
			Allow: types.RoleConditions{
				KubernetesLabels: types.Labels{
					"*": {"*"},
				},
				KubernetesResources: []types.KubernetesResource{
					{Kind: types.KindKubePod, Namespace: "*", Name: "*", Verbs: []string{"*"}},
				},
			},
		},
		"kube-access-deployment": {
			Allow: types.RoleConditions{
				KubernetesLabels: types.Labels{
					"*": {"*"},
				},
				KubernetesResources: []types.KubernetesResource{
					{Kind: types.KindKubeDeployment, Namespace: "*", Name: "*", Verbs: []string{"*"}},
				},
			},
		},
		"db-access-wildcard": {
			Allow: types.RoleConditions{
				DatabaseLabels: types.Labels{
					"*": {"*"},
				},
			},
		},

		"request-undefined_search-wildcard": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{"kube-access-wildcard", "db-access-wildcard"},
				},
			},
		},
		"request-pod_search-as-roles-undefined": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					KubernetesResources: []types.RequestKubernetesResource{
						{Kind: types.KindKubePod},
					},
				},
			},
		},
		"request-namespace_search-namespace": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{"kube-access-namespace", "db-access-wildcard"},
					KubernetesResources: []types.RequestKubernetesResource{
						{Kind: types.KindKubeNamespace},
					},
				},
			},
		},
		// Allows requesting for any subresources, but NOT kube_cluster
		"request-wildcard_search-wildcard": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{"kube-access-wildcard", "db-access-wildcard"},
					KubernetesResources: []types.RequestKubernetesResource{
						{Kind: types.Wildcard},
					},
				},
			},
		},
		// Allows wildcard search, but should only accept kube secret
		"request-secret_search-wildcard": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{"kube-access-wildcard"},
					KubernetesResources: []types.RequestKubernetesResource{
						{Kind: types.KindKubeSecret},
					},
				},
			},
		},
		"request-pod_search-pods": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{"kube-access-pod"},
					KubernetesResources: []types.RequestKubernetesResource{
						{Kind: types.KindKubePod},
					},
				},
			},
		},
		"request-deployment_search-deployment": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{"kube-access-deployment"},
					KubernetesResources: []types.RequestKubernetesResource{
						{Kind: types.KindKubeDeployment},
					},
				},
			},
		},
		"request-deployment-pod_search-deployment-pod": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{"kube-access-deployment", "kube-access-pod"},
					KubernetesResources: []types.RequestKubernetesResource{
						{Kind: types.KindKubeDeployment},
						{Kind: types.KindKubePod},
					},
				},
			},
		},
		"request-namespace-but-no-access": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{"db-access-wildcard", "kube-no-access"},
					KubernetesResources: []types.RequestKubernetesResource{
						{Kind: types.KindNamespace},
					},
				},
			},
		},
		"request-namespace_search-namespace_deny-secret": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{"kube-access-namespace"},
					KubernetesResources: []types.RequestKubernetesResource{
						{Kind: types.KindNamespace},
					},
				},
			},
			Deny: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					KubernetesResources: []types.RequestKubernetesResource{
						{Kind: types.KindKubeSecret},
					},
				},
			},
		},
		"request-undefined_search-wildcard_deny-deployment-pod": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{"kube-access-wildcard"},
				},
			},
			Deny: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					KubernetesResources: []types.RequestKubernetesResource{
						{Kind: types.KindKubeDeployment},
						{Kind: types.KindKubePod},
					},
				},
			},
		},
		"request-wildcard-cancels-deny-wildcard": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{"kube-access-namespace"},
					KubernetesResources: []types.RequestKubernetesResource{
						{Kind: types.Wildcard},
					},
				},
			},
			Deny: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					KubernetesResources: []types.RequestKubernetesResource{
						{Kind: types.Wildcard},
					},
				},
			},
		},
	}
	roles := make(map[string]types.Role)
	for name, spec := range roleDesc {
		role, err := types.NewRole(name, spec)
		require.NoError(t, err)
		roles[name] = role
	}

	// Define a kube server
	kube, err := types.NewKubernetesClusterV3(types.Metadata{
		Name: "kube",
	},
		types.KubernetesClusterSpecV3{},
	)
	require.NoError(t, err)
	kubeServer, err := types.NewKubernetesServerV3FromCluster(kube, "_", "_")
	require.NoError(t, err)

	// Define a db server
	db, err := types.NewDatabaseV3(types.Metadata{
		Name: "db",
	}, types.DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "example.com:3000",
	})
	require.NoError(t, err)
	dbServer, err := types.NewDatabaseServerV3(types.Metadata{
		Name: db.GetName(),
	}, types.DatabaseServerSpecV3{
		HostID:   "db-server",
		Hostname: "db-server",
		Database: db,
	})
	require.NoError(t, err)

	// start test
	testCases := []struct {
		desc                      string
		userStaticRoles           []string
		requestResourceIDs        []types.ResourceID
		requestRoles              []string
		wantInvalidRequestKindErr bool
		wantNoRolesConfiguredErr  bool
		expectedRequestRoles      []string
	}{
		{
			desc:                 "request.kubernetes_resources undefined allows anything (kube_cluster and its subresources)",
			userStaticRoles:      []string{"request-undefined_search-wildcard"},
			expectedRequestRoles: []string{"kube-access-wildcard", "db-access-wildcard"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindKubernetesCluster, ClusterName: myClusterName, Name: "kube"},
				{Kind: types.KindDatabase, ClusterName: myClusterName, Name: "db"},
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "some-namespace"},
			},
		},
		{
			desc:                 "request.kubernetes_resources undefined takes precedence over configured allow field (allows anything)",
			userStaticRoles:      []string{"request-undefined_search-wildcard", "request-secret_search-wildcard"},
			expectedRequestRoles: []string{"kube-access-wildcard", "db-access-wildcard"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindKubernetesCluster, ClusterName: myClusterName, Name: "kube"},
				{Kind: types.KindDatabase, ClusterName: myClusterName, Name: "db"},
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "some-namespace"},
			},
		},
		{
			desc:            "configured deny request.kubernetes_resources takes precedence over undefined deny field",
			userStaticRoles: []string{"request-wildcard-cancels-deny-wildcard", "request-namespace_search-namespace"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace"},
				{Kind: types.KindDatabase, ClusterName: myClusterName, Name: "db"},
			},
			wantInvalidRequestKindErr: true,
		},
		{
			desc:                 "request.kubernetes_resources does not get applied with a role without search_as_roles defined",
			userStaticRoles:      []string{"request-namespace_search-namespace", "request-pod_search-as-roles-undefined"},
			expectedRequestRoles: []string{"kube-access-namespace", "db-access-wildcard"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindDatabase, ClusterName: myClusterName, Name: "db"},
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "some-namespace"},
			},
		},
		{
			desc:                 "wildcard allows any kube subresources",
			userStaticRoles:      []string{"request-wildcard_search-wildcard"},
			expectedRequestRoles: []string{"kube-access-wildcard", "db-access-wildcard"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "some-namespace"},
				{Kind: types.KindDatabase, ClusterName: myClusterName, Name: "db"},
				{Kind: types.KindKubeSecret, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace/secret-name"},
			},
		},
		{
			desc:            "wildcard rejects kube_cluster kind among other valid requests",
			userStaticRoles: []string{"request-wildcard_search-wildcard"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "some-namespace"},
				{Kind: types.KindDatabase, ClusterName: myClusterName, Name: "db"},
				{Kind: types.KindKubernetesCluster, ClusterName: myClusterName, Name: "kube"},
			},
			wantInvalidRequestKindErr: true,
		},
		{
			desc:            "wildcard rejects single kube_cluster request",
			userStaticRoles: []string{"request-wildcard_search-wildcard"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindKubernetesCluster, ClusterName: myClusterName, Name: "kube"},
			},
			wantInvalidRequestKindErr: true,
		},
		{
			desc:            "error with `no roles configured` if search_as_roles does not grant kube resource access",
			userStaticRoles: []string{"request-namespace-but-no-access"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindDatabase, ClusterName: myClusterName, Name: "db"},
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace"},
			},
			wantNoRolesConfiguredErr: true,
		},
		{
			desc: "prune search_as_roles that does not meet request.kubernetes_resources (unconfigured field)",
			// search as role "kube-access-namespace" got pruned b/c the request included "kube_cluster" which wasn't allowed.
			userStaticRoles:      []string{"request-undefined_search-wildcard", "request-namespace_search-namespace"},
			expectedRequestRoles: []string{"kube-access-wildcard", "db-access-wildcard"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindKubernetesCluster, ClusterName: myClusterName, Name: "kube"},
				{Kind: types.KindDatabase, ClusterName: myClusterName, Name: "db"},
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "some-namespace"},
			},
		},
		{
			desc: "prune search_as_roles that does not match request.kubernetes_resources field",
			userStaticRoles: []string{
				"request-pod_search-pods",
				"request-namespace_search-namespace",
				"request-deployment_search-deployment",
			},
			expectedRequestRoles: []string{"kube-access-namespace", "db-access-wildcard"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindDatabase, ClusterName: myClusterName, Name: "db"},
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "some-namespace"},
			},
		},
		{
			desc: "reject when kinds don't match (root only)",
			userStaticRoles: []string{
				"request-pod_search-pods",
				"request-namespace_search-namespace",
				"request-deployment_search-deployment",
			},
			expectedRequestRoles: []string{"kube-access-pod", "kube-access-namespace", "db-access-wildcard"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindDatabase, ClusterName: myClusterName, Name: "db"},
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace"},
				{Kind: types.KindKubePod, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace/pod-name"},
			},
			wantInvalidRequestKindErr: true,
		},
		{
			desc: "reject when kinds don't match (leaf only)",
			userStaticRoles: []string{
				"request-pod_search-pods",
				"request-namespace_search-namespace",
				"request-deployment_search-deployment",
			},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindDatabase, ClusterName: "some-leaf", Name: "db2"},
				{Kind: types.KindKubeSecret, ClusterName: "some-leaf", Name: "leaf-kube", SubResourceName: "namespace/secret-name"},
			},
			wantInvalidRequestKindErr: true,
		},
		{
			desc: "reject when kinds don't match (mix of leaf and root cluster)",
			userStaticRoles: []string{
				"request-pod_search-pods",
				"request-namespace_search-namespace",
				"request-deployment_search-deployment",
			},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindKubeDeployment, ClusterName: "some-leaf", Name: "kube", SubResourceName: "namespace/secret-name"},
				{Kind: types.KindNamespace, ClusterName: "some-leaf", Name: "kube-leaf", SubResourceName: "namespace"},
			},
			wantInvalidRequestKindErr: true,
		},
		{
			desc: "prune roles that does not give you access",
			userStaticRoles: []string{
				"request-namespace_search-namespace",
				"request-namespace-but-no-access",
				"request-deployment_search-deployment",
			},
			expectedRequestRoles: []string{"kube-access-namespace"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace"},
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace2"},
			},
		},
		{
			desc: "don't further prune roles when leaf requests are present",
			userStaticRoles: []string{
				"request-namespace_search-namespace",
				"request-namespace-but-no-access",
				"request-deployment_search-deployment",
			},
			// db-access-wildcard and kube-no-access shouldn't be in the list, but a leaf is present
			// which skips matcher tests.
			expectedRequestRoles: []string{"kube-access-namespace", "db-access-wildcard", "kube-no-access"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace"},
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace2"},
				{Kind: types.KindNamespace, ClusterName: "some-leaf", Name: "kube-leaf", SubResourceName: "namespace"},
			},
		},
		{
			desc:            "reject if kinds don't match even though search_as_roles allows wildcard access",
			userStaticRoles: []string{"request-secret_search-wildcard"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "some-namespace"},
			},
			wantInvalidRequestKindErr: true,
		},
		{
			desc:                 "allow namespace request when deny is not matched",
			userStaticRoles:      []string{"request-namespace_search-namespace_deny-secret"},
			expectedRequestRoles: []string{"kube-access-namespace"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace"},
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace2"},
			},
		},
		{
			desc:                 "allow namespace request when deny is not matched with leaf clusters",
			userStaticRoles:      []string{"request-namespace_search-namespace_deny-secret"},
			expectedRequestRoles: []string{"kube-access-namespace"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindKubeNamespace, ClusterName: "leaf-cluster", Name: "kube", SubResourceName: "namespace"},
				{Kind: types.KindKubeNamespace, ClusterName: "leaf-cluster", Name: "kube", SubResourceName: "namespace2"},
			},
		},
		{
			desc:                 "allow a list of different request.kubernetes_resources from same role",
			userStaticRoles:      []string{"request-deployment-pod_search-deployment-pod", "request-namespace_search-namespace"},
			expectedRequestRoles: []string{"kube-access-deployment", "kube-access-pod"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindKubeDeployment, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace/deployment"},
				{Kind: types.KindKubePod, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace/pod"},
			},
		},
		{
			desc:            "deny request when deny is defined from another role (denies are globally matched)",
			userStaticRoles: []string{"request-namespace_search-namespace_deny-secret", "request-secret_search-wildcard"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindKubeSecret, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace/secret-name"},
			},
			wantInvalidRequestKindErr: true,
		},
		{
			desc:                 "allow wildcard request when deny is not matched",
			userStaticRoles:      []string{"request-undefined_search-wildcard_deny-deployment-pod"},
			expectedRequestRoles: []string{"kube-access-wildcard"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace"},
				{Kind: types.KindKubernetesCluster, ClusterName: myClusterName, Name: "kube"},
			},
		},
		{
			desc:            "deny wildcard request when deny is matched",
			userStaticRoles: []string{"request-undefined_search-wildcard_deny-deployment-pod"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace"},
				{Kind: types.KindKubernetesCluster, ClusterName: myClusterName, Name: "kube"},
				{Kind: types.KindKubeDeployment, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace/deployment"},
			},
			wantInvalidRequestKindErr: true,
		},
		{
			desc:            "request.kubernetes_resources cancel each other (config error where no kube resources becomes requestable)",
			userStaticRoles: []string{"request-wildcard-cancels-deny-wildcard"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace"},
				{Kind: types.KindKubeDeployment, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace/deployment"},
			},
			wantInvalidRequestKindErr: true,
		},
		{
			desc:                 "request.kubernetes_resources cancel each other also rejects kube_cluster kinds (config error)",
			userStaticRoles:      []string{"request-wildcard-cancels-deny-wildcard"},
			expectedRequestRoles: []string{"kube-access-wildcard"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindKubernetesCluster, ClusterName: myClusterName, Name: "kube"},
			},
			wantInvalidRequestKindErr: true,
		},
		{
			desc: "allow when requested role matches requested kinds",
			userStaticRoles: []string{
				"request-namespace_search-namespace",
				"request-namespace-but-no-access",
				"request-deployment_search-deployment",
			},
			requestRoles:         []string{"kube-access-namespace"},
			expectedRequestRoles: []string{"kube-access-namespace"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace"},
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace2"},
			},
		},
		{
			desc: "reject when requested role does not match ALL requested kinds",
			userStaticRoles: []string{
				"request-namespace_search-namespace",
				"request-namespace-but-no-access",
				"request-deployment_search-deployment",
			},
			requestRoles: []string{"kube-access-namespace", "kube-access-deployment"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace"},
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace2"},
			},
			wantInvalidRequestKindErr: true,
		},
		{
			desc: "reject when requested role does not allow all requested kinds",
			userStaticRoles: []string{
				"request-namespace_search-namespace",
				"request-namespace-but-no-access",
				"request-deployment_search-deployment",
			},
			requestRoles: []string{"kube-access-namespace"},
			requestResourceIDs: []types.ResourceID{
				{Kind: types.KindKubeNamespace, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace"},
				{Kind: types.KindKubeDeployment, ClusterName: myClusterName, Name: "kube", SubResourceName: "namespace/deployment"},
			},
			wantInvalidRequestKindErr: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			uls, err := userloginstate.New(header.Metadata{
				Name: "test-user",
			}, userloginstate.Spec{
				Roles: tc.userStaticRoles,
			})
			require.NoError(t, err)
			userStates := map[string]*userloginstate.UserLoginState{
				uls.GetName(): uls,
			}

			g := &mockGetter{
				roles:       roles,
				userStates:  userStates,
				clusterName: myClusterName,
				kubeServers: make(map[string]types.KubeServer),
				dbServers:   make(map[string]types.DatabaseServer),
			}
			g.kubeServers[kube.GetName()] = kubeServer
			g.dbServers[dbServer.GetName()] = dbServer

			req, err := types.NewAccessRequestWithResources(
				"some-id", uls.GetName(), tc.requestRoles, tc.requestResourceIDs)
			require.NoError(t, err)

			clock := clockwork.NewFakeClock()
			identity := tlsca.Identity{
				Expires: clock.Now().UTC().Add(8 * time.Hour),
			}

			validator, err := NewRequestValidator(context.Background(), clock, g, uls.GetName(), ExpandVars(true))
			require.NoError(t, err)

			err = validator.Validate(context.Background(), req, identity)
			if tc.wantInvalidRequestKindErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), InvalidKubernetesKindAccessRequest)
			} else if tc.wantNoRolesConfiguredErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), `no roles configured in the "search_as_roles"`)
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.expectedRequestRoles, req.GetRoles())
			}
		})
	}
}

type roleTestSet map[string]struct {
	condition types.RoleConditions
	options   types.RoleOptions
}

func getMockGetter(t *testing.T, roleDesc roleTestSet, userDesc map[string][]string) *mockGetter {
	t.Helper()

	roles := make(map[string]types.Role)

	for name, desc := range roleDesc {
		role, err := types.NewRole(name, types.RoleSpecV6{
			Allow:   desc.condition,
			Options: desc.options,
		})
		require.NoError(t, err)

		roles[name] = role
	}

	userStates := make(map[string]*userloginstate.UserLoginState)

	for name, roles := range userDesc {
		uls, err := userloginstate.New(header.Metadata{
			Name: name,
		}, userloginstate.Spec{
			Roles: roles,
		})
		require.NoError(t, err)

		userStates[name] = uls
	}

	g := &mockGetter{
		roles:      roles,
		userStates: userStates,
	}
	return g
}
