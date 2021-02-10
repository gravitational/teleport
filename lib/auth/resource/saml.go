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

package resource

import (
	"fmt"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// SAMLConnectorV2SchemaTemplate is a template JSON Schema for SAMLConnector
const SAMLConnectorV2SchemaTemplate = `{
	"type": "object",
	"additionalProperties": false,
	"required": ["kind", "spec", "metadata", "version"],
	"properties": {
	  "kind": {"type": "string"},
	  "version": {"type": "string", "default": "v1"},
	  "metadata": %v,
	  "spec": %v
	}
  }`

// SAMLConnectorSpecV2Schema is a JSON Schema for SAML Connector
var SAMLConnectorSpecV2Schema = fmt.Sprintf(`{
	"type": "object",
	"additionalProperties": false,
	"required": ["acs"],
	"properties": {
	  "issuer": {"type": "string"},
	  "sso": {"type": "string"},
	  "cert": {"type": "string"},
	  "provider": {"type": "string"},
	  "display": {"type": "string"},
	  "acs": {"type": "string"},
	  "audience": {"type": "string"},
	  "service_provider_issuer": {"type": "string"},
	  "entity_descriptor": {"type": "string"},
	  "entity_descriptor_url": {"type": "string"},
	  "attributes_to_roles": {
		"type": "array",
		"items": %v
	  },
	  "signing_key_pair": %v
	}
  }`, AttributeMappingSchema, SigningKeyPairSchema)

// AttributeMappingSchema is JSON schema for claim mapping
var AttributeMappingSchema = `{
	"type": "object",
	"additionalProperties": false,
	"required": ["name", "value" ],
	"properties": {
	  "name": {"type": "string"},
	  "value": {"type": "string"},
	  "roles": {
		"type": "array",
		"items": {
		  "type": "string"
		}
	  }
	}
  }`

// SigningKeyPairSchema is the JSON schema for signing key pair.
var SigningKeyPairSchema = `{
	"type": "object",
	"additionalProperties": false,
	"properties": {
	  "private_key": {"type": "string"},
	  "cert": {"type": "string"}
	}
  }`

// GetSAMLConnectorSchema returns schema for SAMLConnector
func GetSAMLConnectorSchema() string {
	return fmt.Sprintf(SAMLConnectorV2SchemaTemplate, MetadataSchema, SAMLConnectorSpecV2Schema)
}

// UnmarshalSAMLConnector unmarshals the SAMLConnector resource from JSON.
func UnmarshalSAMLConnector(bytes []byte, opts ...auth.MarshalOption) (SAMLConnector, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h ResourceHeader
	err = utils.FastUnmarshal(bytes, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case V2:
		var c SAMLConnectorV2
		if cfg.SkipValidation {
			if err := utils.FastUnmarshal(bytes, &c); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		} else {
			if err := utils.UnmarshalWithSchema(GetSAMLConnectorSchema(), &c, bytes); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		}

		if err := auth.ValidateSAMLConnector(&c); err != nil {
			return nil, trace.Wrap(err)
		}

		if cfg.ID != 0 {
			c.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			c.SetExpiry(cfg.Expires)
		}

		return &c, nil
	}

	return nil, trace.BadParameter("SAML connector resource version %v is not supported", h.Version)
}

// MarshalSAMLConnector marshals the SAMLConnector resource to JSON.
func MarshalSAMLConnector(c SAMLConnector, opts ...auth.MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch connector := c.(type) {
	case *SAMLConnectorV2:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *connector
			copy.SetResourceID(0)
			connector = &copy
		}
		return utils.FastMarshal(connector)
	default:
		return nil, trace.BadParameter("unrecognized SAMLConnector version %T", c)
	}
}
