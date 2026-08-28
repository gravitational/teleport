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

package local

import (
	"context"

	"github.com/gravitational/trace"

	accessgraphsecretspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	authorizedKeysPrefix = "access_graph_ssh_authorized_keys"
	privateKeysPrefix    = "access_graph_ssh_private_keys"
)

// AccessGraphSecretsService manages secrets found on Teleport Nodes and
// enrolled devices.
type AccessGraphSecretsService struct {
	authorizedKeysSvc *generic.ServiceWrapper[*accessgraphsecretspb.AuthorizedKey]
	privateKeysSvc    *generic.ServiceWrapper[*accessgraphsecretspb.PrivateKey]
}

// NewAccessGraphSecretsService returns a new Access Graph Secrets service.
// This service in Teleport is used to keep track of secrets found in Teleport
// Nodes and on enrolled devices. Currently, it only stores secrets related with
// SSH Keys. Future implementations might extend them.
func NewAccessGraphSecretsService(b backend.Backend) (*AccessGraphSecretsService, error) {
	authorizedKeysSvc, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*accessgraphsecretspb.AuthorizedKey]{
			Backend:       b,
			ResourceKind:  types.KindAccessGraphSecretAuthorizedKey,
			BackendPrefix: backend.NewKey(authorizedKeysPrefix),
			MarshalFunc:   services.MarshalAccessGraphAuthorizedKey,
			UnmarshalFunc: services.UnmarshalAccessGraphAuthorizedKey,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	privateKeysSvc, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*accessgraphsecretspb.PrivateKey]{
			Backend:       b,
			ResourceKind:  types.KindAccessGraphSecretPrivateKey,
			BackendPrefix: backend.NewKey(privateKeysPrefix),
			MarshalFunc:   services.MarshalAccessGraphPrivateKey,
			UnmarshalFunc: services.UnmarshalAccessGraphPrivateKey,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AccessGraphSecretsService{
		authorizedKeysSvc: authorizedKeysSvc,
		privateKeysSvc:    privateKeysSvc,
	}, nil
}

// ListAllAuthorizedKeys lists all authorized keys stored in the backend.
func (k *AccessGraphSecretsService) ListAllAuthorizedKeys(ctx context.Context, pageSize int, pageToken string) ([]*accessgraphsecretspb.AuthorizedKey, string, error) {
	out, next, err := k.authorizedKeysSvc.ListResources(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return out, next, nil
}

// ListAuthorizedKeysForServer lists all authorized keys for a given hostID.
func (k *AccessGraphSecretsService) ListAuthorizedKeysForServer(ctx context.Context, hostID string, pageSize int, pageToken string) ([]*accessgraphsecretspb.AuthorizedKey, string, error) {
	if hostID == "" {
		return nil, "", trace.BadParameter("server name is required")
	}
	svc := k.authorizedKeysSvc.WithPrefix(hostID)
	out, next, err := svc.ListResources(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return out, next, nil
}

// UpsertAuthorizedKey upserts a new authorized key.
func (k *AccessGraphSecretsService) UpsertAuthorizedKey(ctx context.Context, in *accessgraphsecretspb.AuthorizedKey) (*accessgraphsecretspb.AuthorizedKey, error) {
	svc := k.authorizedKeysSvc.WithPrefix(in.Spec.HostId)
	out, err := svc.UpsertResource(ctx, in)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}

// DeleteAuthorizedKey deletes a specific authorized key.
func (k *AccessGraphSecretsService) DeleteAuthorizedKey(ctx context.Context, hostID, name string) error {
	svc := k.authorizedKeysSvc.WithPrefix(hostID)
	return trace.Wrap(svc.DeleteResource(ctx, name))
}

// DeleteAllAuthorizedKeys deletes all authorized keys.
func (k *AccessGraphSecretsService) DeleteAllAuthorizedKeys(ctx context.Context) error {
	return trace.Wrap(k.authorizedKeysSvc.DeleteAllResources(ctx))
}

// ListAllPrivateKeys lists all private keys stored in the backend.
func (k *AccessGraphSecretsService) ListAllPrivateKeys(ctx context.Context, pageSize int, pageToken string) ([]*accessgraphsecretspb.PrivateKey, string, error) {
	out, next, err := k.privateKeysSvc.ListResources(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return out, next, nil
}

// ListPrivateKeysForDevice lists all private keys for a given deviceID.
func (k *AccessGraphSecretsService) ListPrivateKeysForDevice(ctx context.Context, deviceID string, pageSize int, pageToken string) ([]*accessgraphsecretspb.PrivateKey, string, error) {
	if deviceID == "" {
		return nil, "", trace.BadParameter("server name is required")
	}
	svc := k.privateKeysSvc.WithPrefix(deviceID)
	out, next, err := svc.ListResources(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return out, next, nil
}

// UpsertPrivateKey upserts a new private key.
func (k *AccessGraphSecretsService) UpsertPrivateKey(ctx context.Context, in *accessgraphsecretspb.PrivateKey) (*accessgraphsecretspb.PrivateKey, error) {
	svc := k.privateKeysSvc.WithPrefix(in.Spec.DeviceId)
	out, err := svc.UpsertResource(ctx, in)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}

// DeletePrivateKey deletes a specific private key.
func (k *AccessGraphSecretsService) DeletePrivateKey(ctx context.Context, deviceID, name string) error {
	svc := k.privateKeysSvc.WithPrefix(deviceID)
	return trace.Wrap(svc.DeleteResource(ctx, name))
}

// DeleteAllPrivateKeys deletes all private keys.
func (k *AccessGraphSecretsService) DeleteAllPrivateKeys(ctx context.Context) error {
	return trace.Wrap(k.privateKeysSvc.DeleteAllResources(ctx))
}
