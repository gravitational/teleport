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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi"
	"github.com/gravitational/trace"
)

// ARMUserAssignedIdentities provides an interface for
// armmsi.UserAssignedIdentitiesClient.
type ARMUserAssignedIdentities interface {
	Get(ctx context.Context, resourceGroupName, resourceName string, options *armmsi.UserAssignedIdentitiesClientGetOptions) (armmsi.UserAssignedIdentitiesClientGetResponse, error)
}

// UserAssignedIdentitiesClient wraps the armmsi.UserAssignedIdentitiesClient to fetch
// identity info.
type UserAssignedIdentitiesClient struct {
	api ARMUserAssignedIdentities
}

// NewUserAssignedIdentitiesClient creates a new UserAssignedIdentitiesClient
// by subscription and credential.
func NewUserAssignedIdentitiesClient(subscription string, cred azcore.TokenCredential, options *arm.ClientOptions) (*UserAssignedIdentitiesClient, error) {
	api, err := armmsi.NewUserAssignedIdentitiesClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewUserAssignedIdentitiesClientByAPI(api), nil
}

// NewUserAssignedIdentitiesClientByAPI creates a new
// UserAssignedIdentitiesClient by ARMUserAssignedIdentities interface.
func NewUserAssignedIdentitiesClientByAPI(api ARMUserAssignedIdentities) *UserAssignedIdentitiesClient {
	return &UserAssignedIdentitiesClient{
		api: api,
	}
}

// GetClientID returns the client ID for the provided identity.
func (c *UserAssignedIdentitiesClient) GetClientID(ctx context.Context, resourceGroupName, resourceName string) (string, error) {
	identity, err := c.api.Get(ctx, resourceGroupName, resourceName, nil)
	if err != nil {
		return "", trace.Wrap(ConvertResponseError(err))
	}

	if identity.Properties == nil || identity.Properties.ClientID == nil {
		return "", trace.BadParameter("cannot find ClientID from identity %s", resourceName)
	}

	return *identity.Properties.ClientID, nil
}
