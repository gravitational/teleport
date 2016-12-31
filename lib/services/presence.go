/*
Copyright 2015 Gravitational, Inc.

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
	"time"
)

// Presence records and reports the presence of all components
// of the cluster - Nodes, Proxies and SSH nodes
type Presence interface {
	// GetNodes returns a list of registered servers
	GetNodes(namespace string) ([]Server, error)

	// UpsertNode registers node presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertNode(server Server, ttl time.Duration) error

	// GetAuthServers returns a list of registered servers
	GetAuthServers() ([]Server, error)

	// UpsertAuthServer registers auth server presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertAuthServer(server Server, ttl time.Duration) error

	// UpsertProxy registers proxy server presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertProxy(server Server, ttl time.Duration) error

	// GetProxies returns a list of registered proxies
	GetProxies() ([]Server, error)

	// UpsertReverseTunnel upserts reverse tunnel entry temporarily or permanently
	UpsertReverseTunnel(tunnel ReverseTunnel, ttl time.Duration) error

	// GetReverseTunnels returns a list of registered servers
	GetReverseTunnels() ([]ReverseTunnel, error)

	// DeleteReverseTunnel deletes reverse tunnel by it's domain name
	DeleteReverseTunnel(domainName string) error

	// GetNamespaces returns a list of namespaces
	GetNamespaces() ([]Namespace, error)

	// GetNamespace returns namespace by name
	GetNamespace(name string) (*Namespace, error)

	// UpsertNamespace upserts namespace
	UpsertNamespace(Namespace) error

	// DeleteNamespace deletes namespace by name
	DeleteNamespace(name string) error
}

// NewNamespace returns new namespace
func NewNamespace(name string) Namespace {
	return Namespace{
		Kind:    KindNamespace,
		Version: V2,
		Metadata: Metadata{
			Name: name,
		},
	}
}

// Site represents a cluster of teleport nodes who collectively trust the same
// certificate authority (CA) and have a common name.
//
// The CA is represented by an auth server (or multiple auth servers, if running
// in HA mode)
type Site struct {
	Name          string    `json:"name"`
	LastConnected time.Time `json:"lastconnected"`
	Status        string    `json:"status"`
}
