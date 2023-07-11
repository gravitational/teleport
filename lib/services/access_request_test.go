/*
Copyright 2019 Gravitational, Inc.

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

package services

import (
	"context"
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
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/tlsca"
)

// mockGetter mocks the UserAndRoleGetter interface.
type mockGetter struct {
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
	user, err := types.NewUser(name)
	require.NoError(t, err)

	user.SetRoles(roles)
	m.users[name] = user
	return name
}

func (m *mockGetter) GetUser(name string, withSecrets bool) (types.User, error) {
	if withSecrets {
		return nil, trace.NotImplemented("mock getter does not store secrets")
	}
	user, ok := m.users[name]
	if !ok {
		return nil, trace.NotFound("no such user: %q", name)
	}
	return user, nil
}

func (m *mockGetter) GetRole(ctx context.Context, name string) (types.Role, error) {
	role, ok := m.roles[name]
	if !ok {
		return nil, trace.NotFound("no such role: %q", name)
	}
	return role, nil
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
				Where: `contains(request.system_annotations["mechanisms"],"uprising")`,
			},
		},
		// the intelligentsia can put dictators into power via consensus
		"intelligentsia": {
			ReviewRequests: &types.AccessReviewConditions{
				Roles: []string{"dictator"},
				Where: `contains(request.system_annotations["mechanism"],"consensus")`,
			},
		},
		// the military can put dictators into power via a coup our treachery
		"military": {
			ReviewRequests: &types.AccessReviewConditions{
				Roles: []string{"dictator"},
				Where: `contains(request.system_annotations["mechanisms"],"coup") || contains(request.system_annotations["mechanism"],"treachery")`,
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
	userDesc := map[string][]string{
		"alice": {"populist", "proletariat", "intelligentsia", "military"},
		"bob":   {"general", "proletariat", "intelligentsia", "military"},
		"carol": {"conqueror", "proletariat", "intelligentsia", "military"},
		"dave":  {"populist", "general", "conqueror"},
		"erika": {"populist", "idealist"},
	}

	users := make(map[string]types.User)

	for name, roles := range userDesc {
		user, err := types.NewUser(name)
		require.NoError(t, err)

		user.SetRoles(roles)
		users[name] = user
	}

	g := &mockGetter{
		roles: roles,
		users: users,
	}

	const (
		approve = types.RequestState_APPROVED
		deny    = types.RequestState_DENIED
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
	}

	tts := []struct {
		// desc is a short description of the test scenario (should be unique)
		desc string
		// requestor is the name of the requesting user
		requestor string
		// the roles to be requested (defaults to "dictator")
		roles   []string
		reviews []review
	}{
		{
			desc:      "populist approval via multi-threshold match",
			requestor: "alice", // permitted by role populist
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
				{ // adds second denial to all thresholds, no effect since a state-transition was already triggered.
					author:  g.user(t, "proletariat", "intelligentsia", "military"),
					propose: deny,
					expect:  approve,
				},
			},
		},
		{
			desc:      "populist approval via consensus threshold",
			requestor: "alice", // permitted by role populist
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
	}

	for _, tt := range tts {

		if len(tt.roles) == 0 {
			tt.roles = []string{"dictator"}
		}

		// create a request for the specified author
		req, err := types.NewAccessRequest("some-id", tt.requestor, tt.roles...)
		require.NoError(t, err, "scenario=%q", tt.desc)

		clock := clockwork.NewFakeClock()
		identity := tlsca.Identity{
			Expires: clock.Now().UTC().Add(8 * time.Hour),
		}

		// perform request validation (necessary in order to initialize internal
		// request variables like annotations and thresholds).
		validator, err := NewRequestValidator(context.Background(), clock, g, tt.requestor, ExpandVars(true))
		require.NoError(t, err, "scenario=%q", tt.desc)

		require.NoError(t, validator.Validate(context.Background(), req, identity), "scenario=%q", tt.desc)

	Inner:
		for ri, rt := range tt.reviews {
			if rt.expect.IsNone() {
				rt.expect = types.RequestState_PENDING
			}

			checker, err := NewReviewPermissionChecker(context.TODO(), g, rt.author)
			require.NoError(t, err, "scenario=%q, rev=%d", tt.desc, ri)

			canReview, err := checker.CanReviewRequest(req)
			require.NoError(t, err, "scenario=%q, rev=%d", tt.desc, ri)

			if rt.noReview {
				require.False(t, canReview, "scenario=%q, rev=%d", tt.desc, ri)
				continue Inner
			}

			rev := types.AccessReview{
				Author:        rt.author,
				ProposedState: rt.propose,
			}

			author, ok := users[rt.author]
			require.True(t, ok, "scenario=%q, rev=%d", tt.desc, ri)

			err = ApplyAccessReview(req, rev, author)
			require.NoError(t, err, "scenario=%q, rev=%d", tt.desc, ri)
			require.Equal(t, rt.expect.String(), req.GetState().String(), "scenario=%q, rev=%d", tt.desc, ri)
		}
	}
}

// TestMaxLength tests that we reject too large access requests.
func TestMaxLength(t *testing.T) {
	req, err := types.NewAccessRequest("some-id", "dave", "dictator", "never")
	require.NoError(t, err)

	var s []byte
	for i := 0; i <= maxAccessRequestReasonSize; i++ {
		s = append(s, 'a')
	}

	req.SetRequestReason(string(s))
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
				Reviewer: reviewAuthorContext{
					Roles: []string{"dev"},
					Traits: map[string][]string{
						"teams": {"staging-admin"},
					},
				},
				Review: reviewParamsContext{
					Reason: "ok",
					Annotations: map[string][]string{
						"constraints": {"no-admin"},
					},
				},
				Request: reviewRequestContext{
					Roles:  []string{"dev"},
					Reason: "plz",
					SystemAnnotations: map[string][]string{
						"teams": {"staging-dev"},
					},
				},
			},
			willMatch: []string{
				`contains(reviewer.roles,"dev")`,
				`contains(reviewer.traits["teams"],"staging-admin") && contains(request.system_annotations["teams"],"staging-dev")`,
				`!contains(review.annotations["constraints"],"no-admin") || !contains(request.roles,"admin")`,
				`equals(request.reason,"plz") && equals(review.reason,"ok")`,
				`contains(reviewer.roles,"admin") || contains(reviewer.roles,"dev")`,
				`!(contains(reviewer.roles,"foo") || contains(reviewer.roles,"bar"))`,
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
		parser, err := NewJSONBoolParser(tt.ctx)
		require.NoError(t, err)
		for _, expr := range tt.willMatch {
			result, err := parser.EvalBoolPredicate(expr)
			require.NoError(t, err)
			require.True(t, result)
		}

		for _, expr := range tt.wontMatch {
			result, err := parser.EvalBoolPredicate(expr)
			require.NoError(t, err)
			require.False(t, result)
		}

		for _, expr := range tt.wontParse {
			_, err := parser.EvalBoolPredicate(expr)
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
			user, err := types.NewUser("test-user")
			require.NoError(t, err)
			user.SetRoles(tc.currentRoles)
			users := map[string]types.User{
				user.GetName(): user,
			}

			g := &mockGetter{
				roles:       roles,
				users:       users,
				clusterName: "my-cluster",
			}

			req, err := types.NewAccessRequestWithResources(
				"some-id", user.GetName(), tc.requestRoles, tc.requestResourceIDs)
			require.NoError(t, err)

			clock := clockwork.NewFakeClock()
			identity := tlsca.Identity{
				Expires: clock.Now().UTC().Add(8 * time.Hour),
			}

			validator, err := NewRequestValidator(context.Background(), clock, g, user.GetName(), ExpandVars(true))
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
	g.users[user].SetTraits(map[string][]string{
		"logins": {"responder"},
		"team":   {"response-team"},
	})

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

			accessCaps, err := CalculateAccessCapabilities(ctx, clock, g, types.AccessCapabilitiesRequest{User: user, ResourceIDs: tc.requestResourceIDs})
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

// TestRequestTTL verifies that the TTL for the Access Request gets reduced by
// requested access time and lifetime of the requesting certificate.
func TestRequestTTL(t *testing.T) {
	clock := clockwork.NewFakeClock()
	now := clock.Now().UTC()

	tests := []struct {
		desc          string
		expiry        time.Time
		identity      tlsca.Identity
		maxSessionTTL time.Duration
		expectedTTL   time.Duration
		assertion     require.ErrorAssertionFunc
	}{
		{
			desc:          "access request with ttl, below limit",
			expiry:        now.Add(8 * time.Hour),
			identity:      tlsca.Identity{Expires: now.Add(10 * time.Hour)},
			maxSessionTTL: 10 * time.Hour,
			expectedTTL:   8 * time.Hour,
			assertion:     require.NoError,
		},
		{
			desc:          "access request with ttl, above limit",
			expiry:        now.Add(11 * time.Hour),
			identity:      tlsca.Identity{Expires: now.Add(10 * time.Hour)},
			maxSessionTTL: 10 * time.Hour,
			assertion:     require.Error,
		},
		{
			desc:          "access request without ttl (default ttl)",
			expiry:        time.Time{},
			identity:      tlsca.Identity{Expires: now.Add(10 * time.Hour)},
			maxSessionTTL: 10 * time.Hour,
			expectedTTL:   defaults.PendingAccessDuration,
			assertion:     require.NoError,
		},
		{
			desc:          "access request without ttl (default ttl), truncation by identity expiration",
			expiry:        time.Time{},
			identity:      tlsca.Identity{Expires: now.Add(12 * time.Minute)},
			maxSessionTTL: 13 * time.Minute,
			expectedTTL:   12 * time.Minute,
			assertion:     require.NoError,
		},
		{
			desc:          "access request without ttl (default ttl), truncation by role max session ttl",
			expiry:        time.Time{},
			identity:      tlsca.Identity{Expires: now.Add(14 * time.Hour)},
			maxSessionTTL: 13 * time.Minute,
			expectedTTL:   13 * time.Minute,
			assertion:     require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// Setup test user "foo" and "bar" and the mock auth server that
			// will return users and roles.
			user, err := types.NewUser("foo")
			require.NoError(t, err)
			user.SetRoles([]string{"bar"})

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
			request.SetExpiry(tt.expiry)
			require.NoError(t, err)

			ttl, err := validator.requestTTL(context.Background(), tt.identity, request)
			tt.assertion(t, err)
			if err == nil {
				require.Equal(t, tt.expectedTTL, ttl)
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
			user.SetRoles([]string{"bar"})

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

			ttl, err := validator.sessionTTL(context.Background(), tt.identity, request)
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
		assertion func(t *testing.T, validator *RequestValidator)
	}{
		{
			name: "no roles",
			assertion: func(t *testing.T, validator *RequestValidator) {
				require.False(t, validator.requireReason)
				require.False(t, validator.autoRequest)
				require.Empty(t, validator.prompt)
			},
		},
		{
			name:  "with prompt",
			roles: []types.Role{empty, optionalRole, promptRole},
			assertion: func(t *testing.T, validator *RequestValidator) {
				require.False(t, validator.requireReason)
				require.False(t, validator.autoRequest)
				require.Equal(t, "test prompt", validator.prompt)
			},
		},
		{
			name:  "with auto request",
			roles: []types.Role{alwaysRole},
			assertion: func(t *testing.T, validator *RequestValidator) {
				require.False(t, validator.requireReason)
				require.True(t, validator.autoRequest)
				require.Empty(t, validator.prompt)
			},
		},
		{
			name:  "with prompt and auto request",
			roles: []types.Role{promptRole, alwaysRole},
			assertion: func(t *testing.T, validator *RequestValidator) {
				require.False(t, validator.requireReason)
				require.True(t, validator.autoRequest)
				require.Equal(t, "test prompt", validator.prompt)
			},
		},
		{
			name:  "with reason and auto prompt",
			roles: []types.Role{reasonRole},
			assertion: func(t *testing.T, validator *RequestValidator) {
				require.True(t, validator.requireReason)
				require.True(t, validator.autoRequest)
				require.Empty(t, validator.prompt)
			},
		},
	}

	for _, test := range cases {
		user, err := types.NewUser("foo")
		require.NoError(t, err)

		getter := &mockGetter{
			users: make(map[string]types.User),
			roles: make(map[string]types.Role),
		}

		for _, r := range test.roles {
			getter.roles[r.GetName()] = r
			user.AddRole(r.GetName())
		}

		getter.users[user.GetName()] = user

		validator, err := NewRequestValidator(context.Background(), clock, getter, user.GetName(), ExpandVars(true))
		require.NoError(t, err)
		test.assertion(t, &validator)
	}

}

type mockClusterGetter struct {
	localCluster   types.ClusterName
	remoteClusters map[string]types.RemoteCluster
}

func (mcg mockClusterGetter) GetClusterName(opts ...MarshalOption) (types.ClusterName, error) {
	return mcg.localCluster, nil
}

func (mcg mockClusterGetter) GetRemoteCluster(clusterName string) (types.RemoteCluster, error) {
	if cluster, ok := mcg.remoteClusters[clusterName]; ok {
		return cluster, nil
	}
	return nil, trace.NotFound("remote cluster %q was not found", clusterName)
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

type mockResourceLister struct {
	resources []types.ResourceWithLabels
}

func (m *mockResourceLister) ListResources(ctx context.Context, _ proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	return &types.ListResourcesResponse{
		Resources: m.resources,
	}, nil
}

func TestGetResourceDetails(t *testing.T) {
	clusterName := "cluster"

	presence := &mockResourceLister{
		resources: []types.ResourceWithLabels{
			newNode(t, "node1", "hostname 1"),
			newApp(t, "app1", "friendly app 1", types.OriginDynamic),
			newApp(t, "app2", "friendly app 2", types.OriginDynamic),
			newApp(t, "app3", "friendly app 3", types.OriginOkta),
			newUserGroup(t, "group1", "friendly group 1", types.OriginOkta),
		},
	}
	resourceIDs := []types.ResourceID{
		newResourceID(clusterName, types.KindNode, "node1"),
		newResourceID(clusterName, types.KindApp, "app1"),
		newResourceID(clusterName, types.KindApp, "app2"),
		newResourceID(clusterName, types.KindApp, "app3"),
		newResourceID(clusterName, types.KindUserGroup, "group1"),
	}

	ctx := context.Background()

	details, err := GetResourceDetails(ctx, clusterName, presence, resourceIDs)
	require.NoError(t, err)

	// Check the resource details to see if friendly names properly propagated.

	// Node should be named for its hostname.
	require.Equal(t, "hostname 1", details[types.ResourceIDToString(resourceIDs[0])].FriendlyName)

	// app1 and app2 are expected to be empty because they're not Okta sourced resources.
	require.Empty(t, details[types.ResourceIDToString(resourceIDs[1])].FriendlyName)

	require.Empty(t, details[types.ResourceIDToString(resourceIDs[2])].FriendlyName)

	// This Okta sourced app should have a friendly name.
	require.Equal(t, "friendly app 3", details[types.ResourceIDToString(resourceIDs[3])].FriendlyName)

	// This Okta sourced user group should have a friendly name.
	require.Equal(t, "friendly group 1", details[types.ResourceIDToString(resourceIDs[4])].FriendlyName)
}

type roleTestSet map[string]struct {
	condition types.RoleConditions
	options   types.RoleOptions
}

func TestDurations(t *testing.T) {
	// describes a collection of roles and their conditions
	roleDesc := roleTestSet{
		"requestedRole": {
			// ...
		},
		"setMaxTTLRole": {
			options: types.RoleOptions{
				MaxSessionTTL: types.Duration(8 * time.Hour),
			},
		},
		"defaultRole": {
			condition: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles: []string{"requestedRole", "setMaxTTLRole"},
				},
			},
		},
		"shortPersistReqRole": {
			condition: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles:   []string{"requestedRole"},
					Persist: types.Duration(3 * day),
				},
			},
		},
		"maxTTLPersistRole": {
			condition: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles:   []string{"requestedRole"},
					Persist: types.Duration(day),
				},
			},
			options: types.RoleOptions{
				MaxSessionTTL: types.Duration(10 * time.Hour),
			},
		},
	}

	// describes a collection of users with various roles
	userDesc := map[string][]string{
		"alice": {"shortPersistReqRole"},
		"bob":   {"defaultRole"},
		"carol": {"defaultRole"},
		"dave":  {"maxTTLPersistRole"},
		"erika": {"idealist"},
	}

	g := getMockGetter(t, roleDesc, userDesc)

	tts := []struct {
		// desc is a short description of the test scenario (should be unique)
		desc string
		// requestor is the name of the requesting user
		requestor string
		// the roles to be requested (defaults to "dictator")
		roles []string

		persist time.Duration

		expectedAccessDuration time.Duration
	}{
		{
			desc:                   "role persist is respected",
			requestor:              "alice",
			roles:                  []string{"requestedRole"},
			persist:                7 * day,
			expectedAccessDuration: 3 * day,
		},
		{
			desc:                   "persist not set, default maxTTL (8h)",
			requestor:              "bob",
			roles:                  []string{"requestedRole"},
			expectedAccessDuration: 8 * time.Hour,
		},
		{
			desc:                   "persist inside request is respected",
			requestor:              "bob",
			roles:                  []string{"requestedRole"},
			persist:                5 * time.Hour,
			expectedAccessDuration: 5 * time.Hour,
		},
		{
			desc:                   "persist can exceed maxTTL",
			requestor:              "carol",
			roles:                  []string{"setMaxTTLRole"},
			persist:                day,
			expectedAccessDuration: day,
		},
		{
			desc:                   "persist shorter than maxTTL",
			requestor:              "carol",
			roles:                  []string{"setMaxTTLRole"},
			persist:                2 * time.Hour,
			expectedAccessDuration: 2 * time.Hour,
		},
	}

	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			require.GreaterOrEqual(t, len(tt.roles), 1, "at least one role must be specified")

			// create a request for the specified author
			req, err := types.NewAccessRequest("some-id", tt.requestor, tt.roles...)
			require.NoError(t, err)

			clock := clockwork.NewFakeClock()
			now := clock.Now().UTC()
			identity := tlsca.Identity{
				Expires: now.Add(8 * time.Hour),
			}

			// perform request validation (necessary in order to initialize internal
			// request variables like annotations and thresholds).
			validator, err := NewRequestValidator(context.Background(), clock, g, tt.requestor, ExpandVars(true))
			require.NoError(t, err)

			req.SetCreationTime(now)
			req.SetPersist(now.Add(tt.persist))

			require.NoError(t, validator.Validate(context.Background(), req, identity))

			require.Equal(t, now.Add(tt.expectedAccessDuration), req.GetAccessExpiry())
		})
	}
}

func getMockGetter(t *testing.T, roleDesc roleTestSet, userDesc map[string][]string) *mockGetter {
	roles := make(map[string]types.Role)

	for name, desc := range roleDesc {
		role, err := types.NewRole(name, types.RoleSpecV6{
			Allow:   desc.condition,
			Options: desc.options,
		})
		require.NoError(t, err)

		roles[name] = role
	}

	users := make(map[string]types.User)

	for name, roles := range userDesc {
		user, err := types.NewUser(name)
		require.NoError(t, err)

		user.SetRoles(roles)
		users[name] = user
	}

	g := &mockGetter{
		roles: roles,
		users: users,
	}
	return g
}

func newNode(t *testing.T, name, hostname string) types.Server {
	node, err := types.NewServer(name, types.KindNode,
		types.ServerSpecV2{
			Hostname: hostname,
		})
	require.NoError(t, err)
	return node
}

func newApp(t *testing.T, name, description, origin string) types.Application {
	app, err := types.NewAppV3(types.Metadata{
		Name:        name,
		Description: description,
		Labels: map[string]string{
			types.OriginLabel: origin,
		},
	},
		types.AppSpecV3{
			URI:        "https://some-addr.com",
			PublicAddr: "https://some-addr.com",
		})
	require.NoError(t, err)
	return app
}

func newUserGroup(t *testing.T, name, description, origin string) types.UserGroup {
	userGroup, err := types.NewUserGroup(types.Metadata{
		Name:        name,
		Description: description,
		Labels: map[string]string{
			types.OriginLabel: origin,
		},
	}, types.UserGroupSpecV1{})
	require.NoError(t, err)
	return userGroup
}

func newResourceID(clusterName, kind, name string) types.ResourceID {
	return types.ResourceID{
		ClusterName: clusterName,
		Kind:        kind,
		Name:        name,
	}
}
