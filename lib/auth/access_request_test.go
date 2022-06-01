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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/gravitational/trace"

	"github.com/stretchr/testify/require"
)

func TestAccessRequest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	testAuthServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir: t.TempDir(),
	})
	require.NoError(t, err)
	server, err := testAuthServer.NewTestTLSServer()
	require.NoError(t, err)

	clusterName, err := server.Auth().GetClusterName()
	require.NoError(t, err)

	roleSpecs := map[string]types.RoleSpecV5{
		// admins is the role to be requested
		"admins": types.RoleSpecV5{
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
		"operators": types.RoleSpecV5{
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles: []string{"admins"},
				},
			},
		},
		// responders can request the admins role but only with a
		// search-based request limited to specific resources
		"responders": types.RoleSpecV5{
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{"admins"},
				},
			},
		},
		"empty": types.RoleSpecV5{},
	}
	for roleName, roleSpec := range roleSpecs {
		role, err := types.NewRole(roleName, roleSpec)
		require.NoError(t, err)

		err = server.Auth().UpsertRole(ctx, role)
		require.NoError(t, err)
	}

	userDesc := map[string]struct {
		roles []string
	}{
		"admin": {
			roles: []string{"admins"},
		},
		"responder": {
			roles: []string{"responders"},
		},
		"operator": {
			roles: []string{"operators"},
		},
		"nobody": {
			roles: []string{"empty"},
		},
	}
	for name, desc := range userDesc {
		user, err := types.NewUser(name)
		require.NoError(t, err)
		user.SetRoles(desc.roles)
		err = server.Auth().UpsertUser(user)
		require.NoError(t, err)
	}

	type nodeDesc struct {
		labels map[string]string
	}
	nodes := map[string]nodeDesc{
		"staging": nodeDesc{
			labels: map[string]string{
				"env": "staging",
			},
		},
		"prod": nodeDesc{
			labels: map[string]string{
				"env": "prod",
			},
		},
	}
	for nodeName, desc := range nodes {
		node, err := types.NewServerWithLabels(nodeName, types.KindNode, types.ServerSpecV2{}, desc.labels)
		require.NoError(t, err)
		_, err = server.Auth().UpsertNode(context.Background(), node)
		require.NoError(t, err)
	}

	privKey, pubKey, err := native.GenerateKeyPair()
	require.NoError(t, err)

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
			desc:             "search-based request for staging",
			requester:        "responder",
			reviewer:         "admin",
			requestResources: []string{"staging"},
			expectRoles:      []string{"responders", "admins"},
			expectNodes:      []string{"staging"},
		},
		{
			desc:             "search-based request for prod",
			requester:        "responder",
			reviewer:         "admin",
			requestResources: []string{"prod"},
			expectRoles:      []string{"responders", "admins"},
			expectNodes:      []string{"prod"},
		},
		{
			desc:             "search-based request for both",
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
			expectRequestError: trace.AccessDenied(`user does not have any "search_as_roles" which are valid for this request`),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			requester := TestUser(tc.requester)
			requesterClient, err := server.NewClient(requester)
			require.NoError(t, err)

			// generateCerts executes a GenerateUserCerts request, optionally applying
			// one or more access-requests to the certificate.
			generateCerts := func(reqIDs ...string) (*proto.Certs, error) {
				return requesterClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
					PublicKey:      pubKey,
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
			checkCerts(t, certs, userDesc[tc.requester].roles, nil, nil, nil)

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
					ClusterName: clusterName.GetClusterName(),
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
			reviewerClient, err := server.NewClient(reviewer)
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

			elevatedCert, err := tls.X509KeyPair(certs.TLS, privKey)
			require.NoError(t, err)
			elevatedClient := server.NewClientWithCert(elevatedCert)

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
				PublicKey: pubKey,
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
			require.NoError(t, server.Auth().SetAccessRequestState(ctx, types.AccessRequestUpdate{
				RequestID: req.GetName(),
				State:     types.RequestState_DENIED,
			}))
			_, err = generateCerts(req.GetName())
			require.ErrorIs(t, err, trace.AccessDenied("access request %q has been denied", req.GetName()))

			// ensure that once in the DENIED state, a request cannot be set back to PENDING state.
			require.Error(t, server.Auth().SetAccessRequestState(ctx, types.AccessRequestUpdate{
				RequestID: req.GetName(),
				State:     types.RequestState_PENDING,
			}))

			// ensure that once in the DENIED state, a request cannot be set back to APPROVED state.
			require.Error(t, server.Auth().SetAccessRequestState(ctx, types.AccessRequestUpdate{
				RequestID: req.GetName(),
				State:     types.RequestState_APPROVED,
			}))

			// ensure that identities with requests in the DENIED state can't reissue new certs.
			_, err = elevatedClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
				PublicKey: pubKey,
				Username:  tc.requester,
				Expires:   time.Now().Add(time.Hour).UTC(),
				// no new access requests
				AccessRequests: nil,
			})
			require.ErrorIs(t, err, trace.AccessDenied("access request %q has been denied", req.GetName()))
		})
	}
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
	require.ElementsMatch(t, roles, sshCertRoles)
	require.ElementsMatch(t, roles, tlsIdentity.Groups)

	// Make sure both certs have the expected logins/principals.
	for _, certLogins := range [][]string{sshCert.ValidPrincipals, tlsIdentity.Principals} {
		// filter out invalid logins placed in the cert
		validCertLogins := []string{}
		for _, certLogin := range certLogins {
			if !strings.HasPrefix(certLogin, "-teleport") {
				validCertLogins = append(validCertLogins, certLogin)
			}
		}
		require.ElementsMatch(t, logins, validCertLogins)
	}

	// Make sure both certs have the expected access requests, if any.
	rawSSHCertAccessRequests := sshCert.Permissions.Extensions[teleport.CertExtensionTeleportActiveRequests]
	sshCertAccessRequests := services.RequestIDs{}
	if len(rawSSHCertAccessRequests) > 0 {
		require.NoError(t, sshCertAccessRequests.Unmarshal([]byte(rawSSHCertAccessRequests)))
	}
	require.ElementsMatch(t, accessRequests, sshCertAccessRequests.AccessRequests)
	require.ElementsMatch(t, accessRequests, tlsIdentity.ActiveRequests)

	// Make sure both certs have the expected allowed resources, if any.
	sshCertAllowedResources, err := types.ResourceIDsFromString(sshCert.Permissions.Extensions[teleport.CertExtensionAllowedResources])
	require.NoError(t, err)
	require.ElementsMatch(t, resourceIDs, sshCertAllowedResources)
	require.ElementsMatch(t, resourceIDs, tlsIdentity.AllowedResourceIDs)
}
