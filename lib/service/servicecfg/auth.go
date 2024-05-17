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
	"github.com/dustin/go-humanize"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/utils"
)

// AuthConfig is a configuration of the auth server
type AuthConfig struct {
	// Enabled turns auth role on or off for this process
	Enabled bool

	// PROXYProtocolMode controls behavior related to unsigned PROXY protocol headers.
	PROXYProtocolMode multiplexer.PROXYProtocolMode

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
	KeyStore KeystoreConfig

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

	// AssistAPIKey is the OpenAI API key.
	// TODO: This key will be moved to a plugin once support for plugins is implemented.
	AssistAPIKey string

	// AccessMonitoring configures access monitoring.
	AccessMonitoring *AccessMonitoringOptions
}

// AccessMonitoringOptions configures access monitoring.
type AccessMonitoringOptions struct {
	// EnabledString is the string representation of the Enabled field.
	EnabledString string `yaml:"enabled"`
	// Enabled is true if access monitoring is enabled.
	Enabled bool `yaml:"-"`

	// RoleARN is the ARN of the IAM role to assume when accessing Athena.
	RoleARN string `yaml:"role_arn,omitempty"`
	// RoleTags are the tags to use when assuming the IAM role.
	RoleTags map[string]string `yaml:"role_tags,omitempty"`

	// DataLimitString is the string representation of the DataLimit field.
	DataLimitString string `yaml:"data_limit,omitempty"`
	// DataLimit is the maximum amount of data that can be returned by a query.
	DataLimit uint64 `yaml:"-"`

	// Database is the name of the database to use.
	Database string `yaml:"database,omitempty"`
	// Table is the name of the table to use.
	Table string `yaml:"table,omitempty"`
	// Workgroup is the name of the Athena workgroup to use.
	Workgroup string `yaml:"workgroup,omitempty"`
	// QueryResults is the S3 bucket to use for query results.
	QueryResults string `yaml:"query_results,omitempty"`
	// ReportResults is the S3 bucket to use for report results.
	ReportResults string `yaml:"report_results,omitempty"`
}

// IsAccessMonitoringEnabled returns true if access monitoring is enabled.
func (a *AuthConfig) IsAccessMonitoringEnabled() bool {
	return a.AccessMonitoring != nil && a.AccessMonitoring.Enabled
}

// CheckAndSetDefaults checks and sets default values for any missing fields.
func (a *AccessMonitoringOptions) CheckAndSetDefaults() error {
	var err error
	if a.DataLimitString != "" {
		if a.DataLimit, err = humanize.ParseBytes(a.DataLimitString); err != nil {
			return trace.Wrap(err)
		}
	}
	if a.EnabledString != "" {
		if a.Enabled, err = apiutils.ParseBool(a.EnabledString); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// HostedPluginsConfig configures the hosted plugin runtime.
type HostedPluginsConfig struct {
	Enabled        bool
	OAuthProviders PluginOAuthProviders
}

// PluginOAuthProviders holds application credentials for each
// 3rd party API provider
type PluginOAuthProviders struct {
	Slack *oauth2.ClientCredentials
}

// KeystoreConfig configures the auth keystore.
type KeystoreConfig struct {
	// PKCS11 holds configuration parameters specific to PKCS#11 keystores.
	PKCS11 PKCS11Config
	// GCPKMS holds configuration parameters specific to GCP KMS keystores.
	GCPKMS GCPKMSConfig
	// AWSKMS holds configuration parameter specific to AWS KMS keystores.
	AWSKMS AWSKMSConfig
}

type PKCS11Config struct {
	Path       string
	SlotNumber *int
	TokenLabel string
	Pin        string
	HostUUID   string
}

type GCPKMSConfig struct {
	KeyRing         string
	ProtectionLevel string
	HostUUID        string
}

type AWSKMSConfig struct {
	Cluster    string
	AWSAccount string
	AWSRegion  string
}
