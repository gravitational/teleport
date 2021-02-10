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
	"encoding/json"
	"fmt"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// WebSessionSpecV2Schema is JSON schema for cert authority V2
const WebSessionSpecV2Schema = `{
	"type": "object",
	"additionalProperties": false,
	"required": ["pub", "bearer_token", "bearer_token_expires", "expires", "user"],
	"properties": {
	  "user": {"type": "string"},
	  "pub": {"type": "string"},
	  "priv": {"type": "string"},
	  "tls_cert": {"type": "string"},
	  "bearer_token": {"type": "string"},
	  "bearer_token_expires": {"type": "string"},
	  "expires": {"type": "string"}%v
	}
  }`

// GetWebSessionSchema returns JSON Schema for web session
func GetWebSessionSchema() string {
	return GetWebSessionSchemaWithExtensions("")
}

// GetWebSessionSchemaWithExtensions returns JSON Schema for web session with user-supplied extensions
func GetWebSessionSchemaWithExtensions(extension string) string {
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, fmt.Sprintf(WebSessionSpecV2Schema, extension), DefaultDefinitions)
}

// ExtendWebSession renews web session and is used to
// inject additional data in extenstions when session is getting renewed
func ExtendWebSession(ws WebSession) (WebSession, error) {
	return ws, nil
}

// UnmarshalWebSession unmarshals the WebSession resource from JSON.
func UnmarshalWebSession(bytes []byte, opts ...auth.MarshalOption) (types.WebSession, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var h ResourceHeader
	err = json.Unmarshal(bytes, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case V2:
		var ws types.WebSessionV2
		if err := utils.UnmarshalWithSchema(GetWebSessionSchema(), &ws, bytes); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		utils.UTC(&ws.Spec.BearerTokenExpires)
		utils.UTC(&ws.Spec.Expires)

		if err := ws.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			ws.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			ws.SetExpiry(cfg.Expires)
		}

		return &ws, nil
	}

	return nil, trace.BadParameter("web session resource version %v is not supported", h.Version)
}

// MarshalWebSession marshals the WebSession resource to JSON.
func MarshalWebSession(webSession types.WebSession, opts ...auth.MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch webSession := webSession.(type) {
	case *WebSessionV2:
		if version := webSession.GetVersion(); version != V2 {
			return nil, trace.BadParameter("mismatched web session version %v and type %T", version, webSession)
		}
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *webSession
			copy.SetResourceID(0)
			webSession = &copy
		}
		return utils.FastMarshal(webSession)
	default:
		return nil, trace.BadParameter("unrecognized web session version %T", webSession)
	}
}

// MarshalWebToken serializes the web token as JSON-encoded payload
func MarshalWebToken(webToken types.WebToken, opts ...auth.MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch webToken := webToken.(type) {
	case *types.WebTokenV3:
		if version := webToken.GetVersion(); version != V3 {
			return nil, trace.BadParameter("mismatched web token version %v and type %T", version, webToken)
		}
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *webToken
			copy.SetResourceID(0)
			webToken = &copy
		}
		return utils.FastMarshal(webToken)
	default:
		return nil, trace.BadParameter("unrecognized web token version %T", webToken)
	}
}

// UnmarshalWebToken interprets bytes as JSON-encoded web token value
func UnmarshalWebToken(bytes []byte, opts ...auth.MarshalOption) (types.WebToken, error) {
	config, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var hdr ResourceHeader
	err = json.Unmarshal(bytes, &hdr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch hdr.Version {
	case V3:
		var token types.WebTokenV3
		if err := utils.UnmarshalWithSchema(GetWebTokenSchema(), &token, bytes); err != nil {
			return nil, trace.BadParameter("invalid web token: %v", err.Error())
		}
		if err := token.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if config.ID != 0 {
			token.SetResourceID(config.ID)
		}
		if !config.Expires.IsZero() {
			token.Metadata.SetExpiry(config.Expires)
		}
		utils.UTC(token.Metadata.Expires)
		return &token, nil
	}
	return nil, trace.BadParameter("web token resource version %v is not supported", hdr.Version)
}

// GetWebTokenSchema returns JSON schema for the web token resource
func GetWebTokenSchema() string {
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, WebTokenSpecV3Schema, "")
}

// WebTokenSpecV3Schema is JSON schema for the web token V3
const WebTokenSpecV3Schema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["token", "user"],
  "properties": {
    "user": {"type": "string"},
    "token": {"type": "string"}
  }
}`
