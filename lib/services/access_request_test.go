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
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/fixtures"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// mockUserAndRoleGetter mocks the UserAndRoleGetter interface.
type mockUserAndRoleGetter struct {
	users map[string]types.User
	roles map[string]types.Role
}

// user inserts a new user with the specified roles and returns the username.
func (m *mockUserAndRoleGetter) user(t *testing.T, roles ...string) string {
	name := uuid.New().String()
	user, err := types.NewUser(name)
	require.NoError(t, err)

	user.SetRoles(roles)
	m.users[name] = user
	return name
}

func (m *mockUserAndRoleGetter) GetUser(name string, withSecrets bool) (types.User, error) {
	if withSecrets {
		return nil, trace.NotImplemented("mock getter does not store secrets")
	}
	user, ok := m.users[name]
	if !ok {
		return nil, trace.NotFound("no such user: %q", name)
	}
	return user, nil
}

func (m *mockUserAndRoleGetter) GetRole(ctx context.Context, name string) (types.Role, error) {
	role, ok := m.roles[name]
	if !ok {
		return nil, trace.NotFound("no such role: %q", name)
	}
	return role, nil
}

func (m *mockUserAndRoleGetter) GetRoles(ctx context.Context) ([]types.Role, error) {
	roles := make([]types.Role, 0, len(m.roles))
	for _, r := range m.roles {
		roles = append(roles, r)
	}
	return roles, nil
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
	}

	roles := make(map[string]types.Role)

	for name, conditions := range roleDesc {
		role, err := types.NewRole(name, types.RoleSpecV5{
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

	g := &mockUserAndRoleGetter{
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
				{ // 1 of 2 required denials (does not triger threshold)
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
				{ // 1 of 2 required denials for "coup" (does not triger threshold)
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

		// perform request validation (necessary in order to initialize internal
		// request variables like annotations and thresholds).
		validator, err := NewRequestValidator(g, tt.requestor, ExpandVars(true))
		require.NoError(t, err, "scenario=%q", tt.desc)

		require.NoError(t, validator.Validate(req), "scenario=%q", tt.desc)

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
						"teams": []string{"staging-admin"},
					},
				},
				Review: reviewParamsContext{
					Reason: "ok",
					Annotations: map[string][]string{
						"constraints": []string{"no-admin"},
					},
				},
				Request: reviewRequestContext{
					Roles:  []string{"dev"},
					Reason: "plz",
					SystemAnnotations: map[string][]string{
						"teams": []string{"staging-dev"},
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

	require.True(t, cmp.Equal(req1, req2))
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
