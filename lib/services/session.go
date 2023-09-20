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
	"encoding/json"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/utils"
)

// UnmarshalWebSession unmarshals the WebSession resource from JSON.
func UnmarshalWebSession(bytes []byte, opts ...MarshalOption) (types.WebSession, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var h types.ResourceHeader
	err = json.Unmarshal(bytes, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V2:
		var ws types.WebSessionV2
		if err := utils.FastUnmarshal(bytes, &ws); err != nil {
			return nil, trace.Wrap(err)
		}
		apiutils.UTC(&ws.Spec.BearerTokenExpires)
		apiutils.UTC(&ws.Spec.Expires)

		if err := ws.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			ws.SetResourceID(cfg.ID)
		}
		if cfg.Revision != "" {
			ws.SetRevision(cfg.Revision)
		}
		if !cfg.Expires.IsZero() {
			ws.SetExpiry(cfg.Expires)
		}

		return &ws, nil
	}

	return nil, trace.BadParameter("web session resource version %v is not supported", h.Version)
}

// MarshalWebSession marshals the WebSession resource to JSON.
func MarshalWebSession(webSession types.WebSession, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch webSession := webSession.(type) {
	case *types.WebSessionV2:
		if version := webSession.GetVersion(); version != types.V2 {
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
func MarshalWebToken(webToken types.WebToken, opts ...MarshalOption) ([]byte, error) {
	if err := webToken.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch webToken := webToken.(type) {
	case *types.WebTokenV3:
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
func UnmarshalWebToken(bytes []byte, opts ...MarshalOption) (types.WebToken, error) {
	config, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var hdr types.ResourceHeader
	err = json.Unmarshal(bytes, &hdr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch hdr.Version {
	case types.V3:
		var token types.WebTokenV3
		if err := utils.FastUnmarshal(bytes, &token); err != nil {
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
		apiutils.UTC(token.Metadata.Expires)
		return &token, nil
	}
	return nil, trace.BadParameter("web token resource version %v is not supported", hdr.Version)
}
