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
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"testing"
	"time"

	"github.com/digitorus/pkcs7"
	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/fixtures"
)

func withCerts(certs []*x509.Certificate) azureRegisterOption {
	return func(cfg *azureRegisterConfig) {
		cfg.certificateAuthorities = certs
	}
}

func withVerifyFunc(verify azureVerifyTokenFunc) azureRegisterOption {
	return func(cfg *azureRegisterConfig) {
		cfg.verify = verify
	}
}

func withVMClientGetter(getVMClient vmClientGetter) azureRegisterOption {
	return func(cfg *azureRegisterConfig) {
		cfg.getVMClient = getVMClient
	}
}

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

func (m *mockAzureVMClient) GetByVMID(_ context.Context, vmID string, opts ...azure.GetByVMIDOption) (*azure.VirtualMachine, error) {
	for _, vm := range m.vms {
		if vm.VMID == vmID {
			return vm, nil
		}
	}
	return nil, trace.NotFound("no vm with id %q", vmID)
}

func makeVMClientGetter(clients map[string]*mockAzureVMClient) vmClientGetter {
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

func mockVerifyToken(err error) azureVerifyTokenFunc {
	return func(_ context.Context, rawToken string) (*accessTokenClaims, error) {
		if err != nil {
			return nil, err
		}
		tok, err := jwt.ParseSigned(rawToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		var claims accessTokenClaims
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
	claims := accessTokenClaims{
		Claims: jwt.Claims{
			Issuer:    "https://sts.windows.net/test-tenant-id/",
			Audience:  []string{azureAccessTokenAudience},
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

	tlsConfig, err := fixtures.LocalTLSConfig()
	require.NoError(t, err)

	block, _ := pem.Decode(fixtures.LocalhostKey)
	pkey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	require.NoError(t, err)

	tlsPublicKey, err := PrivateKeyToPublicKeyTLS(sshPrivateKey)
	require.NoError(t, err)

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
		verify                         azureVerifyTokenFunc
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
			certs:       []*x509.Certificate{tlsConfig.Certificate},
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
			certs:       []*x509.Certificate{tlsConfig.Certificate},
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
			certs:       []*x509.Certificate{tlsConfig.Certificate},
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
			certs:                []*x509.Certificate{tlsConfig.Certificate},
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
			certs:       []*x509.Certificate{tlsConfig.Certificate},
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
			certs:       []*x509.Certificate{tlsConfig.Certificate},
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
			certs:       []*x509.Certificate{tlsConfig.Certificate},
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
			certs:       []*x509.Certificate{tlsConfig.Certificate},
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
			certs:       []*x509.Certificate{tlsConfig.Certificate},
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
			certs:       []*x509.Certificate{tlsConfig.Certificate},
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
			certs:       []*x509.Certificate{tlsConfig.Certificate},
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

			accessToken, err := makeToken(mirID, "", a.clock.Now())
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

				ad := attestedData{
					Nonce:          cfg.Challenge,
					SubscriptionID: tc.tokenSubscription,
					ID:             tc.tokenVMID,
				}
				adBytes, err := json.Marshal(&ad)
				require.NoError(t, err)
				s, err := pkcs7.NewSignedData(adBytes)
				require.NoError(t, err)
				require.NoError(t, s.AddSigner(tlsConfig.Certificate, pkey, pkcs7.SignerInfoConfig{}))
				signature, err := s.Finish()
				require.NoError(t, err)
				signedAD := signedAttestedData{
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
			}, withCerts(tc.certs), withVerifyFunc(tc.verify), withVMClientGetter(getVMClient))
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

	tlsConfig, err := fixtures.LocalTLSConfig()
	require.NoError(t, err)

	block, _ := pem.Decode(fixtures.LocalhostKey)
	pkey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	require.NoError(t, err)

	tlsPublicKey, err := PrivateKeyToPublicKeyTLS(sshPrivateKey)
	require.NoError(t, err)

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
		verify                         azureVerifyTokenFunc
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
			certs:       []*x509.Certificate{tlsConfig.Certificate},
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
			certs:       []*x509.Certificate{tlsConfig.Certificate},
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
			certs:       []*x509.Certificate{tlsConfig.Certificate},
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
			certs:       []*x509.Certificate{tlsConfig.Certificate},
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
			certs:       []*x509.Certificate{tlsConfig.Certificate},
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
			certs:       []*x509.Certificate{tlsConfig.Certificate},
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
			certs:       []*x509.Certificate{tlsConfig.Certificate},
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
			certs:       []*x509.Certificate{tlsConfig.Certificate},
			assertError: isAccessDenied,
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
			accessToken, err := makeToken(mirID, azrID, a.clock.Now())
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

				ad := attestedData{
					Nonce:          cfg.Challenge,
					SubscriptionID: tc.tokenSubscription,
					ID:             tc.tokenVMID,
				}
				adBytes, err := json.Marshal(&ad)
				require.NoError(t, err)
				s, err := pkcs7.NewSignedData(adBytes)
				require.NoError(t, err)
				require.NoError(t, s.AddSigner(tlsConfig.Certificate, pkey, pkcs7.SignerInfoConfig{}))
				signature, err := s.Finish()
				require.NoError(t, err)
				signedAD := signedAttestedData{
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
			}, withCerts(tc.certs), withVerifyFunc(tc.verify), withVMClientGetter(getVMClient))
			tc.assertError(t, err)
		})
	}
}
