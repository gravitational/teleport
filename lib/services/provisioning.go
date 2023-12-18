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

package services

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// Provisioner governs adding new nodes to the cluster
type Provisioner interface {
	// UpsertToken adds provisioning tokens for the auth server
	UpsertToken(ctx context.Context, token types.ProvisionToken) error

	// CreateToken adds provisioning tokens for the auth server
	CreateToken(ctx context.Context, token types.ProvisionToken) error

	// GetToken finds and returns token by id
	GetToken(ctx context.Context, token string) (types.ProvisionToken, error)

	// DeleteToken deletes provisioning token
	// Imlementations must guarantee that this returns trace.NotFound error if the token doesn't exist
	DeleteToken(ctx context.Context, token string) error

	// DeleteAllTokens deletes all provisioning tokens
	DeleteAllTokens() error

	// GetTokens returns all non-expired tokens
	GetTokens(ctx context.Context) ([]types.ProvisionToken, error)
}

// MustCreateProvisionToken returns a new valid provision token
// or panics, used in tests
func MustCreateProvisionToken(token string, roles types.SystemRoles, expires time.Time) types.ProvisionToken {
	t, err := types.NewProvisionToken(token, roles, expires)
	if err != nil {
		panic(err)
	}
	return t
}

// UnmarshalProvisionToken unmarshals the ProvisionToken resource from JSON.
func UnmarshalProvisionToken(data []byte, opts ...MarshalOption) (types.ProvisionToken, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing provision token data")
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var h types.ResourceHeader
	err = utils.FastUnmarshal(data, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch h.Version {
	case "":
		var p types.ProvisionTokenV1
		err := utils.FastUnmarshal(data, &p)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		v2 := p.V2()
		if cfg.ID != 0 {
			v2.SetResourceID(cfg.ID)
		}
		if cfg.Revision != "" {
			v2.SetRevision(cfg.Revision)
		}
		return v2, nil
	case types.V2:
		var p types.ProvisionTokenV2
		if err := utils.FastUnmarshal(data, &p); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		if err := p.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			p.SetResourceID(cfg.ID)
		}
		if cfg.Revision != "" {
			p.SetRevision(cfg.Revision)
		}
		return &p, nil
	}
	return nil, trace.BadParameter("server resource version %v is not supported", h.Version)
}

// MarshalProvisionToken marshals the ProvisionToken resource to JSON.
func MarshalProvisionToken(provisionToken types.ProvisionToken, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch provisionToken := provisionToken.(type) {
	case *types.ProvisionTokenV2:
		if err := provisionToken.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		provisionToken = maybeResetProtoResourceID(cfg.PreserveResourceID, provisionToken)
		if cfg.GetVersion() == types.V1 {
			return utils.FastMarshal(provisionToken.V1())
		}
		return utils.FastMarshal(provisionToken)
	default:
		return nil, trace.BadParameter("unrecognized provision token version %T", provisionToken)
	}
}
