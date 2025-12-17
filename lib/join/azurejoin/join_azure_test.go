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

package azurejoin_test

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/digitorus/pkcs7"
	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/machineid/machineidv1"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/join/azurejoin"
	"github.com/gravitational/teleport/lib/join/joinclient"
	"github.com/gravitational/teleport/lib/tlsca"
)

type mockAzureVMClient struct {
	azure.VirtualMachinesClient
	vms map[string]*azure.VirtualMachine
}

func (m *mockAzureVMClient) Get(_ context.Context, resourceID string) (*azure.VirtualMachine, error) {
	vm, ok := m.vms[resourceID]
	if !ok {
		return nil, trace.NotFound("no vm with resource id %q", resourceID)
	}
	return vm, nil
}

func (m *mockAzureVMClient) GetByVMID(_ context.Context, vmID string) (*azure.VirtualMachine, error) {
	for _, vm := range m.vms {
		if vm.VMID == vmID {
			return vm, nil
		}
	}
	return nil, trace.NotFound("no vm with id %q", vmID)
}

func makeVMClientGetter(clients map[string]*mockAzureVMClient) azurejoin.VMClientGetter {
	return func(subscriptionID string, _ *azure.StaticCredential) (azure.VirtualMachinesClient, error) {
		if client, ok := clients[subscriptionID]; ok {
			return client, nil
		}
		return nil, trace.NotFound("no client for subscription %q", subscriptionID)
	}
}

func vmssResourceID(subscription, resourceGroup, name string) string {
	return resourceID("Microsoft.Compute/virtualMachineScaleSets", subscription, resourceGroup, name)
}

func vmResourceID(subscription, resourceGroup, name string) string {
	return resourceID("Microsoft.Compute/virtualMachines", subscription, resourceGroup, name)
}

func identityResourceID(subscription, resourceGroup, name string) string {
	return resourceID("Microsoft.ManagedIdentity/userAssignedIdentities", subscription, resourceGroup, name)
}

func resourceID(resourceType, subscription, resourceGroup, name string) string {
	return fmt.Sprintf(
		"/subscriptions/%v/resourcegroups/%v/providers/%v/%v",
		subscription, resourceGroup, resourceType, name,
	)
}

func mockVerifyToken(err error) azurejoin.AzureVerifyTokenFunc {
	return func(_ context.Context, rawToken string) (*azurejoin.AccessTokenClaims, error) {
		if err != nil {
			return nil, err
		}
		tok, err := jwt.ParseSigned(rawToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		var claims azurejoin.AccessTokenClaims
		if err := tok.UnsafeClaimsWithoutVerification(&claims); err != nil {
			return nil, trace.Wrap(err)
		}
		return &claims, nil
	}
}

func makeToken(managedIdentityResourceID, azureResourceID string, issueTime time.Time) (string, error) {
	sig, err := jose.NewSigner(jose.SigningKey{
		Algorithm: jose.HS256,
		Key:       []byte("test-key"),
	}, &jose.SignerOptions{})
	if err != nil {
		return "", trace.Wrap(err)
	}
	claims := azurejoin.AccessTokenClaims{
		TokenClaims: oidc.TokenClaims{
			Issuer:     "https://sts.windows.net/test-tenant-id/",
			Audience:   []string{azurejoin.AzureAccessTokenAudience},
			Subject:    "test",
			IssuedAt:   oidc.FromTime(issueTime),
			NotBefore:  oidc.FromTime(issueTime),
			Expiration: oidc.FromTime(issueTime.Add(time.Minute)),
			JWTID:      "id",
		},
		ManangedIdentityResourceID: managedIdentityResourceID,
		AzureResourceID:            azureResourceID,
		TenantID:                   "test-tenant-id",
		Version:                    "1.0",
	}
	raw, err := jwt.Signed(sig).Claims(claims).CompactSerialize()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return raw, nil
}

func TestJoinAzure(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	server, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir: t.TempDir(),
		},
	})
	require.NoError(t, err)
	a := server.Auth()

	nopClient, err := server.NewClient(authtest.TestNop())
	require.NoError(t, err)

	caChain := newFakeAzureCAChain(t)
	httpClient := newFakeAzureIssuerHTTPClient(caChain.intermediateCertDER)
	instanceKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	instanceCert := caChain.issueLeafCert(t,
		instanceKey.Public(),
		"instance.metadata.azure.com",
		"http://www.microsoft.com/pkiops/certs/testcert.crt")

	isAccessDenied := func(t require.TestingT, err error, _ ...any) {
		require.True(t, trace.IsAccessDenied(err), "expected Access Denied error, actual error: %v", err)
	}
	isBadParameter := func(t require.TestingT, err error, _ ...any) {
		require.True(t, trace.IsBadParameter(err), "expected Bad Parameter error, actual error: %v", err)
	}

	defaultSubscription := uuid.NewString()
	defaultResourceGroup := "my-resource-group"
	defaultVMName := "test-vm"
	defaultIdentityName := "test-id"
	defaultVMID := "my-vm-id"
	defaultVMResourceID := vmResourceID(defaultSubscription, defaultResourceGroup, defaultVMName)
	defaultIdentityResourceID := identityResourceID(defaultSubscription, defaultResourceGroup, defaultIdentityName)

	tests := []struct {
		name                           string
		tokenManagedIdentityResourceID string
		tokenAzureResourceID           string
		tokenSubscription              string
		tokenVMID                      string
		requestTokenName               string
		tokenSpec                      types.ProvisionTokenSpecV2
		overrideReturnedChallenge      string
		challengeResponseErr           error
		certs                          []*x509.Certificate
		verify                         azurejoin.AzureVerifyTokenFunc
		assertError                    require.ErrorAssertionFunc
	}{
		{
			name:              "basic passing case",
			requestTokenName:  "test-token",
			tokenSubscription: defaultSubscription,
			tokenVMID:         defaultVMID,
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription:   defaultSubscription,
							ResourceGroups: []string{defaultResourceGroup},
						},
					},
				},
				JoinMethod: types.JoinMethodAzure,
			},
			verify:      mockVerifyToken(nil),
			certs:       []*x509.Certificate{caChain.rootCert},
			assertError: require.NoError,
		},
		{
			name:              "resource group is case insensitive",
			requestTokenName:  "test-token",
			tokenSubscription: defaultSubscription,
			tokenVMID:         defaultVMID,
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription:   defaultSubscription,
							ResourceGroups: []string{"MY-resource-GROUP"},
						},
					},
				},
				JoinMethod: types.JoinMethodAzure,
			},
			verify:      mockVerifyToken(nil),
			certs:       []*x509.Certificate{caChain.rootCert},
			assertError: require.NoError,
		},
		{
			name:              "wrong token",
			requestTokenName:  "wrong-token",
			tokenSubscription: defaultSubscription,
			tokenVMID:         defaultVMID,
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription: defaultSubscription,
						},
					},
				},
				JoinMethod: types.JoinMethodAzure,
			},
			verify:      mockVerifyToken(nil),
			certs:       []*x509.Certificate{caChain.rootCert},
			assertError: isAccessDenied,
		},
		{
			name:              "challenge response error",
			requestTokenName:  "test-token",
			tokenSubscription: defaultSubscription,
			tokenVMID:         defaultVMID,
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription: defaultSubscription,
						},
					},
				},
				JoinMethod: types.JoinMethodAzure,
			},
			verify:               mockVerifyToken(nil),
			certs:                []*x509.Certificate{caChain.rootCert},
			challengeResponseErr: trace.BadParameter("test error"),
			assertError:          isBadParameter,
		},
		{
			name:              "wrong subscription",
			requestTokenName:  "test-token",
			tokenSubscription: defaultSubscription,
			tokenVMID:         defaultVMID,
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription: "alternate-subscription-id",
						},
					},
				},
				JoinMethod: types.JoinMethodAzure,
			},
			verify:      mockVerifyToken(nil),
			certs:       []*x509.Certificate{caChain.rootCert},
			assertError: isAccessDenied,
		},
		{
			name:              "wrong resource group",
			requestTokenName:  "test-token",
			tokenSubscription: defaultSubscription,
			tokenVMID:         defaultVMID,
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription:   defaultSubscription,
							ResourceGroups: []string{"alternate-resource-group"},
						},
					},
				},
				JoinMethod: types.JoinMethodAzure,
			},
			verify:      mockVerifyToken(nil),
			certs:       []*x509.Certificate{caChain.rootCert},
			assertError: isAccessDenied,
		},
		{
			name:              "wrong challenge",
			requestTokenName:  "test-token",
			tokenSubscription: defaultSubscription,
			tokenVMID:         defaultVMID,
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription: defaultSubscription,
						},
					},
				},
				JoinMethod: types.JoinMethodAzure,
			},
			overrideReturnedChallenge: "wrong-challenge",
			verify:                    mockVerifyToken(nil),
			certs:                     []*x509.Certificate{caChain.rootCert},
			assertError:               isAccessDenied,
		},
		{
			name:              "invalid signature",
			requestTokenName:  "test-token",
			tokenSubscription: defaultSubscription,
			tokenVMID:         defaultVMID,
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription: defaultSubscription,
						},
					},
				},
				JoinMethod: types.JoinMethodAzure,
			},
			verify:      mockVerifyToken(nil),
			certs:       []*x509.Certificate{},
			assertError: require.Error,
		},
		{
			name:                           "attested data and access token from different VMs",
			requestTokenName:               "test-token",
			tokenSubscription:              defaultSubscription,
			tokenVMID:                      "some-other-vm-id",
			tokenManagedIdentityResourceID: defaultIdentityResourceID,
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription: defaultSubscription,
						},
					},
				},
				JoinMethod: types.JoinMethodAzure,
			},
			verify:      mockVerifyToken(nil),
			certs:       []*x509.Certificate{caChain.rootCert},
			assertError: isAccessDenied,
		},
		{
			name:                           "vm not found",
			requestTokenName:               "test-token",
			tokenSubscription:              defaultSubscription,
			tokenVMID:                      "invalid-id",
			tokenManagedIdentityResourceID: identityResourceID(defaultSubscription, defaultResourceGroup, "invalid-vm"),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription: defaultSubscription,
						},
					},
				},
				JoinMethod: types.JoinMethodAzure,
			},
			verify:      mockVerifyToken(nil),
			certs:       []*x509.Certificate{caChain.rootCert},
			assertError: isAccessDenied,
		},
		{
			name:                           "lookup vm by id",
			requestTokenName:               "test-token",
			tokenSubscription:              defaultSubscription,
			tokenVMID:                      defaultVMID,
			tokenManagedIdentityResourceID: defaultIdentityResourceID,
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription: defaultSubscription,
						},
					},
				},
				JoinMethod: types.JoinMethodAzure,
			},
			verify:      mockVerifyToken(nil),
			certs:       []*x509.Certificate{caChain.rootCert},
			assertError: require.NoError,
		},
		{
			name:                           "vm is in a different subscription than the token it provides",
			requestTokenName:               "test-token",
			tokenSubscription:              defaultSubscription,
			tokenVMID:                      defaultVMID,
			tokenManagedIdentityResourceID: identityResourceID("some-other-subscription", defaultResourceGroup, defaultVMName),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription: defaultSubscription,
						},
					},
				},
				JoinMethod: types.JoinMethodAzure,
			},
			verify:      mockVerifyToken(nil),
			certs:       []*x509.Certificate{caChain.rootCert},
			assertError: require.NoError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vmClient := &mockAzureVMClient{
				vms: map[string]*azure.VirtualMachine{
					defaultVMResourceID: {
						ID:            defaultVMResourceID,
						Name:          defaultVMName,
						Subscription:  defaultSubscription,
						ResourceGroup: defaultResourceGroup,
						VMID:          defaultVMID,
					},
				},
			}
			getVMClient := makeVMClientGetter(map[string]*mockAzureVMClient{
				defaultSubscription: vmClient,
			})

			a.SetAzureJoinConfig(&azurejoin.AzureJoinConfig{
				CertificateAuthorities: tc.certs,
				Verify:                 tc.verify,
				GetVMClient:            getVMClient,
				IssuerHTTPClient:       httpClient,
			})

			token, err := types.NewProvisionTokenFromSpec(
				"test-token",
				time.Now().Add(time.Minute),
				tc.tokenSpec)
			require.NoError(t, err)
			require.NoError(t, a.UpsertToken(ctx, token))
			t.Cleanup(func() {
				require.NoError(t, a.DeleteToken(ctx, token.GetName()))
			})

			mirID := tc.tokenManagedIdentityResourceID
			if mirID == "" {
				mirID = vmResourceID(defaultSubscription, defaultResourceGroup, defaultVMName)
			}

			accessToken, err := makeToken(mirID, "", a.GetClock().Now())
			require.NoError(t, err)

			imdsClient := &fakeIMDSClient{
				accessToken:       accessToken,
				accessTokenErr:    tc.challengeResponseErr,
				overrideChallenge: tc.overrideReturnedChallenge,
				signingCert:       instanceCert,
				signingKey:        instanceKey,
				subscription:      tc.tokenSubscription,
				vmID:              tc.tokenVMID,
			}

			t.Run("legacy", func(t *testing.T) {
				_, err = joinclient.LegacyJoin(ctx, joinclient.JoinParams{
					Token:      tc.requestTokenName,
					JoinMethod: types.JoinMethodAzure,
					ID: state.IdentityID{
						Role:     types.RoleInstance,
						HostUUID: "testuuid",
					},
					AuthClient: nopClient,
					AzureParams: joinclient.AzureParams{
						ClientID:   tc.tokenVMID,
						IMDSClient: imdsClient,
					},
				})
				tc.assertError(t, err)
			})
			t.Run("new", func(t *testing.T) {
				_, err = joinclient.Join(ctx, joinclient.JoinParams{
					Token: tc.requestTokenName,
					ID: state.IdentityID{
						Role: types.RoleInstance,
					},
					AuthClient: nopClient,
					AzureParams: joinclient.AzureParams{
						ClientID:         tc.tokenVMID,
						IMDSClient:       imdsClient,
						IssuerHTTPClient: httpClient,
					},
				})
				tc.assertError(t, err)
			})
		})
	}
}

// TestAuth_RegisterUsingAzureClaims tests the Azure join method by verifying
// joining VMs by the token claims rather than from the Azure VM API.
func TestJoinAzureClaims(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	server, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir: t.TempDir(),
		},
	})
	require.NoError(t, err)
	a := server.Auth()

	nopClient, err := server.NewClient(authtest.TestNop())
	require.NoError(t, err)

	caChain := newFakeAzureCAChain(t)
	httpClient := newFakeAzureIssuerHTTPClient(caChain.intermediateCertDER)
	instanceKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	instanceCert := caChain.issueLeafCert(t,
		instanceKey.Public(),
		"instance.metadata.azure.com",
		"http://www.microsoft.com/pkiops/certs/testcert.crt")

	isAccessDenied := func(t require.TestingT, err error, _ ...any) {
		require.True(t, trace.IsAccessDenied(err), "expected Access Denied error, actual error: %v", err)
	}
	defaultSubscription := uuid.NewString()
	defaultResourceGroup := "my-resource-group"
	defaultVMName := "test-vm"
	defaultIdentityName := "test-id"
	defaultVMID := "my-vm-id"

	botName := "botty"
	_, err = machineidv1.UpsertBot(ctx, a, &machineidv1pb.Bot{
		Kind:    types.KindBot,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: botName,
		},
		Spec: &machineidv1pb.BotSpec{},
	}, a.GetClock().Now(), "")
	require.NoError(t, err)

	tests := []struct {
		name                           string
		tokenManagedIdentityResourceID string
		tokenAzureResourceID           string
		tokenSubscription              string
		tokenVMID                      string
		requestTokenName               string
		tokenSpec                      types.ProvisionTokenSpecV2
		challengeResponseErr           error
		certs                          []*x509.Certificate
		verify                         azurejoin.AzureVerifyTokenFunc
		assertError                    require.ErrorAssertionFunc
	}{
		{
			name:                           "system-managed identity ok",
			requestTokenName:               "test-token",
			tokenSubscription:              "system-managed-test",
			tokenVMID:                      defaultVMID,
			tokenManagedIdentityResourceID: vmResourceID("system-managed-test", "system-managed-test", defaultVMName),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription:   "system-managed-test",
							ResourceGroups: []string{"system-managed-test"},
						},
					},
				},
				JoinMethod: types.JoinMethodAzure,
			},
			verify:      mockVerifyToken(nil),
			certs:       []*x509.Certificate{caChain.rootCert},
			assertError: require.NoError,
		},
		{
			name:                           "system-managed identity with wrong subscription",
			requestTokenName:               "test-token",
			tokenSubscription:              "system-managed-test",
			tokenVMID:                      defaultVMID,
			tokenManagedIdentityResourceID: vmResourceID("system-managed-test", "system-managed-test", defaultVMName),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription:   defaultSubscription,
							ResourceGroups: []string{"system-managed-test"},
						},
					},
				},
				JoinMethod: types.JoinMethodAzure,
			},
			verify:      mockVerifyToken(nil),
			certs:       []*x509.Certificate{caChain.rootCert},
			assertError: isAccessDenied,
		},
		{
			name:                           "system-managed identity with wrong resource group",
			requestTokenName:               "test-token",
			tokenSubscription:              "system-managed-test",
			tokenVMID:                      defaultVMID,
			tokenManagedIdentityResourceID: vmResourceID("system-managed-test", "system-managed-test", defaultVMName),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription:   "system-managed-test",
							ResourceGroups: []string{defaultResourceGroup},
						},
					},
				},
				JoinMethod: types.JoinMethodAzure,
			},
			verify:      mockVerifyToken(nil),
			certs:       []*x509.Certificate{caChain.rootCert},
			assertError: isAccessDenied,
		},
		{
			name:                           "user-managed identity ok",
			requestTokenName:               "test-token",
			tokenSubscription:              "user-managed-test",
			tokenVMID:                      defaultVMID,
			tokenManagedIdentityResourceID: identityResourceID("user-managed-test", "user-managed-test", defaultIdentityName),
			tokenAzureResourceID:           vmResourceID("user-managed-test", "user-managed-test", defaultVMName),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription:   "user-managed-test",
							ResourceGroups: []string{"user-managed-test"},
						},
					},
				},
				JoinMethod: types.JoinMethodAzure,
			},
			verify:      mockVerifyToken(nil),
			certs:       []*x509.Certificate{caChain.rootCert},
			assertError: require.NoError,
		},
		{
			name:                           "user-managed identity with wrong subscription",
			requestTokenName:               "test-token",
			tokenSubscription:              "user-managed-test",
			tokenVMID:                      defaultVMID,
			tokenManagedIdentityResourceID: identityResourceID("user-managed-test", "user-managed-test", defaultIdentityName),
			tokenAzureResourceID:           vmResourceID("user-managed-test", "user-managed-test", defaultVMName),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription:   defaultSubscription,
							ResourceGroups: []string{"user-managed-test"},
						},
					},
				},
				JoinMethod: types.JoinMethodAzure,
			},
			verify:      mockVerifyToken(nil),
			certs:       []*x509.Certificate{caChain.rootCert},
			assertError: isAccessDenied,
		},
		{
			name:                           "user-managed identity with wrong resource group",
			requestTokenName:               "test-token",
			tokenSubscription:              "user-managed-test",
			tokenVMID:                      defaultVMID,
			tokenManagedIdentityResourceID: identityResourceID("user-managed-test", "user-managed-test", defaultIdentityName),
			tokenAzureResourceID:           vmResourceID("user-managed-test", "user-managed-test", defaultVMName),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription:   "user-managed-test",
							ResourceGroups: []string{defaultResourceGroup},
						},
					},
				},
				JoinMethod: types.JoinMethodAzure,
			},
			verify:      mockVerifyToken(nil),
			certs:       []*x509.Certificate{caChain.rootCert},
			assertError: isAccessDenied,
		},
		{
			name:                           "user-managed identity from different subscription",
			requestTokenName:               "test-token",
			tokenSubscription:              "user-managed-test",
			tokenVMID:                      defaultVMID,
			tokenManagedIdentityResourceID: identityResourceID("invalid-user-managed-test", "invalid-user-managed-test", defaultIdentityName),
			tokenAzureResourceID:           vmResourceID("user-managed-test", "user-managed-test", defaultVMName),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription:   "user-managed-test",
							ResourceGroups: []string{"user-managed-test"},
						},
					},
				},
				JoinMethod: types.JoinMethodAzure,
			},
			verify:      mockVerifyToken(nil),
			certs:       []*x509.Certificate{caChain.rootCert},
			assertError: require.NoError,
		},
		{
			name:                           "subscription mismatch between attestation and token",
			requestTokenName:               "test-token",
			tokenSubscription:              "attested-subscription",
			tokenVMID:                      defaultVMID,
			tokenManagedIdentityResourceID: vmResourceID("token-subscription", defaultResourceGroup, defaultVMName),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription:   "token-subscription",
							ResourceGroups: []string{defaultResourceGroup},
						},
					},
				},
				JoinMethod: types.JoinMethodAzure,
			},
			verify:      mockVerifyToken(nil),
			certs:       []*x509.Certificate{caChain.rootCert},
			assertError: isAccessDenied,
		},
		{
			name:                           "vmss resource type",
			requestTokenName:               "test-token",
			tokenSubscription:              "token-subscription",
			tokenVMID:                      defaultVMID,
			tokenManagedIdentityResourceID: vmssResourceID("token-subscription", defaultResourceGroup, defaultVMName),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription:   "token-subscription",
							ResourceGroups: []string{defaultResourceGroup},
						},
					},
				},
				JoinMethod: types.JoinMethodAzure,
			},
			verify:      mockVerifyToken(nil),
			certs:       []*x509.Certificate{caChain.rootCert},
			assertError: require.NoError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			token, err := types.NewProvisionTokenFromSpec(
				"test-token",
				time.Now().Add(time.Minute),
				tc.tokenSpec)
			require.NoError(t, err)
			require.NoError(t, a.UpsertToken(ctx, token))
			t.Cleanup(func() {
				require.NoError(t, a.DeleteToken(ctx, token.GetName()))
			})

			mirID := tc.tokenManagedIdentityResourceID
			azrID := tc.tokenAzureResourceID
			accessToken, err := makeToken(mirID, azrID, a.GetClock().Now())
			require.NoError(t, err)

			vmClient := &mockAzureVMClient{
				vms: map[string]*azure.VirtualMachine{},
			}
			getVMClient := makeVMClientGetter(map[string]*mockAzureVMClient{
				defaultSubscription: vmClient,
			})

			a.SetAzureJoinConfig(&azurejoin.AzureJoinConfig{
				CertificateAuthorities: tc.certs,
				Verify:                 tc.verify,
				GetVMClient:            getVMClient,
				IssuerHTTPClient:       httpClient,
			})

			imdsClient := &fakeIMDSClient{
				accessToken:    accessToken,
				accessTokenErr: tc.challengeResponseErr,
				signingCert:    instanceCert,
				signingKey:     instanceKey,
				subscription:   tc.tokenSubscription,
				vmID:           tc.tokenVMID,
			}

			t.Run("legacy", func(t *testing.T) {
				// Try to join via the legacy join service.
				_, err = joinclient.LegacyJoin(ctx, joinclient.JoinParams{
					Token:      tc.requestTokenName,
					JoinMethod: types.JoinMethodAzure,
					ID: state.IdentityID{
						Role:     types.RoleInstance,
						HostUUID: "testuuid",
					},
					AuthClient: nopClient,
					AzureParams: joinclient.AzureParams{
						ClientID:   tc.tokenVMID,
						IMDSClient: imdsClient,
					},
				})
				tc.assertError(t, err)
			})
			t.Run("new", func(t *testing.T) {
				// Try to join via the new join service.
				_, err = joinclient.Join(ctx, joinclient.JoinParams{
					Token: tc.requestTokenName,
					ID: state.IdentityID{
						Role: types.RoleInstance,
					},
					AuthClient: nopClient,
					AzureParams: joinclient.AzureParams{
						ClientID:         tc.tokenVMID,
						IMDSClient:       imdsClient,
						IssuerHTTPClient: httpClient,
					},
				})
				tc.assertError(t, err)
			})
			t.Run("bot", func(t *testing.T) {
				// Try to join as a bot.
				tokenSpec := tc.tokenSpec
				tokenSpec.BotName = botName
				tokenSpec.Roles = types.SystemRoles{types.RoleBot}
				token, err := types.NewProvisionTokenFromSpec(
					"test-token",
					time.Now().Add(time.Minute),
					tokenSpec)
				require.NoError(t, err)
				require.NoError(t, a.UpsertToken(ctx, token))

				result, err := joinclient.Join(ctx, joinclient.JoinParams{
					Token: tc.requestTokenName,
					ID: state.IdentityID{
						Role: types.RoleBot,
					},
					AuthClient: nopClient,
					AzureParams: joinclient.AzureParams{
						ClientID:         tc.tokenVMID,
						IMDSClient:       imdsClient,
						IssuerHTTPClient: httpClient,
					},
				})
				tc.assertError(t, err)
				if err != nil {
					return
				}

				cert, err := tlsca.ParseCertificatePEM(result.Certs.TLS)
				require.NoError(t, err)
				identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
				require.NoError(t, err)

				// Make sure the LoginIP was set on the identity.
				require.NotEmpty(t, identity.LoginIP)

				// Make sure the JoinAttributes were set.
				require.NotNil(t, identity.JoinAttributes)
				require.NotNil(t, identity.JoinAttributes.Azure)
				require.Equal(t, tc.tokenSubscription, identity.JoinAttributes.Azure.Subscription)
			})
		})
	}
}

type fakeIMDSClient struct {
	accessToken    string
	accessTokenErr error

	// overrideChallenge overrides the challenge/nonce included in attested data.
	overrideChallenge string
	signingCert       *x509.Certificate
	signingKey        crypto.Signer
	subscription      string
	vmID              string
}

func (c *fakeIMDSClient) IsAvailable(_ context.Context) bool {
	return true
}

func (c *fakeIMDSClient) GetAttestedData(_ context.Context, nonce string) ([]byte, error) {
	ad := azurejoin.AttestedData{
		Nonce:          nonce,
		SubscriptionID: c.subscription,
		ID:             c.vmID,
	}
	if c.overrideChallenge != "" {
		ad.Nonce = c.overrideChallenge
	}
	adBytes, err := json.Marshal(&ad)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s, err := pkcs7.NewSignedData(adBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.AddSigner(c.signingCert, c.signingKey, pkcs7.SignerInfoConfig{}); err != nil {
		return nil, trace.Wrap(err)
	}
	signature, err := s.Finish()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signedAD := azurejoin.SignedAttestedData{
		Encoding:  "pkcs7",
		Signature: base64.StdEncoding.EncodeToString(signature),
	}
	signedADBytes, err := json.Marshal(&signedAD)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return signedADBytes, nil
}

func (c *fakeIMDSClient) GetAccessToken(_ context.Context, clientID string) (string, error) {
	return c.accessToken, trace.Wrap(c.accessTokenErr)
}

type fakeAzureCAChain struct {
	intermediateKey     crypto.Signer
	intermediateCert    *x509.Certificate
	intermediateCertDER []byte
	rootCert            *x509.Certificate
}

func newFakeAzureCAChain(t *testing.T) *fakeAzureCAChain {
	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	rootCertTemplate := &x509.Certificate{
		Subject: pkix.Name{
			CommonName: "test root CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	rootCertDER, err := x509.CreateCertificate(rand.Reader, rootCertTemplate, rootCertTemplate, rootKey.Public(), rootKey)
	require.NoError(t, err)
	rootCert, err := x509.ParseCertificate(rootCertDER)
	require.NoError(t, err)

	intermediateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	intermediateCertTemplate := &x509.Certificate{
		Subject: pkix.Name{
			CommonName: "test intermediate CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	intermediateCertDER, err := x509.CreateCertificate(rand.Reader, intermediateCertTemplate, rootCert, intermediateKey.Public(), rootKey)
	require.NoError(t, err)
	intermediateCert, err := x509.ParseCertificate(intermediateCertDER)
	require.NoError(t, err)

	return &fakeAzureCAChain{
		intermediateKey:     intermediateKey,
		intermediateCert:    intermediateCert,
		intermediateCertDER: intermediateCertDER,
		rootCert:            rootCert,
	}
}

func (c *fakeAzureCAChain) issueLeafCert(t *testing.T, pub crypto.PublicKey, commonName, issuerURL string) *x509.Certificate {
	leafCertTemplate := &x509.Certificate{
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		IssuingCertificateURL: []string{issuerURL},
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	leafCertDER, err := x509.CreateCertificate(rand.Reader, leafCertTemplate, c.intermediateCert, pub, c.intermediateKey)
	require.NoError(t, err)
	leafCert, err := x509.ParseCertificate(leafCertDER)
	require.NoError(t, err)
	return leafCert
}

type fakeAzureIssuerHTTPClient struct {
	issuerCertDER []byte
	called        int
}

func newFakeAzureIssuerHTTPClient(issuerCertDER []byte) *fakeAzureIssuerHTTPClient {
	return &fakeAzureIssuerHTTPClient{
		issuerCertDER: issuerCertDER,
	}
}
func (c *fakeAzureIssuerHTTPClient) Do(req *http.Request) (*http.Response, error) {
	c.called++
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(c.issuerCertDER)),
	}, nil
}
