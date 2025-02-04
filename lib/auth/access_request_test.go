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

package auth

import (
	"cmp"
	"context"
	"crypto/tls"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/tlsca"
)

type accessRequestTestPack struct {
	tlsServer   *TestTLSServer
	clusterName string
	roles       map[string]types.RoleSpecV6
	users       map[string][]string
	privKey     []byte
	pubKey      []byte
}

func newAccessRequestTestPack(ctx context.Context, t *testing.T) *accessRequestTestPack {
	testAuthServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir: t.TempDir(),
	})
	require.NoError(t, err, "%s", trace.DebugReport(err))
	t.Cleanup(func() { require.NoError(t, testAuthServer.Close()) })

	tlsServer, err := testAuthServer.NewTestTLSServer()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, tlsServer.Close()) })

	clusterName, err := tlsServer.Auth().GetClusterName()
	require.NoError(t, err)

	roles := map[string]types.RoleSpecV6{
		// superadmins have access to all nodes
		"superadmins": {
			Allow: types.RoleConditions{
				Logins: []string{"root"},
				NodeLabels: types.Labels{
					"*": []string{"*"},
				},
			},
		},
		// admins have access to nodes in prod and staging
		"admins": {
			Allow: types.RoleConditions{
				Logins: []string{"root"},
				NodeLabels: types.Labels{
					"env": []string{"prod", "staging"},
				},
				ReviewRequests: &types.AccessReviewConditions{
					Roles: []string{"admins"},
				},
			},
		},
		// operators can request the admins role
		"operators": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles: []string{"admins"},
				},
			},
		},
		// responders can request the admins role but only with a
		// resource request limited to specific resources
		"responders": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{"admins"},
				},
			},
		},
		// requesters can request everything possible
		"requesters": {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles:         []string{"admins", "superadmins"},
					SearchAsRoles: []string{"admins", "superadmins"},
					MaxDuration:   types.Duration(services.MaxAccessDuration),
				},
			},
		},
		"empty": {},
	}
	for roleName, roleSpec := range roles {
		role, err := types.NewRole(roleName, roleSpec)
		require.NoError(t, err)

		_, err = tlsServer.Auth().UpsertRole(ctx, role)
		require.NoError(t, err)
	}

	users := map[string][]string{
		"admin":     {"admins"},
		"responder": {"responders"},
		"operator":  {"operators"},
		"requester": {"requesters"},
		"nobody":    {"empty"},
	}
	for name, roles := range users {
		user, err := types.NewUser(name)
		require.NoError(t, err)
		user.SetRoles(roles)
		_, err = tlsServer.Auth().UpsertUser(ctx, user)
		require.NoError(t, err)
	}

	type nodeDesc struct {
		labels map[string]string
	}
	nodes := map[string]nodeDesc{
		"staging": {
			labels: map[string]string{
				"env": "staging",
			},
		},
		"prod": {
			labels: map[string]string{
				"env": "prod",
			},
		},
	}
	for nodeName, desc := range nodes {
		node, err := types.NewServerWithLabels(nodeName, types.KindNode, types.ServerSpecV2{}, desc.labels)
		require.NoError(t, err)
		_, err = tlsServer.Auth().UpsertNode(context.Background(), node)
		require.NoError(t, err)
	}

	privKey, pubKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	return &accessRequestTestPack{
		tlsServer:   tlsServer,
		clusterName: clusterName.GetClusterName(),
		roles:       roles,
		users:       users,
		privKey:     privKey,
		pubKey:      pubKey,
	}
}

func TestAccessRequest(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	testPack := newAccessRequestTestPack(ctx, t)
	t.Run("single", func(t *testing.T) { testSingleAccessRequests(t, testPack) })
	t.Run("multi", func(t *testing.T) { testMultiAccessRequests(t, testPack) })
	t.Run("role refresh with bogus request ID", func(t *testing.T) { testRoleRefreshWithBogusRequestID(t, testPack) })
	t.Run("bot user approver", func(t *testing.T) { testBotAccessRequestReview(t, testPack) })
	t.Run("deny", func(t *testing.T) { testAccessRequestDenyRules(t, testPack) })
}

// waitForAccessRequests is a helper for writing access request tests that need to wait for access request CRUD. the supplied condition is
// repeatedly called with the contents of the access request cache until it returns true or a reasonably long timeout is exceeded. this is
// similar to require.Eventually except that it is safe to use normal (test-failing) assertions within the supplied condition closure.
func waitForAccessRequests(t *testing.T, ctx context.Context, getter services.AccessRequestGetter, condition func([]*types.AccessRequestV3) bool) {
	t.Helper()

	timeout := time.After(time.Second * 30)
	for {
		var reqs []*types.AccessRequestV3
		var nextKey string
	Paginate:
		for {
			rsp, err := getter.ListAccessRequests(ctx, &proto.ListAccessRequestsRequest{
				Limit:    1_000,
				StartKey: nextKey,
			})
			require.NoError(t, err, "ListAccessRequests API call should succeed")

			reqs = append(reqs, rsp.AccessRequests...)
			nextKey = rsp.NextKey
			if nextKey == "" {
				break Paginate
			}
		}

		if condition(reqs) {
			return
		}

		select {
		case <-time.After(time.Millisecond * 150):
		case <-timeout:
			require.FailNow(t, "timeout waiting for access request condition to pass")
		}
	}
}

// TestListAccessRequests tests some basic functionality of the ListAccessRequests API, including access-control,
// filtering, sort, and pagination.
func TestListAccessRequests(t *testing.T) {
	const (
		requestsPerUser = 200
		pageSize        = 7
	)

	t.Parallel()

	clock := clockwork.NewFakeClock()

	authServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clock,
	})
	require.NoError(t, err)
	defer authServer.Close()

	tlsServer, err := authServer.NewTestTLSServer()
	require.NoError(t, err)
	defer tlsServer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	userA, userB := "lister-a", "lister-b"
	roleA, roleB := userA+"-role", userB+"-role"
	rroleA, rroleB := userA+"-rrole", userB+"-rrole"

	roles := map[string]types.RoleSpecV6{
		roleA: {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles: []string{rroleA},
				},
			},
		},
		roleB: {
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles: []string{rroleB},
				},
			},
		},
		rroleA: {},
		rroleB: {},
	}

	for roleName, roleSpec := range roles {
		role, err := types.NewRole(roleName, roleSpec)
		require.NoError(t, err)
		_, err = tlsServer.Auth().UpsertRole(ctx, role)
		require.NoError(t, err)
	}

	for userName, roleName := range map[string]string{userA: roleA, userB: roleB} {
		user, err := types.NewUser(userName)
		require.NoError(t, err)
		user.SetRoles([]string{roleName})
		_, err = tlsServer.Auth().UpsertUser(ctx, user)
		require.NoError(t, err)
	}

	clientA, err := tlsServer.NewClient(TestUser(userA))
	require.NoError(t, err)
	defer clientA.Close()

	clientB, err := tlsServer.NewClient(TestUser(userB))
	require.NoError(t, err)
	defer clientB.Close()

	// orderedIDs is a list of all access request IDs in order of creation (used to
	// verify sort order).
	var orderedIDs []string

	for i := 0; i < requestsPerUser; i++ {
		clock.Advance(time.Second)
		reqA, err := services.NewAccessRequest(userA, rroleA)
		require.NoError(t, err)
		rr, err := clientA.CreateAccessRequestV2(ctx, reqA)
		require.NoError(t, err)
		orderedIDs = append(orderedIDs, rr.GetName())
	}

	for i := 0; i < requestsPerUser; i++ {
		clock.Advance(time.Second)
		reqB, err := services.NewAccessRequest(userB, rroleB)
		require.NoError(t, err)
		rr, err := clientB.CreateAccessRequestV2(ctx, reqB)
		require.NoError(t, err)
		orderedIDs = append(orderedIDs, rr.GetName())
	}

	// wait for all written requests to propagate to cache
	waitForAccessRequests(t, ctx, tlsServer.Auth(), func(reqs []*types.AccessRequestV3) bool {
		return len(reqs) == len(orderedIDs)
	})

	var reqs []*types.AccessRequestV3
	var observedIDs []string
	var nextKey string
	for {
		rsp, err := tlsServer.Auth().ListAccessRequests(ctx, &proto.ListAccessRequestsRequest{
			Limit:    pageSize,
			StartKey: nextKey,
			Sort:     proto.AccessRequestSort_CREATED,
		})
		require.NoError(t, err)

		for _, r := range rsp.AccessRequests {
			observedIDs = append(observedIDs, r.GetName())
		}

		reqs = append(reqs, rsp.AccessRequests...)

		nextKey = rsp.NextKey
		if nextKey == "" {
			break
		}

		require.Len(t, rsp.AccessRequests, pageSize)
	}

	// verify that we observed the requests in the same order that they were created (i.e. that the
	// default sort order is ascending and time-based).
	require.Equal(t, orderedIDs, observedIDs)

	// verify that time-based sorting can be checked via creation time field as expected (relied upon later)
	require.True(t, slices.IsSortedFunc(reqs, func(a, b *types.AccessRequestV3) int {
		return a.GetCreationTime().Compare(b.GetCreationTime())
	}))

	reqs = nil
	nextKey = ""
	for {
		rsp, err := tlsServer.Auth().ListAccessRequests(ctx, &proto.ListAccessRequestsRequest{
			Filter: &types.AccessRequestFilter{
				User: userB,
			},
			Limit:    pageSize,
			StartKey: nextKey,
		})
		require.NoError(t, err)

		reqs = append(reqs, rsp.AccessRequests...)

		nextKey = rsp.NextKey
		if nextKey == "" {
			break
		}

		require.Len(t, rsp.AccessRequests, pageSize)
	}
	require.Len(t, reqs, requestsPerUser)

	// verify that access-control filtering is applied and exercise a different combination of sort params
	for _, clt := range []authclient.ClientI{clientA, clientB} {
		reqs = nil
		nextKey = ""
		for {
			rsp, err := clt.ListAccessRequests(ctx, &proto.ListAccessRequestsRequest{
				Limit:      pageSize,
				StartKey:   nextKey,
				Sort:       proto.AccessRequestSort_CREATED,
				Descending: true,
			})
			require.NoError(t, err)

			reqs = append(reqs, rsp.AccessRequests...)

			nextKey = rsp.NextKey
			if nextKey == "" {
				break
			}

			require.Len(t, rsp.AccessRequests, pageSize)
		}

		require.Len(t, reqs, requestsPerUser)
		require.True(t, slices.IsSortedFunc(reqs, func(a, b *types.AccessRequestV3) int {
			// note that we flip `a` and `b` to assert Descending ordering.
			return b.GetCreationTime().Compare(a.GetCreationTime())
		}))
	}

	// set requests to a variety of states so that state-based ordering
	// is distinctly different from time-based ordering.
	var deny bool
	expectStates := make(map[string]types.RequestState)
	for i, id := range observedIDs {
		if i%2 == 0 {
			// leave half the requests as pending
			continue
		}
		state := types.RequestState_APPROVED
		if deny {
			state = types.RequestState_DENIED
		}
		deny = !deny // toggle next target state

		require.NoError(t, tlsServer.Auth().SetAccessRequestState(ctx, types.AccessRequestUpdate{
			RequestID: id,
			State:     state,
		}))
		expectStates[id] = state
	}

	// wait until all requests in cache to present the expected state
	waitForAccessRequests(t, ctx, tlsServer.Auth(), func(reqs []*types.AccessRequestV3) bool {
		for _, r := range reqs {
			if expected, ok := expectStates[r.GetName()]; ok && r.GetState() != expected {
				return false
			}
		}
		return true
	})

	// aggregate requests by descending state ordering
	reqs = nil
	nextKey = ""
	for {
		rsp, err := clientA.ListAccessRequests(ctx, &proto.ListAccessRequestsRequest{
			Sort:       proto.AccessRequestSort_STATE,
			Descending: true,
			Limit:      pageSize,
			StartKey:   nextKey,
		})
		require.NoError(t, err)

		reqs = append(reqs, rsp.AccessRequests...)

		nextKey = rsp.NextKey
		if nextKey == "" {
			break
		}

		require.Len(t, rsp.AccessRequests, pageSize)
	}

	require.Len(t, reqs, requestsPerUser)

	// verify that requests are sorted by state
	require.True(t, slices.IsSortedFunc(reqs, func(a, b *types.AccessRequestV3) int {
		// state sort index sorts by the string representation of state. note that we
		// flip `a` and `b` to assert Descending ordering.
		return cmp.Compare(b.GetState().String(), a.GetState().String())
	}))

	// sanity-check to ensure that we did force a custom state ordering
	require.False(t, slices.IsSortedFunc(reqs, func(a, b *types.AccessRequestV3) int {
		return b.GetCreationTime().Compare(a.GetCreationTime())
	}))
}

func testAccessRequestDenyRules(t *testing.T, testPack *accessRequestTestPack) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	userName := "denied"

	accessRequest, err := services.NewAccessRequest(userName, "admins")
	require.NoError(t, err)

	for _, tc := range []struct {
		desc               string
		roles              map[string]types.RoleSpecV6
		expectGetDenied    bool
		expectCreateDenied bool
	}{
		{
			desc: "all allowed",
			roles: map[string]types.RoleSpecV6{
				"allow": {
					Allow: types.RoleConditions{
						Request: &types.AccessRequestConditions{
							Roles: []string{"admins"},
						},
						ReviewRequests: &types.AccessReviewConditions{
							Roles: []string{"admins"},
						},
					},
				},
			},
		},
		{
			desc: "all denied",
			roles: map[string]types.RoleSpecV6{
				"allow": {
					Allow: types.RoleConditions{
						Request: &types.AccessRequestConditions{
							Roles: []string{"admins"},
						},
						ReviewRequests: &types.AccessReviewConditions{
							Roles: []string{"admins"},
						},
					},
				},
				"deny": {
					Deny: types.RoleConditions{
						Rules: []types.Rule{
							{
								Resources: []string{"access_request"},
								Verbs:     []string{"read", "create", "list"},
							},
						},
					},
				},
			},
			expectGetDenied:    true,
			expectCreateDenied: true,
		},
		{
			desc: "create denied",
			roles: map[string]types.RoleSpecV6{
				"allow": {
					Allow: types.RoleConditions{
						Request: &types.AccessRequestConditions{
							Roles: []string{"admins"},
						},
						ReviewRequests: &types.AccessReviewConditions{
							Roles: []string{"admins"},
						},
					},
				},
				"deny": {
					Deny: types.RoleConditions{
						Rules: []types.Rule{
							{
								Resources: []string{"access_request"},
								Verbs:     []string{"create"},
							},
						},
					},
				},
			},
			expectCreateDenied: true,
		},
		{
			desc: "get denied",
			roles: map[string]types.RoleSpecV6{
				"allow": {
					Allow: types.RoleConditions{
						Request: &types.AccessRequestConditions{
							Roles: []string{"admins"},
						},
						ReviewRequests: &types.AccessReviewConditions{
							Roles: []string{"admins"},
						},
					},
				},
				"deny": {
					Deny: types.RoleConditions{
						Rules: []types.Rule{
							{
								Resources: []string{"access_request"},
								Verbs:     []string{"read"},
							},
						},
					},
				},
			},
			expectGetDenied: true,
		},
		{
			desc: "list denied",
			roles: map[string]types.RoleSpecV6{
				"allow": {
					Allow: types.RoleConditions{
						Request: &types.AccessRequestConditions{
							Roles: []string{"admins"},
						},
						ReviewRequests: &types.AccessReviewConditions{
							Roles: []string{"admins"},
						},
					},
				},
				"deny": {
					Deny: types.RoleConditions{
						Rules: []types.Rule{
							{
								Resources: []string{"access_request"},
								Verbs:     []string{"list"},
							},
						},
					},
				},
			},
			expectGetDenied: true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			for roleName, roleSpec := range tc.roles {
				role, err := types.NewRole(roleName, roleSpec)
				require.NoError(t, err)
				_, err = testPack.tlsServer.Auth().UpsertRole(ctx, role)
				require.NoError(t, err)
			}
			user, err := types.NewUser(userName)
			require.NoError(t, err)
			user.SetRoles(maps.Keys(tc.roles))
			_, err = testPack.tlsServer.Auth().UpsertUser(ctx, user)
			require.NoError(t, err)

			client, err := testPack.tlsServer.NewClient(TestUser(userName))
			require.NoError(t, err)

			_, err = client.GetAccessRequests(ctx, types.AccessRequestFilter{})
			if tc.expectGetDenied {
				assert.True(t, trace.IsAccessDenied(err), "want access denied, got %v", err)
			} else {
				assert.NoError(t, err)
			}

			_, err = client.CreateAccessRequestV2(ctx, accessRequest)
			if tc.expectCreateDenied {
				assert.True(t, trace.IsAccessDenied(err), "want access denied, got %v", err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func testSingleAccessRequests(t *testing.T, testPack *accessRequestTestPack) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	testCases := []struct {
		desc                   string
		requester              string
		reviewer               string
		expectRequestableRoles []string
		requestRoles           []string
		expectRoles            []string
		expectNodes            []string
		requestResources       []string
		expectRequestError     error
		expectReviewError      error
	}{
		{
			desc:                   "role request",
			requester:              "operator",
			reviewer:               "admin",
			expectRequestableRoles: []string{"admins"},
			requestRoles:           []string{"admins"},
			expectRoles:            []string{"operators", "admins"},
			expectNodes:            []string{"prod", "staging"},
		},
		{
			desc:               "no requestable roles",
			requester:          "nobody",
			requestRoles:       []string{"admins"},
			expectRequestError: trace.BadParameter(`user "nobody" can not request role "admins"`),
		},
		{
			desc:                   "role not allowed",
			requester:              "operator",
			expectRequestableRoles: []string{"admins"},
			requestRoles:           []string{"super-admins"},
			expectRequestError:     trace.BadParameter(`user "operator" can not request role "super-admins"`),
		},
		{
			desc:                   "review own request",
			requester:              "operator",
			reviewer:               "operator",
			expectRequestableRoles: []string{"admins"},
			requestRoles:           []string{"admins"},
			expectReviewError:      trace.AccessDenied(`user "operator" cannot submit reviews`),
		},
		{
			desc:             "resource request for staging",
			requester:        "responder",
			reviewer:         "admin",
			requestResources: []string{"staging"},
			expectRoles:      []string{"responders", "admins"},
			expectNodes:      []string{"staging"},
		},
		{
			desc:             "resource request for prod",
			requester:        "responder",
			reviewer:         "admin",
			requestResources: []string{"prod"},
			expectRoles:      []string{"responders", "admins"},
			expectNodes:      []string{"prod"},
		},
		{
			desc:             "resource request for both",
			requester:        "responder",
			reviewer:         "admin",
			requestResources: []string{"prod", "staging"},
			expectRoles:      []string{"responders", "admins"},
			expectNodes:      []string{"prod", "staging"},
		},
		{
			desc:               "no search_as_roles",
			requester:          "nobody",
			requestResources:   []string{"prod"},
			expectRequestError: trace.AccessDenied(`Resource Access Requests require usable "search_as_roles", none found for user "nobody"`),
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			requester := TestUser(tc.requester)
			requesterClient, err := testPack.tlsServer.NewClient(requester)
			require.NoError(t, err)

			// generateCerts executes a GenerateUserCerts request, optionally applying
			// one or more access-requests to the certificate.
			generateCerts := func(reqIDs ...string) (*proto.Certs, error) {
				return requesterClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
					PublicKey:      testPack.pubKey,
					Username:       tc.requester,
					Expires:        time.Now().Add(time.Hour).UTC(),
					Format:         constants.CertificateFormatStandard,
					AccessRequests: reqIDs,
				})
			}

			// sanity check we can get certs with no access request
			certs, err := generateCerts()
			require.NoError(t, err)

			// should have no logins, requests, or resources in ssh cert
			checkCerts(t, certs, testPack.users[tc.requester], nil, nil, nil)

			// should not be able to list any nodes
			nodes, err := requesterClient.GetNodes(ctx, defaults.Namespace)
			require.NoError(t, err)
			require.Empty(t, nodes)

			// requestable roles should be correct
			caps, err := requesterClient.GetAccessCapabilities(ctx, types.AccessCapabilitiesRequest{
				RequestableRoles: true,
			})
			require.NoError(t, err)
			require.Equal(t, tc.expectRequestableRoles, caps.RequestableRoles)

			// create the access request object
			requestResourceIDs := []types.ResourceID{}
			for _, nodeName := range tc.requestResources {
				requestResourceIDs = append(requestResourceIDs, types.ResourceID{
					ClusterName: testPack.clusterName,
					Kind:        types.KindNode,
					Name:        nodeName,
				})
			}
			req, err := services.NewAccessRequestWithResources(tc.requester, tc.requestRoles, requestResourceIDs)
			require.NoError(t, err)

			// send the request to the auth server
			req, err = requesterClient.CreateAccessRequestV2(ctx, req)
			require.ErrorIs(t, err, tc.expectRequestError)
			if tc.expectRequestError != nil {
				return
			}

			// try logging in with request in PENDING state (should fail)
			_, err = generateCerts(req.GetName())
			require.ErrorIs(t, err, trace.AccessDenied("access request %q is awaiting approval", req.GetName()))

			reviewer := TestUser(tc.reviewer)
			reviewerClient, err := testPack.tlsServer.NewClient(reviewer)
			require.NoError(t, err)

			// approve the request
			req, err = reviewerClient.SubmitAccessReview(ctx, types.AccessReviewSubmission{
				RequestID: req.GetName(),
				Review: types.AccessReview{
					ProposedState: types.RequestState_APPROVED,
				},
			})
			require.ErrorIs(t, err, tc.expectReviewError)
			if tc.expectReviewError != nil {
				return
			}
			require.NoError(t, reviewerClient.Close())
			require.Equal(t, types.RequestState_APPROVED, req.GetState())

			// log in now that request has been approved
			certs, err = generateCerts(req.GetName())
			require.NoError(t, err)

			// cert should have login from requested role, the access request
			// should be in the cert, and any requested resources should be
			// encoded in the cert
			checkCerts(t,
				certs,
				tc.expectRoles,
				[]string{"root"},
				[]string{req.GetName()},
				requestResourceIDs)

			elevatedCert, err := tls.X509KeyPair(certs.TLS, testPack.privKey)
			require.NoError(t, err)
			elevatedClient := testPack.tlsServer.NewClientWithCert(elevatedCert)

			// should be able to list the expected nodes
			nodes, err = elevatedClient.GetNodes(ctx, defaults.Namespace)
			require.NoError(t, err)
			gotNodes := []string{}
			for _, node := range nodes {
				gotNodes = append(gotNodes, node.GetName())
			}
			sort.Strings(gotNodes)
			require.Equal(t, tc.expectNodes, gotNodes)

			// renew elevated certs
			newCerts, err := elevatedClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
				PublicKey: testPack.pubKey,
				Username:  tc.requester,
				Expires:   time.Now().Add(time.Hour).UTC(),
				// no new access requests
				AccessRequests: nil,
			})
			require.NoError(t, err)

			// in spite of providing no access requests, we still have elevated
			// roles and the certicate shows the original access request
			checkCerts(t,
				newCerts,
				tc.expectRoles,
				[]string{"root"},
				[]string{req.GetName()},
				requestResourceIDs)

			// attempt to apply request in DENIED state (should fail)
			require.NoError(t, testPack.tlsServer.Auth().SetAccessRequestState(ctx, types.AccessRequestUpdate{
				RequestID: req.GetName(),
				State:     types.RequestState_DENIED,
			}))
			_, err = generateCerts(req.GetName())
			require.ErrorIs(t, err, trace.AccessDenied("access request %q has been denied", req.GetName()))

			// ensure that once in the DENIED state, a request cannot be set back to PENDING state.
			require.Error(t, testPack.tlsServer.Auth().SetAccessRequestState(ctx, types.AccessRequestUpdate{
				RequestID: req.GetName(),
				State:     types.RequestState_PENDING,
			}))

			// ensure that once in the DENIED state, a request cannot be set back to APPROVED state.
			require.Error(t, testPack.tlsServer.Auth().SetAccessRequestState(ctx, types.AccessRequestUpdate{
				RequestID: req.GetName(),
				State:     types.RequestState_APPROVED,
			}))

			// ensure that identities with requests in the DENIED state can't reissue new certs.
			_, err = elevatedClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
				PublicKey: testPack.pubKey,
				Username:  tc.requester,
				Expires:   time.Now().Add(time.Hour).UTC(),
				// no new access requests
				AccessRequests: nil,
			})
			require.ErrorIs(t, err, trace.AccessDenied("access request %q has been denied", req.GetName()))
		})
	}
}

// testBotAccessRequestReview specifically ensures that a bots output cert
// can be used to review a access request. This is because there's a special
// case to handle their role impersonated certs correctly.
func testBotAccessRequestReview(t *testing.T, testPack *accessRequestTestPack) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Create the bot
	adminClient, err := testPack.tlsServer.NewClient(TestAdmin())
	require.NoError(t, err)
	defer adminClient.Close()
	bot, err := adminClient.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: &machineidv1pb.Bot{
			Metadata: &headerv1.Metadata{
				Name: "request-approver",
			},
			Spec: &machineidv1pb.BotSpec{
				Roles: []string{
					// Grants the ability to approve requests
					"admins",
				},
			},
		},
	})
	require.NoError(t, err)

	// Use the bot user to generate some certs using role impersonation.
	// This mimics what the bot actually does.
	botClient, err := testPack.tlsServer.NewClient(TestUser(bot.Status.UserName))
	require.NoError(t, err)
	defer botClient.Close()
	certRes, err := botClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
		Username:  bot.Status.UserName,
		PublicKey: testPack.pubKey,
		Expires:   time.Now().Add(time.Hour),

		RoleRequests:    []string{"admins"},
		UseRoleRequests: true,
	})
	require.NoError(t, err)
	tlsCert, err := tls.X509KeyPair(certRes.TLS, testPack.privKey)
	require.NoError(t, err)
	impersonatedBotClient := testPack.tlsServer.NewClientWithCert(tlsCert)
	defer impersonatedBotClient.Close()

	// Create an access request for the bot to approve
	requesterClient, err := testPack.tlsServer.NewClient(TestUser("requester"))
	require.NoError(t, err)
	defer requesterClient.Close()
	accessRequest, err := services.NewAccessRequest("requester", "admins")
	require.NoError(t, err)
	accessRequest, err = requesterClient.CreateAccessRequestV2(ctx, accessRequest)
	require.NoError(t, err)

	// Approve the access request with the bot
	accessRequest, err = impersonatedBotClient.SubmitAccessReview(ctx, types.AccessReviewSubmission{
		RequestID: accessRequest.GetName(),
		Review: types.AccessReview{
			ProposedState: types.RequestState_APPROVED,
		},
	})
	require.NoError(t, err)

	// Check the final state of the request
	require.Equal(t, bot.Status.UserName, accessRequest.GetReviews()[0].Author)
	require.Equal(t, types.RequestState_APPROVED, accessRequest.GetState())
}

func testMultiAccessRequests(t *testing.T, testPack *accessRequestTestPack) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	username := "requester"

	prodResourceIDs := []types.ResourceID{{
		ClusterName: testPack.clusterName,
		Kind:        types.KindNode,
		Name:        "prod",
	}}
	prodResourceRequest, err := services.NewAccessRequestWithResources(username, []string{"admins"}, prodResourceIDs)
	require.NoError(t, err)

	stagingResourceIDs := []types.ResourceID{{
		ClusterName: testPack.clusterName,
		Kind:        types.KindNode,
		Name:        "staging",
	}}
	stagingResourceRequest, err := services.NewAccessRequestWithResources(username, []string{"admins"}, stagingResourceIDs)
	require.NoError(t, err)

	adminRequest, err := services.NewAccessRequest(username, "admins")
	require.NoError(t, err)

	superAdminRequest, err := services.NewAccessRequest(username, "superadmins")
	require.NoError(t, err)

	bothRolesRequest, err := services.NewAccessRequest(username, "admins", "superadmins")
	require.NoError(t, err)

	for _, request := range []types.AccessRequest{
		prodResourceRequest,
		stagingResourceRequest,
		adminRequest,
		superAdminRequest,
		bothRolesRequest,
	} {
		request.SetState(types.RequestState_APPROVED)
		request.SetAccessExpiry(time.Now().Add(time.Hour).UTC())
		require.NoError(t, testPack.tlsServer.Auth().UpsertAccessRequest(ctx, request))
	}

	requester := TestUser(username)
	requesterClient, err := testPack.tlsServer.NewClient(requester)
	require.NoError(t, err)

	type newClientFunc func(*testing.T, *authclient.Client, *proto.Certs) (*authclient.Client, *proto.Certs)
	updateClientWithNewAndDroppedRequests := func(newRequests, dropRequests []string) newClientFunc {
		return func(t *testing.T, clt *authclient.Client, _ *proto.Certs) (*authclient.Client, *proto.Certs) {
			certs, err := clt.GenerateUserCerts(ctx, proto.UserCertsRequest{
				PublicKey:          testPack.pubKey,
				Username:           username,
				Expires:            time.Now().Add(time.Hour).UTC(),
				AccessRequests:     newRequests,
				DropAccessRequests: dropRequests,
			})
			require.NoError(t, err)
			tlsCert, err := tls.X509KeyPair(certs.TLS, testPack.privKey)
			require.NoError(t, err)
			return testPack.tlsServer.NewClientWithCert(tlsCert), certs
		}
	}
	applyAccessRequests := func(newRequests ...string) newClientFunc {
		return updateClientWithNewAndDroppedRequests(newRequests, nil)
	}
	dropAccessRequests := func(dropRequests ...string) newClientFunc {
		return updateClientWithNewAndDroppedRequests(nil, dropRequests)
	}
	failToApplyAccessRequests := func(reqs ...string) newClientFunc {
		return func(t *testing.T, clt *authclient.Client, certs *proto.Certs) (*authclient.Client, *proto.Certs) {
			// assert that this request fails
			_, err := clt.GenerateUserCerts(ctx, proto.UserCertsRequest{
				PublicKey:      testPack.pubKey,
				Username:       username,
				Expires:        time.Now().Add(time.Hour).UTC(),
				AccessRequests: reqs,
			})
			assert.Error(t, err)
			// return original client and certs unchanged
			return clt, certs
		}
	}

	for _, tc := range []struct {
		desc                 string
		steps                []newClientFunc
		expectRoles          []string
		expectResources      []types.ResourceID
		expectAccessRequests []string
		expectLogins         []string
	}{
		{
			desc: "multi role requests",
			steps: []newClientFunc{
				applyAccessRequests(adminRequest.GetName()),
				applyAccessRequests(superAdminRequest.GetName()),
			},
			expectRoles:          []string{"requesters", "admins", "superadmins"},
			expectLogins:         []string{"root"},
			expectAccessRequests: []string{adminRequest.GetName(), superAdminRequest.GetName()},
		},
		{
			desc: "multi resource requests",
			steps: []newClientFunc{
				applyAccessRequests(prodResourceRequest.GetName()),
				failToApplyAccessRequests(stagingResourceRequest.GetName()),
				failToApplyAccessRequests(prodResourceRequest.GetName(), stagingResourceRequest.GetName()),
			},
			expectRoles:          []string{"requesters", "admins"},
			expectLogins:         []string{"root"},
			expectResources:      prodResourceIDs,
			expectAccessRequests: []string{prodResourceRequest.GetName()},
		},
		{
			desc: "role then resource",
			steps: []newClientFunc{
				applyAccessRequests(adminRequest.GetName()),
				applyAccessRequests(prodResourceRequest.GetName()),
			},
			expectRoles:          []string{"requesters", "admins"},
			expectLogins:         []string{"root"},
			expectResources:      prodResourceIDs,
			expectAccessRequests: []string{adminRequest.GetName(), prodResourceRequest.GetName()},
		},
		{
			desc: "resource then role",
			steps: []newClientFunc{
				applyAccessRequests(prodResourceRequest.GetName()),
				applyAccessRequests(adminRequest.GetName()),
			},
			expectRoles:          []string{"requesters", "admins"},
			expectLogins:         []string{"root"},
			expectResources:      prodResourceIDs,
			expectAccessRequests: []string{adminRequest.GetName(), prodResourceRequest.GetName()},
		},
		{
			desc: "combined",
			steps: []newClientFunc{
				failToApplyAccessRequests(stagingResourceRequest.GetName(), prodResourceRequest.GetName()),
				applyAccessRequests(prodResourceRequest.GetName(), adminRequest.GetName()),
				failToApplyAccessRequests(stagingResourceRequest.GetName(), superAdminRequest.GetName()),
				applyAccessRequests(superAdminRequest.GetName()),
			},
			expectRoles:          []string{"requesters", "admins", "superadmins"},
			expectLogins:         []string{"root"},
			expectResources:      prodResourceIDs,
			expectAccessRequests: []string{adminRequest.GetName(), prodResourceRequest.GetName(), superAdminRequest.GetName()},
		},
		{
			desc: "drop resource request",
			steps: []newClientFunc{
				applyAccessRequests(prodResourceRequest.GetName(), adminRequest.GetName(), superAdminRequest.GetName()),
				dropAccessRequests(prodResourceRequest.GetName()),
			},
			expectRoles:          []string{"requesters", "admins", "superadmins"},
			expectLogins:         []string{"root"},
			expectAccessRequests: []string{adminRequest.GetName(), superAdminRequest.GetName()},
		},
		{
			desc: "drop role request",
			steps: []newClientFunc{
				applyAccessRequests(prodResourceRequest.GetName(), adminRequest.GetName(), superAdminRequest.GetName()),
				dropAccessRequests(superAdminRequest.GetName()),
			},
			expectRoles:          []string{"requesters", "admins"},
			expectResources:      prodResourceIDs,
			expectLogins:         []string{"root"},
			expectAccessRequests: []string{adminRequest.GetName(), prodResourceRequest.GetName()},
		},
		{
			desc: "drop all",
			steps: []newClientFunc{
				applyAccessRequests(prodResourceRequest.GetName(), adminRequest.GetName(), superAdminRequest.GetName()),
				dropAccessRequests("*"),
			},
			expectRoles: []string{"requesters"},
		},
		{
			desc: "switch resource requests",
			steps: []newClientFunc{
				applyAccessRequests(adminRequest.GetName()),
				applyAccessRequests(prodResourceRequest.GetName()),
				failToApplyAccessRequests(stagingResourceRequest.GetName()),
				dropAccessRequests(prodResourceRequest.GetName()),
				applyAccessRequests(stagingResourceRequest.GetName()),
				failToApplyAccessRequests(prodResourceRequest.GetName()),
				dropAccessRequests(stagingResourceRequest.GetName()),
			},
			expectRoles:          []string{"requesters", "admins"},
			expectLogins:         []string{"root"},
			expectAccessRequests: []string{adminRequest.GetName()},
		},
	} {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			client := requesterClient
			var certs *proto.Certs
			for _, step := range tc.steps {
				client, certs = step(t, client, certs)
			}
			checkCerts(t, certs, tc.expectRoles, tc.expectLogins, tc.expectAccessRequests, tc.expectResources)
		})
	}
}

// testRoleRefreshWithBogusRequestID verifies that GenerateUserCerts refreshes the role list based
// on the server state, even when supplied an ID for a nonexistent access request.
//
// Teleport Connect depends on this behavior when setting up roles for Connect My Computer.
// See [teleterm.connectmycomputer.RoleSetup].
func testRoleRefreshWithBogusRequestID(t *testing.T, testPack *accessRequestTestPack) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	username := "user-for-role-refresh"
	auth := testPack.tlsServer.Auth()

	// Create a user.
	user, err := types.NewUser(username)
	require.NoError(t, err)
	user.AddRole("requesters")
	user, err = auth.UpsertUser(ctx, user)
	require.NoError(t, err)

	// Create a client with the old set of roles.
	clt, err := testPack.tlsServer.NewClient(TestUser(username))
	require.NoError(t, err)

	// Add a new role to the user on the server.
	user.AddRole("operators")
	_, err = auth.UpdateUser(ctx, user)
	require.NoError(t, err)

	certs, err := clt.GenerateUserCerts(ctx, proto.UserCertsRequest{
		PublicKey:          testPack.pubKey,
		Username:           username,
		Expires:            time.Now().Add(time.Hour).UTC(),
		DropAccessRequests: []string{"bogus-request-id"},
	})
	require.NoError(t, err)

	// Verify that the new certs issued for the old client have the new role.
	checkCerts(t, certs, []string{"requesters", "operators"}, nil, nil, nil)
}

// checkCerts checks that the ssh and tls certs include the given roles, logins,
// accessRequests, and resourceIDs
func checkCerts(t *testing.T,
	certs *proto.Certs,
	roles []string,
	logins []string,
	accessRequests []string,
	resourceIDs []types.ResourceID,
) {
	t.Helper()

	// Parse SSH cert.
	sshCert, err := sshutils.ParseCertificate(certs.SSH)
	require.NoError(t, err)
	sshIdentity, err := sshca.DecodeIdentity(sshCert)
	require.NoError(t, err)

	// Parse TLS cert.
	tlsCert, err := tlsca.ParseCertificatePEM(certs.TLS)
	require.NoError(t, err)
	tlsIdentity, err := tlsca.FromSubject(tlsCert.Subject, tlsCert.NotAfter)
	require.NoError(t, err)

	// Make sure both certs have the expected roles.
	assert.ElementsMatch(t, roles, sshIdentity.Roles)
	assert.ElementsMatch(t, roles, tlsIdentity.Groups)

	// Make sure both certs have the expected logins/principals.
	for _, certLogins := range [][]string{sshIdentity.Principals, tlsIdentity.Principals} {
		// filter out invalid logins placed in the cert
		validCertLogins := []string{}
		for _, certLogin := range certLogins {
			if !strings.HasPrefix(certLogin, "-teleport") {
				validCertLogins = append(validCertLogins, certLogin)
			}
		}
		assert.ElementsMatch(t, logins, validCertLogins)
	}

	// Make sure both certs have the expected access requests, if any.
	assert.ElementsMatch(t, accessRequests, sshIdentity.ActiveRequests)
	assert.ElementsMatch(t, accessRequests, tlsIdentity.ActiveRequests)

	// Make sure both certs have the expected allowed resources, if any.
	sshCertAllowedResources, err := types.ResourceIDsFromString(sshCert.Permissions.Extensions[teleport.CertExtensionAllowedResources])
	require.NoError(t, err)
	assert.ElementsMatch(t, resourceIDs, sshCertAllowedResources)
	assert.ElementsMatch(t, resourceIDs, tlsIdentity.AllowedResourceIDs)
}

func TestCreateSuggestions(t *testing.T) {
	t.Parallel()

	testAuthServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir: t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testAuthServer.Close()) })

	const username = "admin"

	// Create the access request, so we can attach the promotions to it.
	adminRequest, err := services.NewAccessRequest(username, "admins")
	require.NoError(t, err)

	authSrvClient := testAuthServer.AuthServer
	err = authSrvClient.UpsertAccessRequest(context.Background(), adminRequest)
	require.NoError(t, err)

	// Create the promotions.
	err = authSrvClient.CreateAccessRequestAllowedPromotions(context.Background(), adminRequest, &types.AccessRequestAllowedPromotions{
		Promotions: []*types.AccessRequestAllowedPromotion{
			{AccessListName: "a"},
			{AccessListName: "b"},
			{AccessListName: "c"}},
	})
	require.NoError(t, err)

	// Get the promotions and verify them.
	promotions, err := authSrvClient.GetAccessRequestAllowedPromotions(context.Background(), adminRequest)
	require.NoError(t, err)
	require.Len(t, promotions.Promotions, 3)
	require.Equal(t, []string{"a", "b", "c"},
		[]string{
			promotions.Promotions[0].AccessListName,
			promotions.Promotions[1].AccessListName,
			promotions.Promotions[2].AccessListName,
		})
}

func TestPromotedRequest(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	testPack := newAccessRequestTestPack(ctx, t)

	const requesterUserName = "requester"
	requester := TestUser(requesterUserName)
	requesterClient, err := testPack.tlsServer.NewClient(requester)
	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, requesterClient.Close()) })

	// create the access request object
	req, err := services.NewAccessRequest(requesterUserName, "admins")
	require.NoError(t, err)

	// send the request to the auth server
	createdReq, err := requesterClient.CreateAccessRequestV2(ctx, req)
	require.NoError(t, err)

	const adminUser = "admin"
	approveAs := func(reviewerName string) (types.AccessRequest, error) {
		reviewer := TestUser(reviewerName)
		reviewerClient, err := testPack.tlsServer.NewClient(reviewer)
		require.NoError(t, err)

		t.Cleanup(func() { require.NoError(t, reviewerClient.Close()) })

		// try to promote the request
		return reviewerClient.SubmitAccessReview(ctx, types.AccessReviewSubmission{
			RequestID: req.GetName(),
			Review: types.AccessReview{
				ProposedState: types.RequestState_PROMOTED,
				Author:        adminUser,
				AccessList: &types.PromotedAccessList{
					Title: "ACL title",
					Name:  "0000-00-00-0000",
				},
			},
		})
	}

	t.Run("try promoting using access request API", func(t *testing.T) {
		// Access request promotion is prohibited for everyone, including admins.
		// An access request can be only approved by using Ent AccessRequestPromote API
		// operator can't promote the request
		_, err = approveAs("operator")
		require.Error(t, err)

		// admin can't promote the request
		_, err = approveAs(adminUser)
		require.Error(t, err)

		req2, err := requesterClient.GetAccessRequests(ctx, types.AccessRequestFilter{
			ID: createdReq.GetMetadata().Name,
		})
		require.NoError(t, err)
		require.Len(t, req2, 1)

		// the state should be still pending
		require.Equal(t, types.RequestState_PENDING, req2[0].GetState())
	})

	t.Run("promote without access list data fails", func(t *testing.T) {
		// The only way to promote the request is to use Ent AccessRequestPromote API
		// which is not available in OSS. As a workaround, we can use the access request
		// server API.
		_, err := testPack.tlsServer.AuthServer.AuthServer.SubmitAccessReview(ctx, types.AccessReviewSubmission{
			RequestID: createdReq.GetName(),
			Review: types.AccessReview{
				ProposedState: types.RequestState_PROMOTED,
			},
		})
		// Promoting without access list information is prohibited.
		require.Error(t, err)
	})

	t.Run("promote", func(t *testing.T) {
		promotedRequest, err := testPack.tlsServer.AuthServer.AuthServer.SubmitAccessReview(ctx, types.AccessReviewSubmission{
			RequestID: createdReq.GetName(),
			Review: types.AccessReview{
				ProposedState: types.RequestState_PROMOTED,
				Author:        adminUser,
				AccessList: &types.PromotedAccessList{
					Title: "ACL title",
					Name:  "0000-00-00-0000",
				},
			},
		})
		require.NoError(t, err)

		// verify promotion related fields
		require.Equal(t, types.RequestState_PROMOTED, promotedRequest.GetState())
		require.Equal(t, "0000-00-00-0000", promotedRequest.GetPromotedAccessListName())
		require.Equal(t, "ACL title", promotedRequest.GetPromotedAccessListTitle())
	})
}

func TestUpdateAccessRequestWithAdditionalReviewers(t *testing.T) {
	clock := clockwork.NewFakeClock()

	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			IdentityGovernanceSecurity: true,
		},
	})

	mustRequest := func(suggestedReviewers ...string) types.AccessRequest {
		req, err := services.NewAccessRequest("test-user", "admins")
		require.NoError(t, err)
		req.SetSuggestedReviewers(suggestedReviewers)
		return req
	}

	mustAccessList := func(name string, owners ...string) *accesslist.AccessList {
		ownersSpec := make([]accesslist.Owner, len(owners))
		for i, owner := range owners {
			ownersSpec[i] = accesslist.Owner{
				Name: owner,
			}
		}
		accessList, err := accesslist.NewAccessList(header.Metadata{
			Name: name,
		}, accesslist.Spec{
			Title: "simple",
			Grants: accesslist.Grants{
				Roles: []string{"grant-role"},
			},
			Audit: accesslist.Audit{
				NextAuditDate: clock.Now().AddDate(1, 0, 0),
			},
			Owners: ownersSpec,
		})
		require.NoError(t, err)
		return accessList
	}

	tests := []struct {
		name              string
		req               types.AccessRequest
		accessLists       []*accesslist.AccessList
		promotions        *types.AccessRequestAllowedPromotions
		expectedReviewers []string
	}{
		{
			name:              "nil promotions",
			req:               mustRequest("rev1", "rev2"),
			expectedReviewers: []string{"rev1", "rev2"},
		},
		{
			name: "a few promotions",
			req:  mustRequest("rev1", "rev2"),
			accessLists: []*accesslist.AccessList{
				mustAccessList("name1", "owner1", "owner2"),
				mustAccessList("name2", "owner1", "owner3"),
				mustAccessList("name3", "owner4", "owner5"),
			},
			promotions: &types.AccessRequestAllowedPromotions{
				Promotions: []*types.AccessRequestAllowedPromotion{
					{AccessListName: "name1"},
					{AccessListName: "name2"},
				},
			},
			expectedReviewers: []string{"rev1", "rev2", "owner1", "owner2", "owner3"},
		},
		{
			name: "no promotions",
			req:  mustRequest("rev1", "rev2"),
			accessLists: []*accesslist.AccessList{
				mustAccessList("name1", "owner1", "owner2"),
				mustAccessList("name2", "owner1", "owner3"),
				mustAccessList("name3", "owner4", "owner5"),
			},
			promotions: &types.AccessRequestAllowedPromotions{
				Promotions: []*types.AccessRequestAllowedPromotion{},
			},
			expectedReviewers: []string{"rev1", "rev2"},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			mem, err := memory.New(memory.Config{})
			require.NoError(t, err)
			accessLists, err := local.NewAccessListService(mem, clock)
			require.NoError(t, err)

			ctx := context.Background()
			for _, accessList := range test.accessLists {
				_, err = accessLists.UpsertAccessList(ctx, accessList)
				require.NoError(t, err)
			}

			req := test.req.Copy()
			updateAccessRequestWithAdditionalReviewers(ctx, req, accessLists, test.promotions)
			require.ElementsMatch(t, test.expectedReviewers, req.GetSuggestedReviewers())
		})
	}
}

func TestAssumeStartTime_CreateAccessRequestV2(t *testing.T) {
	ctx := context.Background()
	s := createAccessRequestWithStartTime(t)

	testCases := []struct {
		name      string
		startTime time.Time
		errCheck  require.ErrorAssertionFunc
	}{
		{
			name:      "too far in the future",
			startTime: s.invalidMaxedAssumeStartTime,
			errCheck: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
				require.ErrorContains(t, err, "assume start time is too far in the future")
			},
		},
		{
			name:      "after access expiry time",
			startTime: s.invalidExpiredAssumeStartTime,
			errCheck: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
				require.ErrorContains(t, err, "assume start time must be prior to access expiry time")
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req, err := services.NewAccessRequest(s.requesterUserName, "admins")
			require.NoError(t, err)
			req.SetMaxDuration(s.maxDuration)
			req.SetAssumeStartTime(tc.startTime)
			_, err = s.requesterClient.CreateAccessRequestV2(ctx, req)
			tc.errCheck(t, err)
		})
	}
}

func TestAssumeStartTime_SubmitAccessReview(t *testing.T) {
	ctx := context.Background()
	s := createAccessRequestWithStartTime(t)

	testCases := []struct {
		name      string
		startTime time.Time
		errCheck  require.ErrorAssertionFunc
	}{
		{
			name:      "too far in the future",
			startTime: s.invalidMaxedAssumeStartTime,
			errCheck: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
				require.ErrorContains(t, err, "assume start time is too far in the future")
			},
		},
		{
			name:      "after access expiry time",
			startTime: s.invalidExpiredAssumeStartTime,
			errCheck: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
				require.ErrorContains(t, err, "assume start time must be prior to access expiry time")
			},
		},
		{
			name:      "valid submission",
			startTime: s.validStartTime,
			errCheck:  require.NoError,
		},
	}
	review := types.AccessReviewSubmission{
		RequestID: s.createdRequest.GetName(),
		Review: types.AccessReview{
			Author:        "admin",
			ProposedState: types.RequestState_APPROVED,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			review.Review.AssumeStartTime = &tc.startTime
			resp, err := s.testPack.tlsServer.AuthServer.AuthServer.SubmitAccessReview(ctx, review)
			tc.errCheck(t, err)
			if err == nil {
				require.Equal(t, tc.startTime, *resp.GetAssumeStartTime())
			}
		})
	}
}

func TestAssumeStartTime_SetAccessRequestState(t *testing.T) {
	ctx := context.Background()
	s := createAccessRequestWithStartTime(t)

	testCases := []struct {
		name      string
		startTime time.Time
		errCheck  require.ErrorAssertionFunc
	}{
		{
			name:      "too far in the future",
			startTime: s.invalidMaxedAssumeStartTime,
			errCheck: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
				require.ErrorContains(t, err, "assume start time is too far in the future")
			},
		},
		{
			name:      "after access expiry time",
			startTime: s.invalidExpiredAssumeStartTime,
			errCheck: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
				require.ErrorContains(t, err, "assume start time must be prior to access expiry time")
			},
		},
		{
			name:      "valid set state",
			startTime: s.validStartTime,
			errCheck:  require.NoError,
		},
	}
	update := types.AccessRequestUpdate{
		RequestID: s.createdRequest.GetName(),
		State:     types.RequestState_APPROVED,
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			update.AssumeStartTime = &tc.startTime
			err := s.testPack.tlsServer.Auth().SetAccessRequestState(ctx, update)
			tc.errCheck(t, err)
			if err == nil {
				resp, err := s.testPack.tlsServer.AuthServer.AuthServer.GetAccessRequests(ctx, types.AccessRequestFilter{})
				require.NoError(t, err)
				require.Len(t, resp, 1)
				require.Equal(t, tc.startTime, *resp[0].GetAssumeStartTime())
			}
		})
	}
}

type accessRequestWithStartTime struct {
	testPack                      *accessRequestTestPack
	requesterClient               *authclient.Client
	invalidMaxedAssumeStartTime   time.Time
	invalidExpiredAssumeStartTime time.Time
	validStartTime                time.Time
	maxDuration                   time.Time
	requesterUserName             string
	createdRequest                types.AccessRequest
}

func createAccessRequestWithStartTime(t *testing.T) accessRequestWithStartTime {
	t.Helper()

	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	testPack := newAccessRequestTestPack(ctx, t)

	const requesterUserName = "requester"
	requester := TestUser(requesterUserName)
	requesterClient, err := testPack.tlsServer.NewClient(requester)
	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, requesterClient.Close()) })

	now := time.Now().UTC()
	day := 24 * time.Hour

	maxDuration := time.Now().UTC().Add(12 * day)

	invalidMaxedAssumeStartTime := now.Add(constants.MaxAssumeStartDuration + (1 * day))
	invalidExpiredAssumeStartTime := now.Add(100 * day)
	validStartTime := now.Add(6 * day)

	// create the access request object
	req, err := services.NewAccessRequest(requesterUserName, "admins")
	require.NoError(t, err)
	req.SetMaxDuration(maxDuration)

	req.SetAssumeStartTime(validStartTime)
	createdReq, err := requesterClient.CreateAccessRequestV2(ctx, req)
	require.NoError(t, err)
	require.Equal(t, validStartTime, *createdReq.GetAssumeStartTime())

	return accessRequestWithStartTime{
		testPack:                      testPack,
		requesterClient:               requesterClient,
		invalidMaxedAssumeStartTime:   invalidMaxedAssumeStartTime,
		invalidExpiredAssumeStartTime: invalidExpiredAssumeStartTime,
		validStartTime:                validStartTime,
		maxDuration:                   maxDuration,
		requesterUserName:             requesterUserName,
		createdRequest:                createdReq,
	}
}
