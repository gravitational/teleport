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
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/join/azurejoin"
	"github.com/gravitational/teleport/lib/join/legacyjoin"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils"
)

// RegisterUsingAzureMethod registers the caller using the Azure join method
// and returns signed certs to join the cluster.
//
// The caller must provide a ChallengeResponseFunc which returns a
// *proto.RegisterUsingAzureMethodRequest with a signed attested data document
// including the challenge as a nonce.
//
// TODO(nklaassen): DELETE IN 20 when removing the legacy join service.
func (a *Server) RegisterUsingAzureMethod(
	ctx context.Context,
	challengeResponse client.RegisterAzureChallengeResponseFunc,
) (certs *proto.Certs, err error) {
	var provisionToken types.ProvisionToken
	var joinRequest *types.RegisterUsingTokenRequest
	defer func() {
		// Emit a log message and audit event on join failure.
		if err != nil {
			a.handleJoinFailure(ctx, err, provisionToken, nil, joinRequest)
		}
	}()

	if legacyjoin.Disabled() {
		return nil, trace.Wrap(legacyjoin.ErrDisabled)
	}

	challenge, err := azurejoin.GenerateAzureChallenge()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req, err := challengeResponse(challenge)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	joinRequest = req.RegisterUsingTokenRequest

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	provisionToken, err = a.checkTokenJoinRequestCommon(ctx, req.RegisterUsingTokenRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if provisionToken.GetJoinMethod() != types.JoinMethodAzure {
		return nil, trace.AccessDenied("this token does not support the Azure join method")
	}

	ptv2, ok := provisionToken.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.Wrap(err, "Azure join method only supports ProvisionTokenV2, got %T", provisionToken)
	}

	joinAttrs, err := azurejoin.CheckAzureRequest(ctx, azurejoin.CheckAzureRequestParams{
		AzureJoinConfig: a.GetAzureJoinConfig(),
		Token:           ptv2,
		Challenge:       challenge,
		AttestedData:    req.AttestedData,
		AccessToken:     req.AccessToken,
		Logger:          a.logger,
		Clock:           a.GetClock(),
	})
	if err != nil {
		return nil, trace.Wrap(err, "checking Azure challenge response")
	}

	if req.RegisterUsingTokenRequest.Role == types.RoleBot {
		params := makeBotCertsParams(req.RegisterUsingTokenRequest, nil /*rawClaims*/, &workloadidentityv1pb.JoinAttrs{
			Azure: joinAttrs,
		})
		certs, _, err := a.GenerateBotCertsForJoin(ctx, provisionToken, params)
		return certs, trace.Wrap(err)
	}
	params := makeHostCertsParams(req.RegisterUsingTokenRequest, nil /*rawClaims*/)
	certs, err = a.GenerateHostCertsForJoin(ctx, provisionToken, params)
	return certs, trace.Wrap(err)
}

// GetAzureJoinConfig gets configuration options for azure joining.
func (a *Server) GetAzureJoinConfig() *azurejoin.AzureJoinConfig {
	a.lock.Lock()
	defer a.lock.Unlock()
	if a.azureJoinConfigOverride != nil {
		return a.azureJoinConfigOverride
	}
	return &azurejoin.AzureJoinConfig{
		GetSubscriptionClient: func(ctx context.Context, integration string) (azure.SubscriptionClient, error) {
			// For Azure join flow, the join token might allow wildcard
			// subscriptions which requires listing the Azure subscriptions
			// to check that a VM is allowed to join the cluster.
			// This requires Azure credentials to be accessible to Auth.
			// The credentials can come from an integration or from ambient
			// credentials, when an integration is not specified in the join
			// token.
			//
			// Using ambient credentials when the Auth Service is running
			// within Teleport Cloud is not supported.
			// In that scenario a NotImplemented error is returned.
			if integration == "" && modules.GetModules().Features().Cloud {
				return nil, trace.NotImplemented("Azure subscriptions cannot be listed on Teleport Cloud without an Azure OIDC integration included in the join token spec")
			}
			subClient, err := a.getAzureSubscriptionClient(ctx, integration)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return subClient, nil
		},
	}
}

// SetAzureJoinConfig sets configuration options for azure joining.
func (a *Server) SetAzureJoinConfig(c *azurejoin.AzureJoinConfig) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.azureJoinConfigOverride = c
}

func (a *Server) getAzureSubscriptionClient(ctx context.Context, integration string) (azure.SubscriptionClient, error) {
	subClient, err := utils.FnCacheGet(ctx, a.azureClientCache, integration,
		func(ctx context.Context) (azure.SubscriptionClient, error) {
			var opts []azure.ClientsOption
			if integration != "" {
				opts = append(opts, azure.WithIntegrationCredentials(integration, a))
			}
			clients, err := azure.NewClients(opts...)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			subClient, err := clients.GetSubscriptionClient(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &listSubscriptionsOnceClient{subClient: subClient}, nil
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return subClient, nil
}

// listSubscriptionsOnceClient performs ListSubscriptionIDs only once and caches
// the result for all future calls. It can be returned from a TTL cache to
// provide Azure API response caching with a TTL.
type listSubscriptionsOnceClient struct {
	subClient azure.SubscriptionClient

	listOnce      sync.Once
	subscriptions []string
	err           error
}

func (c *listSubscriptionsOnceClient) ListSubscriptionIDs(ctx context.Context) ([]string, error) {
	c.listOnce.Do(func() {
		subs, err := c.subClient.ListSubscriptionIDs(ctx)
		c.subscriptions, c.err = subs, trace.Wrap(err)
	})
	return c.subscriptions, c.err
}
