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

package servicecfg

import (
	"slices"

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
	SlackCredentials *OAuthClientCredentials
}

// OAuthClientCredentials stores the client_id and client_secret
// of an OAuth application.
type OAuthClientCredentials struct {
	ClientID     string
	ClientSecret string
}

// KeystoreConfig configures the auth keystore.
type KeystoreConfig struct {
	// PKCS11 holds configuration parameters specific to PKCS#11 keystores.
	PKCS11 PKCS11Config
	// GCPKMS holds configuration parameters specific to GCP KMS keystores.
	GCPKMS GCPKMSConfig
	// AWSKMS holds configuration parameter specific to AWS KMS keystores.
	AWSKMS *AWSKMSConfig
}

// CheckAndSetDefaults checks that required parameters of the config are
// properly set and sets defaults.
func (cfg *KeystoreConfig) CheckAndSetDefaults() error {
	count := 0
	if cfg.PKCS11 != (PKCS11Config{}) {
		if err := cfg.PKCS11.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err, "validating pkcs11 config")
		}
		count++
	}
	if cfg.GCPKMS != (GCPKMSConfig{}) {
		if err := cfg.GCPKMS.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err, "validating gcp_kms config")
		}
		count++
	}
	if cfg.AWSKMS != nil {
		if err := cfg.AWSKMS.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err, "validating aws_kms config")
		}
		count++
	}
	if count > 1 {
		return trace.BadParameter("must configure at most one of pkcs11, gcp_kms, or aws_kms")
	}
	return nil
}

// PKCS11Config holds static configuration options for a PKCS#11 HSM.
type PKCS11Config struct {
	// Path is the path to the PKCS11 module.
	Path string
	// SlotNumber is the PKCS11 slot to use.
	SlotNumber *int
	// TokenLabel is the label of the PKCS11 token to use.
	TokenLabel string
	// PIN is the PKCS11 PIN for the given token.
	PIN string
	// MaxSessions is the upper limit of sessions allowed by the HSM.
	MaxSessions int
}

// CheckAndSetDefaults checks that required parameters of the config are
// properly set and sets defaults.
func (cfg *PKCS11Config) CheckAndSetDefaults() error {
	if cfg.Path == "" {
		return trace.BadParameter("must provide Path")
	}
	if cfg.SlotNumber == nil && cfg.TokenLabel == "" {
		return trace.BadParameter("must provide one of SlotNumber or TokenLabel")
	}

	switch {
	case cfg.MaxSessions < 0:
		return trace.BadParameter("the value of PKCS11 MaxSessions must not be negative")
	case cfg.MaxSessions == 1:
		return trace.BadParameter("the minimum value for PKCS11 MaxSessions is 2")
	case cfg.MaxSessions == 0:
	// A value of zero is acceptable and indicates to the pkcs11 library to use the default value.
	default:
	}

	return nil
}

// GCPKMSConfig holds static configuration options for GCP KMS.
type GCPKMSConfig struct {
	// KeyRing is the fully qualified name of the GCP KMS keyring.
	KeyRing string
	// ProtectionLevel specifies how cryptographic operations are performed.
	// For more information, see https://cloud.google.com/kms/docs/algorithms#protection_levels
	// Supported options are "HSM" and "SOFTWARE".
	ProtectionLevel string
}

const (
	// GCPKMSProtectionLevelHSM represents the HSM protection level in GCP KMS.
	GCPKMSProtectionLevelHSM = "HSM"
	// GCPKMSProtectionLevelSoftware represents the SOFTWARE protection level in GCP KMS.
	GCPKMSProtectionLevelSoftware = "SOFTWARE"
)

// CheckAndSetDefaults checks that required parameters of the config are
// properly set and sets defaults.
func (cfg *GCPKMSConfig) CheckAndSetDefaults() error {
	if cfg.KeyRing == "" {
		return trace.BadParameter("must provide a valid KeyRing")
	}
	if !slices.Contains([]string{GCPKMSProtectionLevelHSM, GCPKMSProtectionLevelSoftware}, cfg.ProtectionLevel) {
		return trace.BadParameter("unsupported ProtectionLevel %s", cfg.ProtectionLevel)
	}
	return nil
}

// AWSKMSConfig holds static configuration options for AWS KMS.
type AWSKMSConfig struct {
	// AWSAccount is the AWS account ID where the keys will reside.
	AWSAccount string
	// AWSRegion is the AWS region where the keys will reside.
	AWSRegion string
	// MultiRegion contains configuration for multi-region AWS KMS.
	MultiRegion struct {
		// Enabled configures new keys to be multi-region.
		Enabled bool
	}
	// Tags are key/value pairs used as AWS resource tags. The 'TeleportCluster'
	// tag is added automatically if not specified in the set of tags. Changing tags
	// after Teleport has already created KMS keys may require manually updating
	// the tags of existing keys.
	Tags map[string]string
}

// CheckAndSetDefaults checks that required parameters of the config are
// properly set and sets defaults.
func (c *AWSKMSConfig) CheckAndSetDefaults() error {
	if c.AWSAccount == "" {
		return trace.BadParameter("AWS account is required")
	}
	if c.AWSRegion == "" {
		return trace.BadParameter("AWS region is required")
	}
	return nil
}
