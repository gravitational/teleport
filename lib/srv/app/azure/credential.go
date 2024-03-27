/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package azure

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitational/trace"

	cloudazure "github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/utils"
)

// credentialProvider defines an interface that manages a particular type of
// credential.
type credentialProvider interface {
	// MakeCredential creates an azcore.TokenCredential for provided identity.
	MakeCredential(ctx context.Context, userRequestedIdentity string) (azcore.TokenCredential, error)
	// MapScope maps the input scope if necessary.
	MapScope(scope string) string
}

func getAccessTokenFromCredentialProvider(credProvider credentialProvider) getAccessTokenFunc {
	return func(ctx context.Context, userRequestedIdentity string, scope string) (*azcore.AccessToken, error) {
		credential, err := credProvider.MakeCredential(ctx, userRequestedIdentity)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		opts := policy.TokenRequestOptions{
			Scopes: []string{credProvider.MapScope(scope)},
		}
		token, err := credential.GetToken(ctx, opts)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &token, nil
	}
}

func findDefaultCredentialProvider(ctx context.Context) (credentialProvider, error) {
	// Check if default workload identity is available: the clientID/tenantID
	// for the default workload identity and the token file path are required
	// from environment variables.
	defaultAgentIdentity, err := azidentity.NewWorkloadIdentityCredential(nil)
	if err == nil {
		slog.InfoContext(ctx, "Using workload identity.")
		credProvider, err := newWorloadIdentityCredentialProvider(ctx, defaultAgentIdentity)
		return credProvider, trace.Wrap(err)
	} else {
		slog.With("error", err).DebugContext(ctx, "Failed to load worload identity.")
	}

	slog.InfoContext(ctx, "Using managed identity.")
	return managedIdentityCredentialProvider{}, nil

}

// managedIdentityCredentialProvider implements credentialProvider for using
// managed identities assigned to the host machine. Identities are usually
// checked against the IMDS service available in local network.
type managedIdentityCredentialProvider struct {
}

func (m managedIdentityCredentialProvider) MakeCredential(ctx context.Context, userRequestedIdentity string) (azcore.TokenCredential, error) {
	credenial, err := azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{
		ID: azidentity.ResourceID(userRequestedIdentity),
	})
	return credenial, trace.Wrap(err)
}

func (m managedIdentityCredentialProvider) MapScope(scope string) string {
	// No scope needs to be mapped.
	return scope
}

// workloadIdentityCredentialProvider implements credentialProvider for using
// workload identities assigned to host machine.
//
// https://learn.microsoft.com/en-us/azure/aks/workload-identity-overview
//
// When running on AKS, multiple workload identities can be associated to the
// same service account attached to the pod. Assuming a oorkload identity
// requires a Client ID of that identity but only the default Client ID is
// provided through environment variable. We assume that the default workload
// identity (mapped by the default Client ID) is the app-service identity with
// msi permissions so the client IDs for other user-requested identity can be
// retrieved using the default idenitty.
type workloadIdentityCredentialProvider struct {
	cache                *utils.FnCache
	defaultAgentIdentity azcore.TokenCredential

	// newClient defaults to cloudazure.NewUserAssignedIdentitiesClient. Can be
	// overridden for test.
	newClient func(string, azcore.TokenCredential, *arm.ClientOptions) (*cloudazure.UserAssignedIdentitiesClient, error)
	// newCredential defaults to newWorkloadIdentityCredentialForClientID. Can
	// be overridden for test.
	newCredential func(string) (azcore.TokenCredential, error)
}

func newWorloadIdentityCredentialProvider(ctx context.Context, defaultAgentIdentity azcore.TokenCredential) (*workloadIdentityCredentialProvider, error) {
	cache, err := utils.NewFnCache(utils.FnCacheConfig{
		Context:     ctx,
		TTL:         clientIDCacheTTL,
		ReloadOnErr: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &workloadIdentityCredentialProvider{
		cache:                cache,
		defaultAgentIdentity: defaultAgentIdentity,
		newClient:            cloudazure.NewUserAssignedIdentitiesClient,
		newCredential:        newWorkloadIdentityCredentialForClientID,
	}, nil
}

func newWorkloadIdentityCredentialForClientID(clientID string) (azcore.TokenCredential, error) {
	cred, err := azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
		ClientID: clientID,
	})
	return cred, trace.Wrap(err)
}

func (w *workloadIdentityCredentialProvider) MakeCredential(ctx context.Context, userRequestedIdentity string) (azcore.TokenCredential, error) {
	clientID, err := w.getClientID(ctx, userRequestedIdentity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	credential, err := w.newCredential(clientID)
	return credential, trace.Wrap(err)
}

func (w *workloadIdentityCredentialProvider) MapScope(scope string) string {
	// This scope ("https://management.core.windows.net/") from `az` CLI tool
	// will fail for worload identity as worload identity is only expected to
	// be used with compatible SDKs, whereas the SDK adds ".default" to the
	// scope:
	//
	// https://github.com/Azure/azure-sdk-for-go/blob/9e78ee2b86f0f4989098dd7e545b73841fc8df47/sdk/azcore/arm/runtime/pipeline.go#L35
	if scope == "https://management.core.windows.net/" {
		return scope + ".default"
	}
	return scope
}

func (w *workloadIdentityCredentialProvider) getClientID(ctx context.Context, identityResourceID string) (string, error) {
	clientID, err := utils.FnCacheGet(ctx, w.cache, identityResourceID, func(ctx context.Context) (string, error) {
		resourceID, err := arm.ParseResourceID(identityResourceID)
		if err != nil {
			return "", trace.Wrap(err)
		}

		client, err := w.newClient(resourceID.SubscriptionID, w.defaultAgentIdentity, nil)
		if err != nil {
			return "", trace.Wrap(err)
		}

		clientID, err := client.GetClientID(ctx, resourceID.ResourceGroupName, resourceID.Name)
		return clientID, trace.Wrap(err)
	})
	return clientID, trace.Wrap(err)
}

// clientIDCacheTTL defines how long client IDs should be cached. ClientID
// should never change for an identity so use a longer cache TTL.
var clientIDCacheTTL = 30 * time.Minute
