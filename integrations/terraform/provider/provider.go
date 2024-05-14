/*
Copyright 2015-2022 Gravitational, Inc.

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

package provider

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/integrations/lib"
)

const (
	// minServerVersion is the minimal teleport version the plugin supports.
	minServerVersion = "15.0.0-0"
)

type RetryConfig struct {
	Base     time.Duration
	Cap      time.Duration
	MaxTries int
}

// Provider Teleport Provider
type Provider struct {
	configured  bool
	Client      *client.Client
	RetryConfig RetryConfig
}

// providerData provider schema struct
type providerData struct {
	// Addr Teleport address
	Addr types.String `tfsdk:"addr"`
	// CertPath path to TLS certificate file
	CertPath types.String `tfsdk:"cert_path"`
	// CertBase64 base64 encoded TLS certificate file
	CertBase64 types.String `tfsdk:"cert_base64"`
	// KeyPath path to TLS private key file
	KeyPath types.String `tfsdk:"key_path"`
	// KeyBase64 base64 encoded TLS private key
	KeyBase64 types.String `tfsdk:"key_base64"`
	// RootCAPath path to TLS root CA certificate file
	RootCaPath types.String `tfsdk:"root_ca_path"`
	// RootCaPath base64 encoded root CA certificate
	RootCaBase64 types.String `tfsdk:"root_ca_base64"`
	// ProfileName Teleport profile name
	ProfileName types.String `tfsdk:"profile_name"`
	// ProfileDir Teleport profile dir
	ProfileDir types.String `tfsdk:"profile_dir"`
	// IdentityFilePath path to identity file
	IdentityFilePath types.String `tfsdk:"identity_file_path"`
	// IdentityFile identity file content
	IdentityFile types.String `tfsdk:"identity_file"`
	// IdentityFile identity file content encoded in base64
	IdentityFileBase64 types.String `tfsdk:"identity_file_base64"`
	// RetryBaseDuration is used to setup the retry algorithm when the API returns 'not found'
	RetryBaseDuration types.String `tfsdk:"retry_base_duration"`
	// RetryCapDuration is used to setup the retry algorithm when the API returns 'not found'
	RetryCapDuration types.String `tfsdk:"retry_cap_duration"`
	// RetryMaxTries sets the max number of tries when retrying
	RetryMaxTries types.String `tfsdk:"retry_max_tries"`
	// DialTimeout sets timeout when trying to connect to the server.
	DialTimeoutDuration types.String `tfsdk:"dial_timeout_duration"`
}

// New returns an empty provider struct
func New() tfsdk.Provider {
	return &Provider{}
}

// GetSchema returns the Terraform provider schema
func (p *Provider) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"addr": {
				Type:        types.StringType,
				Optional:    true,
				Description: "host:port where Teleport Auth server is running.",
			},
			"cert_path": {
				Type:        types.StringType,
				Optional:    true,
				Description: "Path to Teleport auth certificate file.",
			},
			"cert_base64": {
				Type:        types.StringType,
				Optional:    true,
				Description: "Base64 encoded TLS auth certificate.",
			},
			"key_path": {
				Type:        types.StringType,
				Optional:    true,
				Description: "Path to Teleport auth key file.",
			},
			"key_base64": {
				Type:        types.StringType,
				Sensitive:   true,
				Optional:    true,
				Description: "Base64 encoded TLS auth key.",
			},
			"root_ca_path": {
				Type:        types.StringType,
				Optional:    true,
				Description: "Path to Teleport Root CA.",
			},
			"root_ca_base64": {
				Type:        types.StringType,
				Optional:    true,
				Description: "Base64 encoded Root CA.",
			},
			"profile_name": {
				Type:        types.StringType,
				Optional:    true,
				Description: "Teleport profile name.",
			},
			"profile_dir": {
				Type:        types.StringType,
				Optional:    true,
				Description: "Teleport profile path.",
			},
			"identity_file_path": {
				Type:        types.StringType,
				Optional:    true,
				Description: "Teleport identity file path.",
			},
			"identity_file": {
				Type:        types.StringType,
				Sensitive:   true,
				Optional:    true,
				Description: "Teleport identity file content.",
			},
			"identity_file_base64": {
				Type:        types.StringType,
				Sensitive:   true,
				Optional:    true,
				Description: "Teleport identity file content base64 encoded.",
			},
			"retry_base_duration": {
				Type:        types.StringType,
				Sensitive:   false,
				Optional:    true,
				Description: "Retry algorithm when the API returns 'not found': base duration between retries (https://pkg.go.dev/time#ParseDuration).",
			},
			"retry_cap_duration": {
				Type:        types.StringType,
				Sensitive:   false,
				Optional:    true,
				Description: "Retry algorithm when the API returns 'not found': max duration between retries (https://pkg.go.dev/time#ParseDuration).",
			},
			"retry_max_tries": {
				Type:        types.StringType,
				Sensitive:   false,
				Optional:    true,
				Description: "Retry algorithm when the API returns 'not found': max tries.",
			},
			"dial_timeout_duration": {
				Type:        types.StringType,
				Sensitive:   false,
				Optional:    true,
				Description: "DialTimeout sets timeout when trying to connect to the server.",
			},
		},
	}, nil
}

// IsConfigured checks if provider is configured, adds diagnostics if not
func (p *Provider) IsConfigured(diags diag.Diagnostics) bool {
	if !p.configured {
		diags.AddError(
			"Provider not configured",
			"The provider hasn't been configured before apply, likely because it depends on an unknown value from another resource. This leads to weird stuff happening, so we'd prefer if you didn't do that. Thanks!",
		)
	}

	return p.configured
}

// Configure configures the Teleport client
func (p *Provider) Configure(ctx context.Context, req tfsdk.ConfigureProviderRequest, resp *tfsdk.ConfigureProviderResponse) {
	var creds []client.Credentials

	p.configureLog()

	var config providerData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	addr := p.stringFromConfigOrEnv(config.Addr, "TF_TELEPORT_ADDR", "")
	certPath := p.stringFromConfigOrEnv(config.CertPath, "TF_TELEPORT_CERT", "")
	certBase64 := p.stringFromConfigOrEnv(config.CertBase64, "TF_TELEPORT_CERT_BASE64", "")
	keyPath := p.stringFromConfigOrEnv(config.KeyPath, "TF_TELEPORT_KEY", "")
	keyBase64 := p.stringFromConfigOrEnv(config.KeyBase64, "TF_TELEPORT_KEY_BASE64", "")
	caPath := p.stringFromConfigOrEnv(config.RootCaPath, "TF_TELEPORT_ROOT_CA", "")
	caBase64 := p.stringFromConfigOrEnv(config.RootCaBase64, "TF_TELEPORT_CA_BASE64", "")
	profileName := p.stringFromConfigOrEnv(config.ProfileName, "TF_TELEPORT_PROFILE_NAME", "")
	profileDir := p.stringFromConfigOrEnv(config.ProfileDir, "TF_TELEPORT_PROFILE_PATH", "")
	identityFilePath := p.stringFromConfigOrEnv(config.IdentityFilePath, "TF_TELEPORT_IDENTITY_FILE_PATH", "")
	identityFile := p.stringFromConfigOrEnv(config.IdentityFile, "TF_TELEPORT_IDENTITY_FILE", "")
	identityFileBase64 := p.stringFromConfigOrEnv(config.IdentityFileBase64, "TF_TELEPORT_IDENTITY_FILE_BASE64", "")
	retryBaseDurationStr := p.stringFromConfigOrEnv(config.RetryBaseDuration, "TF_TELEPORT_RETRY_BASE_DURATION", "1s")
	retryCapDurationStr := p.stringFromConfigOrEnv(config.RetryCapDuration, "TF_TELEPORT_RETRY_CAP_DURATION", "5s")
	maxTriesStr := p.stringFromConfigOrEnv(config.RetryMaxTries, "TF_TELEPORT_RETRY_MAX_TRIES", "10")
	dialTimeoutDurationStr := p.stringFromConfigOrEnv(config.DialTimeoutDuration, "TF_TELEPORT_DIAL_TIMEOUT_DURATION", "30s")

	if !p.validateAddr(addr, resp) {
		return
	}

	log.WithFields(log.Fields{"addr": addr}).Debug("Using Teleport address")

	if certPath != "" && keyPath != "" {
		l := log.WithField("cert_path", certPath).WithField("key_path", keyPath).WithField("root_ca_path", caPath)
		l.Debug("Using auth with certificate, private key and (optionally) CA read from files")

		cred, ok := p.getCredentialsFromKeyPair(certPath, keyPath, caPath, resp)
		if !ok {
			return
		}
		creds = append(creds, cred)
	}

	if certBase64 != "" && keyBase64 != "" {
		log.Debug("Using auth with certificate, private key and (optionally) CA read from base64 encoded vars")
		cred, ok := p.getCredentialsFromBase64(certBase64, keyBase64, caBase64, resp)
		if !ok {
			return
		}
		creds = append(creds, cred)
	}

	if identityFilePath != "" {
		log.WithField("identity_file_path", identityFilePath).Debug("Using auth with identity file")

		if !p.fileExists(identityFilePath) {
			resp.Diagnostics.AddError(
				"Identity file not found",
				fmt.Sprintf(
					"File %v not found! Use `tctl auth sign --user=example@example.com --format=file --out=%v` to generate identity file",
					identityFilePath,
					identityFilePath,
				),
			)
			return
		}

		creds = append(creds, client.LoadIdentityFile(identityFilePath))
	}

	if identityFile != "" {
		log.Debug("Using auth from identity file provided with environment variable TF_TELEPORT_IDENTITY_FILE")
		creds = append(creds, client.LoadIdentityFileFromString(identityFile))
	}

	if identityFileBase64 != "" {
		log.Debug("Using auth from base64 encoded identity file provided with environment variable TF_TELEPORT_IDENTITY_FILE_BASE64")
		decoded, err := base64.StdEncoding.DecodeString(identityFileBase64)
		if err != nil {
			resp.Diagnostics.AddError(
				"Failed to decode Identity file using base 64",
				fmt.Sprintf("Error when trying to decode: %v", err),
			)
			return
		}

		creds = append(creds, client.LoadIdentityFileFromString(string(decoded)))
	}

	if profileDir != "" || len(creds) == 0 {
		log.WithFields(log.Fields{
			"dir":  profileDir,
			"name": profileName,
		}).Debug("Using profile as the default auth method")
		creds = append(creds, client.LoadProfile(profileDir, profileName))
	}

	dialTimeoutDuration, err := time.ParseDuration(dialTimeoutDurationStr)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to parse Dial Timeout Duration Cap Duration",
			fmt.Sprintf("Please check if dial_timeout_duration (or TF_TELEPORT_DIAL_TIMEOUT_DURATION) is set correctly. Error: %s", err),
		)
		return
	}

	client, err := client.New(ctx, client.Config{
		Addrs:       []string{addr},
		Credentials: creds,
		DialTimeout: dialTimeoutDuration,
		DialOpts: []grpc.DialOption{
			grpc.WithReturnConnectionError(),
			grpc.WithDefaultCallOptions(
				grpc.WaitForReady(true),
			),
		},
	})

	if err != nil {
		log.WithError(err).Debug("Error connecting to Teleport!")
		resp.Diagnostics.AddError("Error connecting to Teleport!", err.Error())
		return
	}

	if !p.checkTeleportVersion(ctx, client, resp) {
		return
	}

	retryBaseDuration, err := time.ParseDuration(retryBaseDurationStr)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to parse Retry Base Duration",
			fmt.Sprintf("Please check if retry_cap_duration (or TF_TELEPORT_RETRY_BASE_DURATION) is set correctly. Error: %s", err),
		)
		return
	}

	retryCapDuration, err := time.ParseDuration(retryCapDurationStr)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to parse Retry Cap Duration",
			fmt.Sprintf("Please check if retry_cap_duration (or TF_TELEPORT_RETRY_CAP_DURATION) is set correctly. Error: %s", err),
		)
		return
	}

	maxTries, err := strconv.ParseUint(maxTriesStr, 10, 32)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to parse Retry Max Tries",
			fmt.Sprintf("Please check if retry_max_tries (or TF_TELEPORT_RETRY_MAX_TRIES) is set correctly. Error: %s", err),
		)
		return
	}

	p.RetryConfig = RetryConfig{
		Base:     retryBaseDuration,
		Cap:      retryCapDuration,
		MaxTries: int(maxTries),
	}
	p.Client = client
	p.configured = true
}

// checkTeleportVersion ensures that Teleport version is at least minServerVersion
func (p *Provider) checkTeleportVersion(ctx context.Context, client *client.Client, resp *tfsdk.ConfigureProviderResponse) bool {
	log.Debug("Checking Teleport server version")
	pong, err := client.Ping(ctx)
	if err != nil {
		if trace.IsNotImplemented(err) {
			resp.Diagnostics.AddError(
				"Teleport version is too old!",
				fmt.Sprintf("Server version must be at least %s", minServerVersion),
			)
			return false
		}
		log.WithError(err).Debug("Teleport version check error!")
		resp.Diagnostics.AddError("Unable to get Teleport server version!", "Unable to get Teleport server version!")
		return false
	}
	err = lib.AssertServerVersion(pong, minServerVersion)
	if err != nil {
		log.WithError(err).Debug("Teleport version check error!")
		resp.Diagnostics.AddError("Teleport version check error!", err.Error())
		return false
	}
	return true
}

// stringFromConfigOrEnv returns value from config or from env var if config value is empty, default otherwise
func (p *Provider) stringFromConfigOrEnv(value types.String, env string, def string) string {
	if value.Unknown || value.Null {
		value := os.Getenv(env)
		if value != "" {
			return value
		}
	}

	configValue := strings.TrimSpace(value.Value)

	if configValue == "" {
		return def
	}

	return configValue
}

// validateAddr validates passed addr
func (p *Provider) validateAddr(addr string, resp *tfsdk.ConfigureProviderResponse) bool {
	if addr == "" {
		resp.Diagnostics.AddError(
			"Teleport address is empty",
			"Please, specify either TF_TELEPORT_ADDR or addr in provider configuration",
		)
		return false
	}

	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		log.WithField("addr", addr).WithError(err).Debug("Teleport addr format error!")
		resp.Diagnostics.AddError(
			"Invalid Teleport addr format",
			"Teleport addr must be specified as host:port",
		)
		return false
	}
	return true
}

// getCredentialsFromBase64 returns client.Credentials built from base64 encoded keys
func (p *Provider) getCredentialsFromBase64(certBase64, keyBase64, caBase64 string, resp *tfsdk.ConfigureProviderResponse) (client.Credentials, bool) {
	cert, err := base64.StdEncoding.DecodeString(certBase64)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to base64 decode cert",
			fmt.Sprintf("Please check if cert_base64 (or TF_TELEPORT_CERT_BASE64) is set correctly. Error: %s", err),
		)
		return nil, false
	}
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to base64 decode key",
			fmt.Sprintf("Please check if key_base64 (or TF_TELEPORT_KEY_BASE64) is set correctly. Error: %s", err),
		)
		return nil, false
	}
	rootCa, err := base64.StdEncoding.DecodeString(caBase64)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to base64 decode root ca",
			fmt.Sprintf("Please check if root_ca_base64 (or TF_TELEPORT_CA_BASE64) is set correctly. Error: %s", err),
		)
		return nil, false
	}
	tlsConfig, err := createTLSConfig(cert, key, rootCa)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to create TLS config",
			fmt.Sprintf("Error: %s", err),
		)
		return nil, false
	}
	return client.LoadTLS(tlsConfig), true
}

// getCredentialsFromKeyPair returns client.Credentials built from path to key files
func (p *Provider) getCredentialsFromKeyPair(certPath string, keyPath string, caPath string, resp *tfsdk.ConfigureProviderResponse) (client.Credentials, bool) {
	if !p.fileExists(certPath) {
		resp.Diagnostics.AddError(
			"Certificate file not found",
			fmt.Sprintf("File %v not found! Use 'tctl auth sign --user=example@example.com --format=tls --out=%v' to generate keys",
				certPath,
				filepath.Dir(certPath),
			),
		)
		return nil, false
	}

	if !p.fileExists(keyPath) {
		resp.Diagnostics.AddError(
			"Private key file not found",
			fmt.Sprintf("File %v not found! Use 'tctl auth sign --user=example@example.com --format=tls --out=%v' to generate keys",
				keyPath,
				filepath.Dir(keyPath),
			),
		)
		return nil, false
	}

	if !p.fileExists(caPath) {
		resp.Diagnostics.AddError(
			"Root CA certificate file not found",
			fmt.Sprintf("File %v not found! Use 'tctl auth sign --user=example@example.com --format=tls --out=%v' to generate keys",
				caPath,
				filepath.Dir(caPath),
			),
		)
		return nil, false
	}

	return client.LoadKeyPair(certPath, keyPath, caPath), true
}

// fileExists returns true if file exists
func (p *Provider) fileExists(path string) bool {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		return false
	}
	return true
}

// configureLog configures logging
func (p *Provider) configureLog() {
	// Get Terraform log level
	level, err := log.ParseLevel(os.Getenv("TF_LOG"))
	if err != nil {
		log.SetLevel(log.ErrorLevel)
	} else {
		log.SetLevel(level)
	}

	log.SetFormatter(&log.TextFormatter{})

	// Show GRPC debug logs only if TF_LOG=DEBUG
	if log.GetLevel() >= log.DebugLevel {
		l := grpclog.NewLoggerV2(log.StandardLogger().Out, log.StandardLogger().Out, log.StandardLogger().Out)
		grpclog.SetLoggerV2(l)
	}
}

// createTLSConfig returns tls.Config build from keys
func createTLSConfig(cert, key, rootCa []byte) (*tls.Config, error) {
	keyPair, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(rootCa)

	return &tls.Config{
		Certificates: []tls.Certificate{keyPair},
		RootCAs:      caCertPool,
	}, nil
}

// GetResources returns the map of provider resources
func (p *Provider) GetResources(_ context.Context) (map[string]tfsdk.ResourceType, diag.Diagnostics) {
	return map[string]tfsdk.ResourceType{
		"teleport_app":                        resourceTeleportAppType{},
		"teleport_auth_preference":            resourceTeleportAuthPreferenceType{},
		"teleport_cluster_maintenance_config": resourceTeleportClusterMaintenanceConfigType{},
		"teleport_cluster_networking_config":  resourceTeleportClusterNetworkingConfigType{},
		"teleport_database":                   resourceTeleportDatabaseType{},
		"teleport_github_connector":           resourceTeleportGithubConnectorType{},
		"teleport_provision_token":            resourceTeleportProvisionTokenType{},
		"teleport_oidc_connector":             resourceTeleportOIDCConnectorType{},
		"teleport_role":                       resourceTeleportRoleType{},
		"teleport_saml_connector":             resourceTeleportSAMLConnectorType{},
		"teleport_session_recording_config":   resourceTeleportSessionRecordingConfigType{},
		"teleport_trusted_cluster":            resourceTeleportTrustedClusterType{},
		"teleport_user":                       resourceTeleportUserType{},
		"teleport_bot":                        resourceTeleportBotType{},
		"teleport_login_rule":                 resourceTeleportLoginRuleType{},
		"teleport_trusted_device":             resourceTeleportDeviceV1Type{},
		"teleport_okta_import_rule":           resourceTeleportOktaImportRuleType{},
		"teleport_access_list":                resourceTeleportAccessListType{},
		"teleport_server":                     resourceTeleportServerType{},
	}, nil
}

// GetDataSources returns the map of provider data sources
func (p *Provider) GetDataSources(_ context.Context) (map[string]tfsdk.DataSourceType, diag.Diagnostics) {
	return map[string]tfsdk.DataSourceType{
		"teleport_app":                        dataSourceTeleportAppType{},
		"teleport_auth_preference":            dataSourceTeleportAuthPreferenceType{},
		"teleport_cluster_maintenance_config": dataSourceTeleportClusterMaintenanceConfigType{},
		"teleport_cluster_networking_config":  dataSourceTeleportClusterNetworkingConfigType{},
		"teleport_database":                   dataSourceTeleportDatabaseType{},
		"teleport_github_connector":           dataSourceTeleportGithubConnectorType{},
		"teleport_provision_token":            dataSourceTeleportProvisionTokenType{},
		"teleport_oidc_connector":             dataSourceTeleportOIDCConnectorType{},
		"teleport_role":                       dataSourceTeleportRoleType{},
		"teleport_saml_connector":             dataSourceTeleportSAMLConnectorType{},
		"teleport_session_recording_config":   dataSourceTeleportSessionRecordingConfigType{},
		"teleport_trusted_cluster":            dataSourceTeleportTrustedClusterType{},
		"teleport_user":                       dataSourceTeleportUserType{},
		"teleport_login_rule":                 dataSourceTeleportLoginRuleType{},
		"teleport_trusted_device":             dataSourceTeleportDeviceV1Type{},
		"teleport_okta_import_rule":           dataSourceTeleportOktaImportRuleType{},
		"teleport_access_list":                dataSourceTeleportAccessListType{},
	}, nil
}
