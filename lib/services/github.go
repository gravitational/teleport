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
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

var ErrRequiresEnterprise = trace.AccessDenied("this feature requires Teleport Enterprise")

// githubConnectorMutex is a mutex for the Github auth connector creator.
var githubConnectorMutex sync.RWMutex

// GithubAuthCreator creates a new Github connector.
type GithubAuthCreator func(string, types.GithubConnectorSpecV3) (types.GithubConnector, error)

// githubAuthCreator is the factory function that will create Github auth connectors.
var githubAuthCreator GithubAuthCreator

// RegisterGithubAuthCreator registers a function to create Github auth connectors.
func RegisterGithubAuthCreator(creator GithubAuthCreator) {
	githubConnectorMutex.Lock()
	defer githubConnectorMutex.Unlock()
	githubAuthCreator = creator
}

// NewGithubConnector creates a new Github auth connector.
func NewGithubConnector(name string, spec types.GithubConnectorSpecV3) (types.GithubConnector, error) {
	githubConnectorMutex.RLock()
	defer githubConnectorMutex.RUnlock()
	return githubAuthCreator(name, spec)
}

// GithubAuthInitializer creates a new Github connector.
type GithubAuthInitializer func(types.GithubConnector) (types.GithubConnector, error)

// githubAuthInitializer
var githubAuthInitializer GithubAuthInitializer

// RegisterGithubAuthCreator registers a function to create Github auth connectors.
func RegisterGithubAuthInitializer(init GithubAuthInitializer) {
	githubConnectorMutex.Lock()
	defer githubConnectorMutex.Unlock()
	githubAuthInitializer = init
}

// InitGithubConnector creates a new Github auth connector.
func InitGithubConnector(c types.GithubConnector) (types.GithubConnector, error) {
	githubConnectorMutex.RLock()
	defer githubConnectorMutex.RUnlock()
	return githubAuthInitializer(c)
}

// GithubAuthConverter
type GithubAuthConverter func(types.GithubConnector) (*types.GithubConnectorV3, error)

// githubAuthConverter
var githubAuthConverter GithubAuthConverter

// RegisterGithubAuthCreator registers a function to create Github auth connectors.
func RegisterGithubAuthConverter(convert GithubAuthConverter) {
	githubConnectorMutex.Lock()
	defer githubConnectorMutex.Unlock()
	githubAuthConverter = convert
}

// ConvertGithubConnector
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
func UnmarshalGithubConnector(bytes []byte) (types.GithubConnector, error) {
	r, err := UnmarshalResource(types.KindGithubConnector, bytes, nil)
	if err != nil {
		return nil, err
	}
	connector, ok := r.(types.GithubConnector)
	if !ok {
		return nil, trace.BadParameter("expected GithubConnector, got %T", r)
	}

	return connector, nil
}

func unmarshalGithubConnector(bytes []byte) (types.GithubConnector, error) {
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
		return &c, nil
	}
	return nil, trace.BadParameter(
		"Github connector resource version %q is not supported", h.Version)
}

// MarshalGithubConnector marshals the GithubConnector resource to JSON.
func MarshalGithubConnector(connector types.GithubConnector, opts ...MarshalOption) ([]byte, error) {
	return MarshalResource(connector, opts...)
}

func marshalGithubConnector(githubConnector types.GithubConnector, opts ...MarshalOption) ([]byte, error) {
	if err := githubConnector.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch githubConnector := githubConnector.(type) {
	case *types.GithubConnectorV3:
		if githubConnector.Spec.EndpointURL != "" {
			return nil, ErrRequiresEnterprise
		}

		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *githubConnector
			copy.SetResourceID(0)
			githubConnector = &copy
		}
		return utils.FastMarshal(githubConnector)
	default:
		return nil, trace.BadParameter("unrecognized github connector version %T", githubConnector)
	}
}
