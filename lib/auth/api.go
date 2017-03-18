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

package auth

import (
	"github.com/gravitational/teleport/lib/services"
)

// AccessPoint is an API interface implemented by a certificate authority (CA)
type AccessPoint interface {
	// GetReverseTunnels returns  a list of reverse tunnels
	GetReverseTunnels() ([]services.ReverseTunnel, error)

	// GetDomainName returns domain name AKA ("cluster name") of the auth
	// server / certificate authority (CA)
	GetDomainName() (string, error)

	// GetNamespaces returns a list of namespaces
	GetNamespaces() ([]services.Namespace, error)

	// GetNamespace returns namespace by name
	GetNamespace(name string) (*services.Namespace, error)

	// GetServers returns a list of registered servers
	GetNodes(namespace string) ([]services.Server, error)

	// UpsertServer registers server presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertNode(s services.Server) error

	// UpsertProxy registers server presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertProxy(s services.Server) error

	// GetProxies returns a list of proxy servers registered in the cluster
	GetProxies() ([]services.Server, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(caType services.CertAuthType, loadKeys bool) ([]services.CertAuthority, error)

	// GetUsers returns a list of local users registered with this domain
	GetUsers() ([]services.User, error)

	// GetRole returns role by name
	GetRole(name string) (services.Role, error)

	// GetRoles returns a list of roles
	GetRoles() ([]services.Role, error)
}
