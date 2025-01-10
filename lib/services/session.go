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
		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, webSession))
	default:
		return nil, trace.BadParameter("unrecognized web session version %T", webSession)
	}
}

// MarshalWebToken serializes the web token as JSON-encoded payload
func MarshalWebToken(webToken types.WebToken, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch webToken := webToken.(type) {
	case *types.WebTokenV3:
		if err := webToken.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, webToken))
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
		if config.Revision != "" {
			token.SetRevision(config.Revision)
		}
		if !config.Expires.IsZero() {
			token.Metadata.SetExpiry(config.Expires)
		}
		apiutils.UTC(token.Metadata.Expires)
		return &token, nil
	}
	return nil, trace.BadParameter("web token resource version %v is not supported", hdr.Version)
}
