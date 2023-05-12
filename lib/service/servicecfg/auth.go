// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package servicecfg

import (
	"github.com/coreos/go-oidc/oauth2"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/utils"
)

// AuthConfig is a configuration of the auth server
type AuthConfig struct {
	// Enabled turns auth role on or off for this process
	Enabled bool

	// EnableProxyProtocol enables proxy protocol support
	EnableProxyProtocol bool

	// ListenAddr is the listening address of the auth service
	ListenAddr utils.NetAddr

	// Authorities is a set of trusted certificate authorities
	// that will be added by this auth server on the first start
	Authorities []types.CertAuthority

	// BootstrapResources is a set of previously backed up resources
	// used to bootstrap backend state on the first start.
	BootstrapResources []types.Resource

	// ApplyOnStartupResources is a set of resources that should be applied
	// on each Teleport start.
	ApplyOnStartupResources []types.Resource

	// Roles is a set of roles to pre-provision for this cluster
	Roles []types.Role

	// ClusterName is a name that identifies this authority and all
	// host nodes in the cluster that will share this authority domain name
	// as a base name, e.g. if authority domain name is example.com,
	// all nodes in the cluster will have UUIDs in the form: <uuid>.example.com
	ClusterName types.ClusterName

	// StaticTokens are pre-defined host provisioning tokens supplied via config file for
	// environments where paranoid security is not needed
	StaticTokens types.StaticTokens

	// StorageConfig contains configuration settings for the storage backend.
	StorageConfig backend.Config

	Limiter limiter.Config

	// NoAudit, when set to true, disables session recording and event audit
	NoAudit bool

	// Preference defines the authentication preference (type and second factor) for
	// the auth server.
	Preference types.AuthPreference

	// AuditConfig stores cluster audit configuration.
	AuditConfig types.ClusterAuditConfig

	// NetworkingConfig stores cluster networking configuration.
	NetworkingConfig types.ClusterNetworkingConfig

	// SessionRecordingConfig stores session recording configuration.
	SessionRecordingConfig types.SessionRecordingConfig

	// LicenseFile is a full path to the license file
	LicenseFile string

	// PublicAddrs affects the SSH host principals and DNS names added to the SSH and TLS certs.
	PublicAddrs []utils.NetAddr

	// KeyStore configuration. Handles CA private keys which may be held in a HSM.
	KeyStore keystore.Config

	// LoadAllCAs sends the host CAs of all clusters to SSH clients logging in when enabled,
	// instead of just the host CA for the current cluster.
	LoadAllCAs bool

	// HostedPlugins configures the Enterprise hosted plugin runtime
	HostedPlugins HostedPluginsConfig

	// Clock is the clock instance auth uses. Typically you'd only want to set
	// this during testing.
	Clock clockwork.Clock

	// HTTPClientForAWSSTS overwrites the default HTTP client used for making
	// STS requests. Used in test.
	HTTPClientForAWSSTS utils.HTTPDoClient
}

// HostedPluginsConfig configures the hosted plugin runtime.
type HostedPluginsConfig struct {
	Enabled        bool
	OAuthProviders PluginOAuthProviders
}

// PluginOAuthProviders holds application credentials for each
// 3rd party API provider
type PluginOAuthProviders struct {
	Slack      *oauth2.ClientCredentials
	Discord    *oauth2.ClientCredentials
	Mattermost *oauth2.ClientCredentials
}
