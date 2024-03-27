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
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/utils"
)

type accessTokenGetter interface {
	GetToken(ctx context.Context, identityResourceID string, scope string) (*azcore.AccessToken, error)
}

// accessTokenGetterFunc implements accessTokenGetter.
type accessTokenGetterFunc func(context.Context, string, string) (*azcore.AccessToken, error)

func (f accessTokenGetterFunc) GetToken(ctx context.Context, identityResourceID string, scope string) (*azcore.AccessToken, error) {
	return f(ctx, identityResourceID, scope)
}

// credentialMakerFunc implements accessTokenGetter.
type credentialMakerFunc func(ctx context.Context, identityResourceID string) (azcore.TokenCredential, error)

func (f credentialMakerFunc) GetToken(ctx context.Context, identityResourceID string, scope string) (*azcore.AccessToken, error) {
	credential, err := f(ctx, identityResourceID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	opts := policy.TokenRequestOptions{Scopes: []string{scope}}
	token, err := credential.GetToken(ctx, opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &token, nil
}

func makeDefaultAccessTokenGetter(ctx context.Context, log logrus.FieldLogger) (accessTokenGetter, error) {
	// Check if default workload identity is available: the clientID/tenantID
	// for the default workload identity and the token file path are required
	// from environment variables.
	defaultWorkloadIdentity, err := azidentity.NewWorkloadIdentityCredential(nil)
	if err != nil {
		log.Infof("Default workload identity not found: %v. Using managed identity.", err)
		return credentialMakerFunc(makeManagedIdentityCredential), nil
	}

	// When running on AKS, multiple workload identities can be associated to
	// the same service account attached to the pod. Workload identity requires
	// Client ID to be assumed but only the default Client ID is provided
	// through environment variable. We assume that the default workload
	// identity (mapped by the default Client ID) is the app-service identity
	// with msi permissions so the Client IDs for other user-requested identity
	// can be retrieved.
	clientIDGetter, err := newCachedClientIDGetter(ctx, newMSIClientIDGetter(defaultWorkloadIdentity), clientIDCacheTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If default workload identity is available, try both workload identity
	// and managed identity.
	log.Info("Using both workload identity and managed identity.")
	return makeChainedCredential(
		log,
		makeWorkloadIdentityCredential(clientIDGetter),
		makeManagedIdentityCredential,
	), nil
}

// makeManagedIdentityCredential is a credentialMakerFunc for using managed
// identity credential.
func makeManagedIdentityCredential(ctx context.Context, identityResourceID string) (azcore.TokenCredential, error) {
	credenial, err := azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{
		ID: azidentity.ResourceID(identityResourceID),
	})
	return credenial, trace.Wrap(err)
}

// makeManagedIdentityCredential returns a credentialMakerFunc for using workload
// identity credential.
func makeWorkloadIdentityCredential(clientIDGetter clientIDGetter) credentialMakerFunc {
	return func(ctx context.Context, identityResourceID string) (azcore.TokenCredential, error) {
		clientID, err := clientIDGetter.GetClientID(ctx, identityResourceID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cred, err := azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
			ClientID: clientID,
		})
		return cred, trace.Wrap(err)
	}
}

// makeChainedCredential returns a credentialMakerFunc for chaining multiple
// credentialMakerFunc.
func makeChainedCredential(log logrus.FieldLogger, credentialMakers ...credentialMakerFunc) credentialMakerFunc {
	return credentialMakerFunc(func(ctx context.Context, identityResourceID string) (azcore.TokenCredential, error) {
		var sources []azcore.TokenCredential
		for _, makeCredential := range credentialMakers {
			if cred, err := makeCredential(ctx, identityResourceID); err != nil {
				log.WithError(err).WithField("identity", identityResourceID).Debugf("Failed to make credenial.")
			} else {
				sources = append(sources, cred)
			}
		}
		chained, err := azidentity.NewChainedTokenCredential(sources, nil)
		return chained, trace.Wrap(err)
	})
}

type clientIDGetter interface {
	GetClientID(ctx context.Context, identityResourceID string) (string, error)
}

type msiClientIDGetter struct {
	cred azcore.TokenCredential
}

func newMSIClientIDGetter(cred azcore.TokenCredential) *msiClientIDGetter {
	return &msiClientIDGetter{
		cred: cred,
	}
}

func (m *msiClientIDGetter) GetClientID(ctx context.Context, identityResourceID string) (string, error) {
	resourceID, err := arm.ParseResourceID(identityResourceID)
	if err != nil {
		return "", trace.Wrap(err)
	}

	client, err := armmsi.NewUserAssignedIdentitiesClient(resourceID.SubscriptionID, m.cred, nil)
	if err != nil {
		return "", trace.Wrap(err)
	}

	identity, err := client.Get(ctx, resourceID.ResourceGroupName, resourceID.Name, nil)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if identity.Properties == nil || identity.Properties.ClientID == nil {
		return "", trace.BadParameter("cannot find ClientID from identity %s", resourceID)
	}
	return *identity.Properties.ClientID, nil
}

type cachedClientIDGetter struct {
	inner clientIDGetter
	cache *utils.FnCache
}

func newCachedClientIDGetter(ctx context.Context, inner clientIDGetter, ttl time.Duration) (*cachedClientIDGetter, error) {
	cache, err := utils.NewFnCache(utils.FnCacheConfig{
		Context:     ctx,
		TTL:         ttl,
		ReloadOnErr: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &cachedClientIDGetter{
		inner: inner,
		cache: cache,
	}, nil
}

func (c *cachedClientIDGetter) GetClientID(ctx context.Context, identityResourceID string) (string, error) {
	clientID, err := utils.FnCacheGet(ctx, c.cache, identityResourceID, func(ctx context.Context) (string, error) {
		clientID, err := c.inner.GetClientID(ctx, identityResourceID)
		return clientID, trace.Wrap(err)

	})
	return clientID, trace.Wrap(err)
}

// clientIDCacheTTL defines how long clientID will be cached. ClientID should
// never change for an identity so use a longer cache TTL.
var clientIDCacheTTL = 10 * time.Minute
