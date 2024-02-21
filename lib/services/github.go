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
	"fmt"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils"
)

// ErrRequiresEnterprise indicates that a feature requires
// Teleport Enterprise.
var ErrRequiresEnterprise = &trace.AccessDeniedError{Message: "this feature requires Teleport Enterprise"}

// githubConnectorMutex is a mutex for the GitHub auth connector
// registration functions.
var githubConnectorMutex sync.RWMutex

// GithubAuthCreator creates a new GitHub connector.
type GithubAuthCreator func(string, types.GithubConnectorSpecV3) (types.GithubConnector, error)

var githubAuthCreator GithubAuthCreator

// RegisterGithubAuthCreator registers a function to create GitHub auth connectors.
func RegisterGithubAuthCreator(creator GithubAuthCreator) {
	githubConnectorMutex.Lock()
	defer githubConnectorMutex.Unlock()
	githubAuthCreator = creator
}

// NewGithubConnector creates a new GitHub auth connector.
func NewGithubConnector(name string, spec types.GithubConnectorSpecV3) (types.GithubConnector, error) {
	githubConnectorMutex.RLock()
	defer githubConnectorMutex.RUnlock()
	return githubAuthCreator(name, spec)
}

// GithubAuthInitializer initializes a GitHub auth connector.
type GithubAuthInitializer func(types.GithubConnector) (types.GithubConnector, error)

var githubAuthInitializer GithubAuthInitializer

// RegisterGithubAuthInitializer registers a function to initialize GitHub auth connectors.
func RegisterGithubAuthInitializer(init GithubAuthInitializer) {
	githubConnectorMutex.Lock()
	defer githubConnectorMutex.Unlock()
	githubAuthInitializer = init
}

// InitGithubConnector initializes c and returns a [types.GithubConnector]
// ready for use. InitGithubConnector must be used to initialize any
// uninitialized [types.GithubConnector]s before they can be used.
func InitGithubConnector(c types.GithubConnector) (types.GithubConnector, error) {
	githubConnectorMutex.RLock()
	defer githubConnectorMutex.RUnlock()
	return githubAuthInitializer(c)
}

// GithubAuthConverter converts a GitHub auth connector so it can be
// sent over gRPC.
type GithubAuthConverter func(types.GithubConnector) (*types.GithubConnectorV3, error)

var githubAuthConverter GithubAuthConverter

// RegisterGithubAuthConverter registers a function to convert GitHub auth connectors.
func RegisterGithubAuthConverter(convert GithubAuthConverter) {
	githubConnectorMutex.Lock()
	defer githubConnectorMutex.Unlock()
	githubAuthConverter = convert
}

// ConvertGithubConnector converts a GitHub auth connector so it can be
// sent over gRPC.
func ConvertGithubConnector(c types.GithubConnector) (*types.GithubConnectorV3, error) {
	githubConnectorMutex.RLock()
	defer githubConnectorMutex.RUnlock()
	return githubAuthConverter(c)
}

func init() {
	RegisterGithubAuthCreator(types.NewGithubConnector)
	RegisterGithubAuthInitializer(func(c types.GithubConnector) (types.GithubConnector, error) {
		return c, nil
	})
	RegisterGithubAuthConverter(func(c types.GithubConnector) (*types.GithubConnectorV3, error) {
		connector, ok := c.(*types.GithubConnectorV3)
		if !ok {
			return nil, trace.BadParameter("unrecognized github connector version %T", c)
		}
		return connector, nil
	})
}

// UnmarshalGithubConnector unmarshals the GithubConnector resource from JSON.
func UnmarshalGithubConnector(bytes []byte, opts ...MarshalOption) (types.GithubConnector, error) {
	r, err := UnmarshalResource(types.KindGithubConnector, bytes, opts...)
	if err != nil {
		return nil, err
	}
	connector, ok := r.(types.GithubConnector)
	if !ok {
		return nil, trace.BadParameter("expected GithubConnector, got %T", r)
	}

	return connector, nil
}

// UnmarshalOSSGithubConnector unmarshals the open source variant of the GithubConnector resource from JSON.
func UnmarshalOSSGithubConnector(bytes []byte, opts ...MarshalOption) (types.GithubConnector, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	if err := json.Unmarshal(bytes, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V3:
		var c types.GithubConnectorV3
		if err := utils.FastUnmarshal(bytes, &c); err != nil {
			return nil, trace.Wrap(err)
		}
		if err := c.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			c.SetResourceID(cfg.ID)
		}
		if cfg.Revision != "" {
			c.SetRevision(cfg.Revision)
		}
		if !cfg.Expires.IsZero() {
			c.SetExpiry(cfg.Expires)
		}
		return &c, nil
	}
	return nil, trace.BadParameter(
		"GitHub connector resource version %q is not supported", h.Version)
}

// MarshalGithubConnector marshals a GithubConnector resource to JSON.
func MarshalGithubConnector(connector types.GithubConnector, opts ...MarshalOption) ([]byte, error) {
	return MarshalResource(connector, opts...)
}

// MarshalOSSGithubConnector marshals the open source variant of the GithubConnector resource to JSON.
func MarshalOSSGithubConnector(githubConnector types.GithubConnector, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch githubConnector := githubConnector.(type) {
	case *types.GithubConnectorV3:
		if err := githubConnector.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		// Only return an error if the endpoint url is set and the build is OSS
		// so that the enterprise marshaler can call this marshaler to produce
		// the final output without receiving an error.
		if modules.GetModules().BuildType() == modules.BuildOSS &&
			githubConnector.Spec.EndpointURL != "" {
			return nil, fmt.Errorf("GitHub endpoint URL is set: %w", ErrRequiresEnterprise)
		}
		return utils.FastMarshal(maybeResetProtoResourceID(cfg.PreserveResourceID, githubConnector))
	default:
		return nil, trace.BadParameter("unrecognized github connector version %T", githubConnector)
	}
}
