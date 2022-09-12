/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
		// TODO: Determine if we can remove support for V1.
		// We don't support Marshalling ProvisionTokenV1 any more, and should
		// probably aim to remove this.
		var p types.ProvisionTokenV1
		err := utils.FastUnmarshal(data, &p)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		v3 := p.V3()
		if cfg.ID != 0 {
			v3.SetResourceID(cfg.ID)
		}
		return v3, nil
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
		// For now, V2 and V3 will be supported, so V2 does not need to be
		// migrated to V3 just yet. At a later date once V2 is deprecated,
		// we should convert V2 to V3 here.
		// Migrating V2 straight to V3 without a warning period may cause
		// unexpected behaviour for users who submit a V2 spec and then
		// discover a V3 resource being returned.
		return &p, nil
	case types.V3:
		var p types.ProvisionTokenV3
		if err := utils.FastUnmarshal(data, &p); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		if err := p.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			p.SetResourceID(cfg.ID)
		}
		return &p, nil
	}
	return nil, trace.BadParameter(
		"server resource version %v is not supported",
		h.Version,
	)
}

// MarshalProvisionToken marshals the ProvisionToken resource to JSON.
func MarshalProvisionToken(provisionToken types.ProvisionToken, opts ...MarshalOption) ([]byte, error) {
	if err := provisionToken.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch provisionToken := provisionToken.(type) {
	case *types.ProvisionTokenV2:
		// V2 continues to be supported. At a later date, once deprecation has
		// been announced we should reject the marshalling of V2 specifications
		// to prevent them entering the store.
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *provisionToken
			copy.SetResourceID(0)
			provisionToken = &copy
		}
		return utils.FastMarshal(provisionToken)
	case *types.ProvisionTokenV3:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *provisionToken
			copy.SetResourceID(0)
			provisionToken = &copy
		}
		return utils.FastMarshal(provisionToken)
	default:
		return nil, trace.BadParameter(
			"unrecognized provision token version %T", provisionToken,
		)
	}
}
