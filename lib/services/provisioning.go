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
	"fmt"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// Provisioner governs adding new nodes to the cluster
type Provisioner interface {
	// UpsertToken adds provisioning tokens for the auth server
	UpsertToken(ProvisionToken) error

	// GetToken finds and returns token by id
	GetToken(token string) (ProvisionToken, error)

	// DeleteToken deletes provisioning token
	DeleteToken(token string) error

	// DeleteAllTokens deletes all provisioning tokens
	DeleteAllTokens() error

	// GetTokens returns all non-expired tokens
	GetTokens(opts ...MarshalOption) ([]ProvisionToken, error)
}

// MustCreateProvisionToken returns a new valid provision token
// or panics, used in testes
func MustCreateProvisionToken(token string, roles types.TeleportRoles, expires time.Time) ProvisionToken {
	t, err := NewProvisionToken(token, roles, expires)
	if err != nil {
		panic(err)
	}
	return t
}

// ProvisionTokenSpecV2Schema is a JSON schema for provision token
const ProvisionTokenSpecV2Schema = `{
	"type": "object",
	"additionalProperties": false,
	"properties": {
	  "roles": {"type": "array", "items": {"type": "string"}}
	}
  }`

// GetProvisionTokenSchema returns provision token schema
func GetProvisionTokenSchema() string {
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, ProvisionTokenSpecV2Schema, DefaultDefinitions)
}

// UnmarshalProvisionToken unmarshals the ProvisionToken resource.
func UnmarshalProvisionToken(data []byte, opts ...MarshalOption) (ProvisionToken, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing provision token data")
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var h ResourceHeader
	err = utils.FastUnmarshal(data, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch h.Version {
	case "":
		var p ProvisionTokenV1
		err := utils.FastUnmarshal(data, &p)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		v2 := p.V2()
		if cfg.ID != 0 {
			v2.SetResourceID(cfg.ID)
		}
		return v2, nil
	case V2:
		var p ProvisionTokenV2
		if cfg.SkipValidation {
			if err := utils.FastUnmarshal(data, &p); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		} else {
			if err := utils.UnmarshalWithSchema(GetProvisionTokenSchema(), &p, data); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		}
		if err := p.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			p.SetResourceID(cfg.ID)
		}
		return &p, nil
	}
	return nil, trace.BadParameter("server resource version %v is not supported", h.Version)
}

// MarshalProvisionToken marshals the ProvisionToken resource.
func MarshalProvisionToken(t ProvisionToken, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	type token1 interface {
		V1() *ProvisionTokenV1
	}
	type token2 interface {
		V2() *ProvisionTokenV2
	}

	version := cfg.GetVersion()
	switch version {
	case V1:
		v, ok := t.(token1)
		if !ok {
			return nil, trace.BadParameter("don't know how to marshal %v", V1)
		}
		return utils.FastMarshal(v.V1())
	case V2:
		v, ok := t.(token2)
		if !ok {
			return nil, trace.BadParameter("don't know how to marshal %v", V2)
		}
		return utils.FastMarshal(v.V2())
	default:
		return nil, trace.BadParameter("version %v is not supported", version)
	}
}
