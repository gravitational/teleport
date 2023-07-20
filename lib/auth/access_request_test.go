/*
Copyright 2022 Gravitational, Inc.

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

package auth

import (
	"context"
	"crypto/tls"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
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
	require.NoError(t, err)
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
				},
			},
		},
		"empty": {},
	}
	for roleName, roleSpec := range roles {
		role, err := types.NewRole(roleName, roleSpec)
		require.NoError(t, err)

		err = tlsServer.Auth().UpsertRole(ctx, role)
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
		err = tlsServer.Auth().UpsertUser(user)
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
			err = requesterClient.CreateAccessRequest(ctx, req)
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

	type newClientFunc func(*testing.T, *Client, *proto.Certs) (*Client, *proto.Certs)
	updateClientWithNewAndDroppedRequests := func(newRequests, dropRequests []string) newClientFunc {
		return func(t *testing.T, clt *Client, _ *proto.Certs) (*Client, *proto.Certs) {
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
		return func(t *testing.T, clt *Client, certs *proto.Certs) (*Client, *proto.Certs) {
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
	err = auth.UpsertUser(user)
	require.NoError(t, err)

	// Create a client with the old set of roles.
	clt, err := testPack.tlsServer.NewClient(TestUser(username))
	require.NoError(t, err)

	// Add a new role to the user on the server.
	user.AddRole("operators")
	err = auth.UpdateUser(ctx, user)
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

	// Parse TLS cert.
	tlsCert, err := tlsca.ParseCertificatePEM(certs.TLS)
	require.NoError(t, err)
	tlsIdentity, err := tlsca.FromSubject(tlsCert.Subject, tlsCert.NotAfter)
	require.NoError(t, err)

	// Make sure both certs have the expected roles.
	rawSSHCertRoles := sshCert.Permissions.Extensions[teleport.CertExtensionTeleportRoles]
	sshCertRoles, err := services.UnmarshalCertRoles(rawSSHCertRoles)
	require.NoError(t, err)
	assert.ElementsMatch(t, roles, sshCertRoles)
	assert.ElementsMatch(t, roles, tlsIdentity.Groups)

	// Make sure both certs have the expected logins/principals.
	for _, certLogins := range [][]string{sshCert.ValidPrincipals, tlsIdentity.Principals} {
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
	rawSSHCertAccessRequests := sshCert.Permissions.Extensions[teleport.CertExtensionTeleportActiveRequests]
	sshCertAccessRequests := services.RequestIDs{}
	if len(rawSSHCertAccessRequests) > 0 {
		require.NoError(t, sshCertAccessRequests.Unmarshal([]byte(rawSSHCertAccessRequests)))
	}
	assert.ElementsMatch(t, accessRequests, sshCertAccessRequests.AccessRequests)
	assert.ElementsMatch(t, accessRequests, tlsIdentity.ActiveRequests)

	// Make sure both certs have the expected allowed resources, if any.
	sshCertAllowedResources, err := types.ResourceIDsFromString(sshCert.Permissions.Extensions[teleport.CertExtensionAllowedResources])
	require.NoError(t, err)
	assert.ElementsMatch(t, resourceIDs, sshCertAllowedResources)
	assert.ElementsMatch(t, resourceIDs, tlsIdentity.AllowedResourceIDs)
}
