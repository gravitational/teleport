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

// mockGetter mocks the UserAndRoleGetter interface.
type mockGetter struct {
	users        map[string]types.User
	roles        map[string]types.Role
	nodes        map[string]types.Server
	kubeServices []types.Server
	dbs          map[string]types.Database
	apps         map[string]types.Application
	desktops     map[string]types.WindowsDesktop
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

func (m *mockGetter) GetNode(ctx context.Context, namespace string, name string) (types.Server, error) {
	node, ok := m.nodes[name]
	if !ok {
		return nil, trace.NotFound("no such node: %q", name)
	}
	return node, nil
}

func (m *mockGetter) GetKubeServices(ctx context.Context) ([]types.Server, error) {
	return append([]types.Server{}, m.kubeServices...), nil
}

func (m *mockGetter) GetDatabase(ctx context.Context, name string) (types.Database, error) {
	db, ok := m.dbs[name]
	if !ok {
		return nil, trace.NotFound("no such db: %q", name)
	}
	return db, nil
}

func (m *mockGetter) GetApp(ctx context.Context, name string) (types.Application, error) {
	app, ok := m.apps[name]
	if !ok {
		return nil, trace.NotFound("no such app: %q", name)
	}
	return app, nil
}

func (m *mockGetter) GetWindowsDesktops(ctx context.Context, filter types.WindowsDesktopFilter) ([]types.WindowsDesktop, error) {
	desktop, ok := m.desktops[filter.Name]
	if !ok {
		return nil, trace.NotFound("no such desktop: %q", filter.Name)
	}
	return []types.WindowsDesktop{desktop}, nil
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
		validator, err := NewRequestValidator(context.Background(), g, tt.requestor, ExpandVars(true))
		require.NoError(t, err, "scenario=%q", tt.desc)

		require.NoError(t, validator.Validate(context.Background(), req), "scenario=%q", tt.desc)

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
	roleDesc := map[string]types.RoleSpecV5{
		"db-admins": types.RoleSpecV5{
			Allow: types.RoleConditions{
				NodeLabels: types.Labels{
					"owner": {"db-admins"},
				},
			},
		},
		"db-response-team": types.RoleSpecV5{
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{"db-admins"},
				},
			},
		},
		"deny-db-request": types.RoleSpecV5{
			Deny: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles: []string{"db-admins"},
				},
			},
		},
		"deny-db-search": types.RoleSpecV5{
			Deny: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{"db-admins"},
				},
			},
		},
		"splunk-admins": types.RoleSpecV5{
			Allow: types.RoleConditions{
				NodeLabels: types.Labels{
					"owner": {"splunk-admins"},
				},
			},
		},
		"splunk-response-team": types.RoleSpecV5{
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{"splunk-admins", "splunk-super-admins"},
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
			expectError:        trace.AccessDenied(`user does not have any "search_as_roles" which are valid for this request`),
		},
		{
			desc:               "deny request",
			currentRoles:       []string{"db-response-team", "deny-db-request"},
			requestResourceIDs: resourceIDs,
			expectError:        trace.AccessDenied(`user does not have any "search_as_roles" which are valid for this request`),
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
			expectError:        trace.AccessDenied(`user does not have any "search_as_roles" which are valid for this request`),
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
				roles: roles,
				users: users,
			}

			req, err := types.NewAccessRequestWithResources(
				"some-id", user.GetName(), tc.requestRoles, tc.requestResourceIDs)
			require.NoError(t, err)

			validator, err := NewRequestValidator(context.Background(), g, user.GetName(), ExpandVars(true))
			require.NoError(t, err)

			err = validator.Validate(context.Background(), req)
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

	g := &mockGetter{
		roles:    make(map[string]types.Role),
		users:    make(map[string]types.User),
		nodes:    make(map[string]types.Server),
		dbs:      make(map[string]types.Database),
		apps:     make(map[string]types.Application),
		desktops: make(map[string]types.WindowsDesktop),
	}

	// set up test roles
	roleDesc := map[string]types.RoleSpecV5{
		"response-team": types.RoleSpecV5{
			// By default has access to nothing, but can request many types of
			// resources.
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{
						"node-admins",
						"node-access",
						"kube-admins",
						"db-admins",
						"app-admins",
						"windows-admins",
						"empty",
					},
				},
			},
		},
		"node-access": types.RoleSpecV5{
			// Grants access with user's own login
			Allow: types.RoleConditions{
				NodeLabels: types.Labels{
					"*": {"*"},
				},
				Logins: []string{"{{internal.logins}}"},
			},
		},
		"node-admins": types.RoleSpecV5{
			// Grants root access to specific nodes.
			Allow: types.RoleConditions{
				NodeLabels: types.Labels{
					"owner": {"node-admins"},
				},
				Logins: []string{"{{internal.logins}}", "root"},
			},
		},
		"kube-admins": types.RoleSpecV5{
			Allow: types.RoleConditions{
				KubernetesLabels: types.Labels{
					"*": {"*"},
				},
			},
		},
		"db-admins": types.RoleSpecV5{
			Allow: types.RoleConditions{
				DatabaseLabels: types.Labels{
					"*": {"*"},
				},
			},
		},
		"app-admins": types.RoleSpecV5{
			Allow: types.RoleConditions{
				AppLabels: types.Labels{
					"*": {"*"},
				},
			},
		},
		"windows-admins": types.RoleSpecV5{
			Allow: types.RoleConditions{
				WindowsDesktopLabels: types.Labels{
					"*": {"*"},
				},
			},
		},
		"empty": types.RoleSpecV5{
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
		"logins": []string{"responder"},
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
			name: "denied-node",
		},
	}
	for _, desc := range nodeDesc {
		node, err := types.NewServerWithLabels(desc.name, types.KindNode, types.ServerSpecV2{}, desc.labels)
		require.NoError(t, err)
		g.nodes[desc.name] = node
	}

	kube, err := types.NewServerWithLabels("kube", types.KindKubeService, types.ServerSpecV2{
		KubernetesClusters: []*types.KubernetesCluster{
			&types.KubernetesCluster{
				Name:         "kube",
				StaticLabels: nil,
			},
		},
	}, nil)
	require.NoError(t, err)
	g.kubeServices = append(g.kubeServices, kube)

	db, err := types.NewDatabaseV3(types.Metadata{
		Name: "db",
	}, types.DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "example.com:3000",
	})
	require.NoError(t, err)
	g.dbs[db.GetName()] = db

	app, err := types.NewAppV3(types.Metadata{
		Name: "app",
	}, types.AppSpecV3{
		URI: "example.com:3000",
	})
	require.NoError(t, err)
	g.apps[app.GetName()] = app

	desktop, err := types.NewWindowsDesktopV3("windows", nil, types.WindowsDesktopSpecV3{
		Addr: "example.com:3001",
	})
	require.NoError(t, err)
	g.desktops[desktop.GetName()] = desktop

	clusterName := "my-cluster"

	testCases := []struct {
		desc               string
		requestResourceIDs []types.ResourceID
		loginHint          string
		userTraits         map[string][]string
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
			expectRoles: []string{"node-access", "node-admins", "kube-admins", "db-admins", "app-admins", "windows-admins", "empty"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			req, err := NewAccessRequestWithResources(user, nil, tc.requestResourceIDs)
			require.NoError(t, err)

			req.SetLoginHint(tc.loginHint)

			err = ValidateAccessRequestForUser(ctx, g, req, ExpandVars(true))
			require.NoError(t, err)

			err = PruneResourceRequestRoles(ctx, req, g, clusterName, g.users[user].GetTraits())
			if tc.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			require.ElementsMatch(t, tc.expectRoles, req.GetRoles(),
				"Pruned roles %v don't match expected roles %v", req.GetRoles(), tc.expectRoles)
		})
	}
}
