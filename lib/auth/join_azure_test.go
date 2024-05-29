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

func withVMClient(vmClient azure.VirtualMachinesClient) azureRegisterOption {
	return func(cfg *azureRegisterConfig) {
		cfg.vmClient = vmClient
	}
}

type mockAzureVMClient struct {
	azure.VirtualMachinesClient
	vm *azure.VirtualMachine
}

func (m *mockAzureVMClient) Get(_ context.Context, _ string) (*azure.VirtualMachine, error) {
	return m.vm, nil
}

func (m *mockAzureVMClient) GetByVMID(_ context.Context, _, _ string) (*azure.VirtualMachine, error) {
	return m.vm, nil
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

func resourceID(subscription, resourceGroup, name string) string {
	return fmt.Sprintf(
		"/subscriptions/%v/resourcegroups/%v/providers/Microsoft.Compute/virtualMachines/%v",
		subscription, resourceGroup, name,
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

func makeToken(resourceID string, issueTime time.Time) (string, error) {
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
		ResourceID: resourceID,
		TenantID:   "test-tenant-id",
		Version:    "1.0",
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

	isAccessDenied := func(t require.TestingT, err error, _ ...interface{}) {
		require.True(t, trace.IsAccessDenied(err), "expected Access Denied error, actual error: %v", err)
	}
	isBadParameter := func(t require.TestingT, err error, _ ...interface{}) {
		require.True(t, trace.IsBadParameter(err), "expected Bad Parameter error, actual error: %v", err)
	}

	subID := uuid.NewString()

	tests := []struct {
		name                     string
		subscription             string
		resourceGroup            string
		vmID                     string
		tokenName                string
		requestTokenName         string
		tokenSpec                types.ProvisionTokenSpecV2
		challengeResponseOptions []azureChallengeResponseOption
		challengeResponseErr     error
		certs                    []*x509.Certificate
		verify                   azureVerifyTokenFunc
		vmResult                 *azure.VirtualMachine
		assertError              require.ErrorAssertionFunc
	}{
		{
			name:             "basic passing case",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			subscription:     subID,
			resourceGroup:    "RG",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription:   subID,
							ResourceGroups: []string{"rg"},
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
			name:             "resource group is case insensitive",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			subscription:     subID,
			resourceGroup:    "my-RESOURCE-GROUP",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription:   subID,
							ResourceGroups: []string{"MY-resource-group"},
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
			name:             "wrong token",
			tokenName:        "test-token",
			requestTokenName: "wrong-token",
			subscription:     subID,
			resourceGroup:    "RG",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription: subID,
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
			name:             "challenge response error",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			subscription:     subID,
			resourceGroup:    "RG",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription: subID,
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
			name:             "wrong subscription",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			subscription:     "some-junk",
			resourceGroup:    "RG",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription: subID,
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
			name:             "wrong resource group",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			subscription:     subID,
			resourceGroup:    "WRONG-RG",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription:   subID,
							ResourceGroups: []string{"rg"},
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
			name:             "wrong challenge",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			subscription:     subID,
			resourceGroup:    "RG",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription: subID,
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
			name:             "invalid signature",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			subscription:     subID,
			resourceGroup:    "RG",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription: subID,
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
			name:             "attested data and access token from different VMs",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			subscription:     subID,
			resourceGroup:    "RG",
			vmID:             "vm-id",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Azure: &types.ProvisionTokenSpecV2Azure{
					Allow: []*types.ProvisionTokenSpecV2Azure_Rule{
						{
							Subscription: subID,
						},
					},
				},
				JoinMethod: types.JoinMethodAzure,
			},
			vmResult: &azure.VirtualMachine{
				Subscription:  subID,
				ResourceGroup: "RG",
				VMID:          "different-id",
			},
			verify:      mockVerifyToken(nil),
			certs:       []*x509.Certificate{tlsConfig.Certificate},
			assertError: isAccessDenied,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			token, err := types.NewProvisionTokenFromSpec(
				tc.tokenName,
				time.Now().Add(time.Minute),
				tc.tokenSpec)
			require.NoError(t, err)
			require.NoError(t, a.UpsertToken(ctx, token))
			t.Cleanup(func() {
				require.NoError(t, a.DeleteToken(ctx, token.GetName()))
			})

			rsID := resourceID(tc.subscription, tc.resourceGroup, "test-vm")

			accessToken, err := makeToken(rsID, a.clock.Now())
			require.NoError(t, err)

			vmResult := tc.vmResult
			if vmResult == nil {
				vmResult = &azure.VirtualMachine{
					ID:            rsID,
					Name:          "test-vm",
					Subscription:  tc.subscription,
					ResourceGroup: tc.resourceGroup,
					VMID:          tc.vmID,
				}
			}

			vmClient := &mockAzureVMClient{vm: vmResult}

			_, err = a.RegisterUsingAzureMethod(context.Background(), func(challenge string) (*proto.RegisterUsingAzureMethodRequest, error) {
				cfg := &azureChallengeResponseConfig{Challenge: challenge}
				for _, opt := range tc.challengeResponseOptions {
					opt(cfg)
				}

				ad := attestedData{
					Nonce:          cfg.Challenge,
					SubscriptionID: subID,
					ID:             tc.vmID,
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
			}, withCerts(tc.certs), withVerifyFunc(tc.verify), withVMClient(vmClient))
			tc.assertError(t, err)
		})
	}
}
