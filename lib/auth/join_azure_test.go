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

package auth_test

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
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/cloud/azure"
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

func makeVMClientGetter(clients map[string]*mockAzureVMClient) auth.AzureVMClientGetter {
	return func(subscriptionID string, _ *azure.StaticCredential) (azure.VirtualMachinesClient, error) {
		if client, ok := clients[subscriptionID]; ok {
			return client, nil
		}
		return nil, trace.NotFound("no client for subscription %q", subscriptionID)
	}
}

type azureChallengeResponseConfig struct {
	Challenge string
}

type azureChallengeResponseOption func(*azureChallengeResponseConfig)

func withChallengeAzure(challenge string) azureChallengeResponseOption {
	return func(cfg *azureChallengeResponseConfig) {
		cfg.Challenge = challenge
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

func mockVerifyToken(err error) auth.AzureVerifyTokenFunc {
	return func(_ context.Context, rawToken string) (*auth.AccessTokenClaims, error) {
		if err != nil {
			return nil, err
		}
		tok, err := jwt.ParseSigned(rawToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		var claims auth.AccessTokenClaims
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
	claims := auth.AccessTokenClaims{
		Claims: jwt.Claims{
			Issuer:    "https://sts.windows.net/test-tenant-id/",
			Audience:  []string{auth.AzureAccessTokenAudience},
			Subject:   "test",
			IssuedAt:  jwt.NewNumericDate(issueTime),
			NotBefore: jwt.NewNumericDate(issueTime),
			Expiry:    jwt.NewNumericDate(issueTime.Add(time.Minute)),
			ID:        "id",
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

func TestAuth_RegisterUsingAzureMethod(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)
	a := p.a

	sshPrivateKey, sshPublicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	tlsPublicKey, err := authtest.PrivateKeyToPublicKeyTLS(sshPrivateKey)
	require.NoError(t, err)

	caChain := newFakeAzureCAChain(t)
	// Fake the HTTP client used to fetch the issuer CA.
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
		challengeResponseOptions       []azureChallengeResponseOption
		challengeResponseErr           error
		certs                          []*x509.Certificate
		verify                         auth.AzureVerifyTokenFunc
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
			challengeResponseOptions: []azureChallengeResponseOption{
				withChallengeAzure("wrong-challenge"),
			},
			verify:      mockVerifyToken(nil),
			certs:       []*x509.Certificate{caChain.rootCert},
			assertError: isAccessDenied,
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

			_, err = a.RegisterUsingAzureMethodWithOpts(context.Background(), func(challenge string) (*proto.RegisterUsingAzureMethodRequest, error) {
				cfg := &azureChallengeResponseConfig{Challenge: challenge}
				for _, opt := range tc.challengeResponseOptions {
					opt(cfg)
				}

				ad := auth.AttestedData{
					Nonce:          cfg.Challenge,
					SubscriptionID: tc.tokenSubscription,
					ID:             tc.tokenVMID,
				}
				adBytes, err := json.Marshal(&ad)
				require.NoError(t, err)
				s, err := pkcs7.NewSignedData(adBytes)
				require.NoError(t, err)
				require.NoError(t, s.AddSigner(instanceCert, instanceKey, pkcs7.SignerInfoConfig{}))
				signature, err := s.Finish()
				require.NoError(t, err)
				signedAD := auth.SignedAttestedData{
					Encoding:  "pkcs7",
					Signature: base64.StdEncoding.EncodeToString(signature),
				}
				signedADBytes, err := json.Marshal(&signedAD)
				require.NoError(t, err)

				req := &proto.RegisterUsingAzureMethodRequest{
					RegisterUsingTokenRequest: &types.RegisterUsingTokenRequest{
						Token:        tc.requestTokenName,
						HostID:       "test-node",
						Role:         types.RoleNode,
						PublicSSHKey: sshPublicKey,
						PublicTLSKey: tlsPublicKey,
					},
					AttestedData: signedADBytes,
					AccessToken:  accessToken,
				}
				return req, tc.challengeResponseErr
			},
				auth.WithAzureCerts(tc.certs),
				auth.WithAzureVerifyFunc(tc.verify),
				auth.WithAzureVMClientGetter(getVMClient),
				auth.WithAzureIssuerHTTPClient(httpClient),
			)
			tc.assertError(t, err)
		})
	}
}

// TestAuth_RegisterUsingAzureClaims tests the Azure join method by verifying
// joining VMs by the token claims rather than from the Azure VM API.
func TestAuth_RegisterUsingAzureClaims(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)
	a := p.a

	sshPrivateKey, sshPublicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	tlsPublicKey, err := authtest.PrivateKeyToPublicKeyTLS(sshPrivateKey)
	require.NoError(t, err)

	caChain := newFakeAzureCAChain(t)
	// Fake the HTTP client used to fetch the issuer CA.
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

	tests := []struct {
		name                           string
		tokenManagedIdentityResourceID string
		tokenAzureResourceID           string
		tokenSubscription              string
		tokenVMID                      string
		requestTokenName               string
		tokenSpec                      types.ProvisionTokenSpecV2
		challengeResponseOptions       []azureChallengeResponseOption
		challengeResponseErr           error
		certs                          []*x509.Certificate
		verify                         auth.AzureVerifyTokenFunc
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

			_, err = a.RegisterUsingAzureMethodWithOpts(context.Background(), func(challenge string) (*proto.RegisterUsingAzureMethodRequest, error) {
				cfg := &azureChallengeResponseConfig{Challenge: challenge}
				for _, opt := range tc.challengeResponseOptions {
					opt(cfg)
				}

				ad := auth.AttestedData{
					Nonce:          cfg.Challenge,
					SubscriptionID: tc.tokenSubscription,
					ID:             tc.tokenVMID,
				}
				adBytes, err := json.Marshal(&ad)
				require.NoError(t, err)
				s, err := pkcs7.NewSignedData(adBytes)
				require.NoError(t, err)
				require.NoError(t, s.AddSigner(instanceCert, instanceKey, pkcs7.SignerInfoConfig{}))
				signature, err := s.Finish()
				require.NoError(t, err)
				signedAD := auth.SignedAttestedData{
					Encoding:  "pkcs7",
					Signature: base64.StdEncoding.EncodeToString(signature),
				}
				signedADBytes, err := json.Marshal(&signedAD)
				require.NoError(t, err)

				req := &proto.RegisterUsingAzureMethodRequest{
					RegisterUsingTokenRequest: &types.RegisterUsingTokenRequest{
						Token:        tc.requestTokenName,
						HostID:       "test-node",
						Role:         types.RoleNode,
						PublicSSHKey: sshPublicKey,
						PublicTLSKey: tlsPublicKey,
					},
					AttestedData: signedADBytes,
					AccessToken:  accessToken,
				}
				return req, tc.challengeResponseErr
			},
				auth.WithAzureCerts(tc.certs),
				auth.WithAzureVerifyFunc(tc.verify),
				auth.WithAzureVMClientGetter(getVMClient),
				auth.WithAzureIssuerHTTPClient(httpClient),
			)
			tc.assertError(t, err)
		})
	}
}

func TestAzureIssuerCert(t *testing.T) {
	server, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir: t.TempDir(),
	})
	require.NoError(t, err)
	a := server.AuthServer

	token, err := types.NewProvisionTokenFromSpec("testtoken", time.Now().Add(time.Minute), types.ProvisionTokenSpecV2{
		JoinMethod: types.JoinMethodAzure,
		Roles:      types.SystemRoles{types.RoleNode},
		Azure: &types.ProvisionTokenSpecV2Azure{
			Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
				{
					Subscription: "testsubscription",
				},
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, a.UpsertToken(t.Context(), token))

	caChain := newFakeAzureCAChain(t)

	instanceKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	sshPubKey, err := ssh.NewPublicKey(instanceKey.Public())
	require.NoError(t, err)
	sshPub := ssh.MarshalAuthorizedKey(sshPubKey)
	tlsPub, err := keys.MarshalPublicKey(instanceKey.Public())
	require.NoError(t, err)

	instanceID := vmResourceID("testsubscription", "testgroup", "testid")

	accessToken, err := makeToken(instanceID, instanceID, a.GetClock().Now())
	require.NoError(t, err)

	isAccessDenied := func(t require.TestingT, err error, msgAndArgs ...any) {
		require.ErrorAs(t, err, new(*trace.AccessDeniedError), msgAndArgs...)
	}
	for _, tc := range []struct {
		desc                      string
		commonName                string
		issuerURL                 string
		errorAssertion            require.ErrorAssertionFunc
		expecteRequestedIssuingCA bool
	}{
		{
			desc:                      "passing",
			commonName:                "instance.metadata.azure.com",
			issuerURL:                 "http://www.microsoft.com/pkiops/certs/testca.crt",
			errorAssertion:            require.NoError,
			expecteRequestedIssuingCA: true,
		},
		{
			desc:       "bad common name",
			commonName: "instance.metadata.bad.example.com",
			issuerURL:  "http://www.microsoft.com/pkiops/certs/testca.crt",
			errorAssertion: func(t require.TestingT, err error, msgAndArgs ...any) {
				isAccessDenied(t, err, msgAndArgs...)
				require.ErrorContains(t, err, "certificate common name does not match allow-list")
			},
		},
		{
			desc:       "bad issuer host",
			commonName: "instance.metadata.azure.com",
			issuerURL:  "http://www.bad.example.com/pkiops/certs/testca.crt",
			errorAssertion: func(t require.TestingT, err error, msgAndArgs ...any) {
				isAccessDenied(t, err, msgAndArgs...)
				require.ErrorContains(t, err, "validating issuing certificate URL")
				require.ErrorContains(t, err, "invalid host")
			},
		},
		{
			desc:       "bad cert path",
			commonName: "instance.metadata.azure.com",
			issuerURL:  "http://www.microsoft.com/pkiops/badcerts/badca.crt",
			errorAssertion: func(t require.TestingT, err error, msgAndArgs ...any) {
				isAccessDenied(t, err, msgAndArgs...)
				require.ErrorContains(t, err, "validating issuing certificate URL")
				require.ErrorContains(t, err, "invalid path")
			},
		},
		{
			desc:       "bad url scheme",
			commonName: "instance.metadata.azure.com",
			issuerURL:  "bad://www.microsoft.com/pkiops/certs/testca.crt",
			errorAssertion: func(t require.TestingT, err error, msgAndArgs ...any) {
				isAccessDenied(t, err, msgAndArgs...)
				require.ErrorContains(t, err, "validating issuing certificate URL")
				require.ErrorContains(t, err, "invalid url scheme")
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// Generate an azure instance cert with the common name and issuer URL for the testcase.
			instanceCert := caChain.issueLeafCert(t, instanceKey.Public(), tc.commonName, tc.issuerURL)
			// Fake the HTTP client used to fetch the issuer CA.
			httpClient := newFakeAzureIssuerHTTPClient(caChain.intermediateCertDER)

			solveChallenge := func(challenge string) (*proto.RegisterUsingAzureMethodRequest, error) {
				// Generate fake attested data signed by the instance cert generated for the testcase.
				ad := auth.AttestedData{
					Nonce:          challenge,
					SubscriptionID: "testsubscription",
					ID:             instanceID,
				}
				adBytes, err := json.Marshal(&ad)
				require.NoError(t, err)
				s, err := pkcs7.NewSignedData(adBytes)
				require.NoError(t, err)
				require.NoError(t, s.AddSigner(instanceCert, instanceKey, pkcs7.SignerInfoConfig{}))
				signature, err := s.Finish()
				require.NoError(t, err)
				signedAD := auth.SignedAttestedData{
					Encoding:  "pkcs7",
					Signature: base64.StdEncoding.EncodeToString(signature),
				}
				signedADBytes, err := json.Marshal(&signedAD)
				require.NoError(t, err)

				return &proto.RegisterUsingAzureMethodRequest{
					RegisterUsingTokenRequest: &types.RegisterUsingTokenRequest{
						Token:        "testtoken",
						HostID:       "testid",
						Role:         types.RoleNode,
						PublicSSHKey: sshPub,
						PublicTLSKey: tlsPub,
					},
					AccessToken:  accessToken,
					AttestedData: signedADBytes,
				}, nil
			}

			_, err := a.RegisterUsingAzureMethodWithOpts(t.Context(), solveChallenge,
				auth.WithAzureIssuerHTTPClient(httpClient),
				auth.WithAzureCerts([]*x509.Certificate{caChain.rootCert}),
				auth.WithAzureVerifyFunc(mockVerifyToken(nil)),
			)
			tc.errorAssertion(t, err)
			if tc.expecteRequestedIssuingCA {
				require.Equal(t, 1, httpClient.called, "expected issuing CA to be requested once")
			} else {
				require.Equal(t, 0, httpClient.called, "expected issuing CA not to be requested")
			}
		})
	}
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
