/*
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
	"net/url"
	"os"
	"slices"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
)

// isRunningInAKS checks if the code is running in an AKS pod with Azure Workload Identity
func isRunningInAKS() bool {
	// Check for Kubernetes environment
	_, hasKubeHost := os.LookupEnv("KUBERNETES_SERVICE_HOST")

	// Check for Azure Workload Identity environment variables
	// These are injected by the Azure Workload Identity webhook
	_, hasAzureClientID := os.LookupEnv("AZURE_CLIENT_ID")
	_, hasAzureTenantID := os.LookupEnv("AZURE_TENANT_ID")
	_, hasAzureFederatedTokenFile := os.LookupEnv("AZURE_FEDERATED_TOKEN_FILE")

	return hasKubeHost && hasAzureClientID && hasAzureTenantID && hasAzureFederatedTokenFile
}

// checkAzureRequestAKS validates an Azure join request from an AKS pod
func (a *Server) checkAzureRequestAKS(
	ctx context.Context,
	req *proto.RegisterUsingAzureMethodRequest,
	cfg *azureRegisterConfig,
) (*workloadidentityv1pb.JoinAttrsAzure, error) {
	requestStart := a.clock.Now()
	tokenName := req.RegisterUsingTokenRequest.Token
	provisionToken, err := a.GetToken(ctx, tokenName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if provisionToken.GetJoinMethod() != types.JoinMethodAzure {
		return nil, trace.AccessDenied("this token does not support the Azure join method")
	}
	token, ok := provisionToken.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter("azure join method only supports ProvisionTokenV2, '%T' was provided", provisionToken)
	}

	// For AKS, we don't have attested data, so we verify using just the access token
	tokenClaims, err := cfg.verify(ctx, req.AccessToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Validate token issuer and audience
	expectedIssuer, err := url.JoinPath("https://sts.windows.net", tokenClaims.TenantID, "/")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if tokenClaims.Version == "2.0" {
		expectedIssuer, err = url.JoinPath(expectedIssuer, "2.0")
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	expectedClaims := jwt.Expected{
		Issuer:   expectedIssuer,
		Audience: jwt.Audience{azureAccessTokenAudience},
		Time:     requestStart,
	}

	if err := tokenClaims.AsJWTClaims().Validate(expectedClaims); err != nil {
		return nil, trace.Wrap(err)
	}

	// Extract AKS information from the token claims
	attrs, err := extractAKSInfoFromClaims(tokenClaims)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Check against allow rules
	if err := checkAzureAKSAllowRules(attrs, token); err != nil {
		return attrs, trace.Wrap(err)
	}

	// Note: Unlike the VM flow, we don't make an Azure API call here because:
	// 1. We've already verified the JWT signature with Azure AD's public keys
	// 2. The token can only be obtained from within an AKS pod with the correct workload identity
	// 3. Making API calls would require giving the managed identity Azure permissions it doesn't need

	return attrs, nil
}

// extractAKSInfoFromClaims extracts AKS cluster information from Azure AD token claims
func extractAKSInfoFromClaims(claims *accessTokenClaims) (*workloadidentityv1pb.JoinAttrsAzure, error) {
	// In AKS with Workload Identity, the xms_mirid claim contains the managed identity resource ID
	// Format: /subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.ManagedIdentity/userAssignedIdentities/{identity-name}
	if claims.ManangedIdentityResourceID == "" {
		return nil, trace.BadParameter("token claims do not contain managed identity resource ID")
	}

	resourceID, err := arm.ParseResourceID(claims.ManangedIdentityResourceID)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse managed identity resource ID")
	}

	// Verify this is a managed identity resource, not a VM
	const managedIdentityType = "userAssignedIdentities"
	if !slices.Contains(resourceID.ResourceType.Types, managedIdentityType) {
		return nil, trace.BadParameter("expected managed identity resource type, got: %v", resourceID.ResourceType.Type)
	}

	// Extract subscription and resource group from the managed identity
	attrs := &workloadidentityv1pb.JoinAttrsAzure{
		Subscription:  resourceID.SubscriptionID,
		ResourceGroup: resourceID.ResourceGroupName,
	}

	return attrs, nil
}

// checkAzureAKSAllowRules checks if the AKS pod matches the token's allow rules
func checkAzureAKSAllowRules(attrs *workloadidentityv1pb.JoinAttrsAzure, token *types.ProvisionTokenV2) error {
	for _, rule := range token.Spec.Azure.Allow {
		if rule.Subscription != attrs.Subscription {
			continue
		}
		if !azureResourceGroupIsAllowed(rule.ResourceGroups, attrs.ResourceGroup) {
			continue
		}
		// TODO: Add support for matching on specific AKS cluster names or managed identity names
		// This would require extending the Azure allow rules to include these fields
		return nil
	}
	return trace.AccessDenied("AKS pod did not match any allow rules in token %v", token.GetName())
}


// RegisterUsingAzureMethodAKS handles the Azure join method for AKS pods
func (a *Server) RegisterUsingAzureMethodAKS(
	ctx context.Context,
	req *proto.RegisterUsingAzureMethodRequest,
	cfg *azureRegisterConfig,
) (certs *proto.Certs, err error) {
	var provisionToken types.ProvisionToken
	var joinRequest *types.RegisterUsingTokenRequest
	defer func() {
		// Emit a log message and audit event on join failure.
		if err != nil {
			a.handleJoinFailure(ctx, err, provisionToken, nil, joinRequest)
		}
	}()

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	joinRequest = req.RegisterUsingTokenRequest

	provisionToken, err = a.checkTokenJoinRequestCommon(ctx, req.RegisterUsingTokenRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	joinAttrs, err := a.checkAzureRequestAKS(ctx, req, cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.RegisterUsingTokenRequest.Role == types.RoleBot {
		certs, _, err := a.generateCertsBot(
			ctx,
			provisionToken,
			req.RegisterUsingTokenRequest,
			nil,
			&workloadidentityv1pb.JoinAttrs{
				Azure: joinAttrs,
			},
		)
		return certs, trace.Wrap(err)
	}
	certs, err = a.generateCerts(
		ctx,
		provisionToken,
		req.RegisterUsingTokenRequest,
		&workloadidentityv1pb.JoinAttrs{
			Azure: joinAttrs,
		},
	)
	return certs, trace.Wrap(err)
}