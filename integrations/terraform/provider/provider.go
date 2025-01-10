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
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const (
	// minServerVersion is the minimal teleport version the plugin supports.
	minServerVersion = "15.0.0-0"
)

const (
	// attributeTerraformAddress is the attribute configuring the Teleport address the Terraform provider connects to.
	attributeTerraformAddress = "addr"
	// attributeTerraformCertificates is the attribute configuring the path the Terraform provider loads its
	// client certificates from. This only works for direct auth joining.
	attributeTerraformCertificates = "cert_path"
	// attributeTerraformCertificatesBase64 is the attribute configuring the client certificates used by the
	// Terraform provider. This only works for direct auth joining.
	attributeTerraformCertificatesBase64 = "cert_base64"
	// attributeTerraformKey is the attribute configuring the path the Terraform provider loads its
	// client key from. This only works for direct auth joining.
	attributeTerraformKey = "key_path"
	// attributeTerraformKeyBase64 is the attribute configuring the client key used by the
	// Terraform provider. This only works for direct auth joining.
	attributeTerraformKeyBase64 = "key_base64"
	// attributeTerraformRootCertificates is the attribute configuring the path the Terraform provider loads its
	// trusted CA certificates from. This only works for direct auth joining.
	attributeTerraformRootCertificates = "root_ca_path"
	// attributeTerraformRootCertificatesBase64 is the attribute configuring the CA certificates trusted by the
	// Terraform provider. This only works for direct auth joining.
	attributeTerraformRootCertificatesBase64 = "root_ca_base64"
	// attributeTerraformProfileName is the attribute containing name of the profile used by the Terraform provider.
	attributeTerraformProfileName = "profile_name"
	// attributeTerraformProfilePath is the attribute containing the profile directory used by the Terraform provider.
	attributeTerraformProfilePath = "profile_dir"
	// attributeTerraformIdentityFilePath is the attribute containing the path to the identity file used by the provider.
	attributeTerraformIdentityFilePath = "identity_file_path"
	// attributeTerraformIdentityFile is the attribute containing the identity file used by the Terraform provider.
	attributeTerraformIdentityFile = "identity_file"
	// attributeTerraformIdentityFileBase64 is the attribute containing the base64-encoded identity file used by the Terraform provider.
	attributeTerraformIdentityFileBase64 = "identity_file_base64"
	// attributeTerraformRetryBaseDuration is the attribute configuring the base duration between two Terraform provider retries.
	attributeTerraformRetryBaseDuration = "retry_base_duration"
	// attributeTerraformRetryCapDuration is the attribute configuring the maximum duration between two Terraform provider retries.
	attributeTerraformRetryCapDuration = "retry_cap_duration"
	// attributeTerraformRetryMaxTries is the attribute configuring the maximum number of Terraform provider retries.
	attributeTerraformRetryMaxTries = "retry_max_tries"
	// attributeTerraformDialTimeoutDuration is the attribute configuring the Terraform provider dial timeout.
	attributeTerraformDialTimeoutDuration = "dial_timeout_duration"
	// attributeTerraformJoinMethod is the attribute configuring the Terraform provider native MachineID join method.
	attributeTerraformJoinMethod = "join_method"
	// attributeTerraformJoinToken is the attribute configuring the Terraform provider native MachineID join token.
	attributeTerraformJoinToken = "join_token"
	// attributeTerraformJoinAudienceTag is the attribute configuring the audience tag when using the `terraform` join
	// method.
	attributeTerraformJoinAudienceTag = "audience_tag"
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
	cancel      context.CancelFunc
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
	// JoinMethod is the MachineID join method.
	JoinMethod types.String `tfsdk:"join_method"`
	// JoinMethod is the MachineID join token.
	JoinToken types.String `tfsdk:"join_token"`
	// AudienceTag is the audience  tag for the `terraform` join method
	AudienceTag types.String `tfsdk:"audience_tag"`
}

// New returns an empty provider struct
func New() tfsdk.Provider {
	return &Provider{}
}

// GetSchema returns the Terraform provider schema
func (p *Provider) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			attributeTerraformAddress: {
				Type:        types.StringType,
				Optional:    true,
				Description: fmt.Sprintf("host:port of the Teleport address. This can be the Teleport Proxy Service address (port 443 or 4080) or the Teleport Auth Service address (port 3025). This can also be set with the environment variable `%s`.", constants.EnvVarTerraformAddress),
			},
			attributeTerraformCertificates: {
				Type:        types.StringType,
				Optional:    true,
				Description: fmt.Sprintf("Path to Teleport auth certificate file. This can also be set with the environment variable `%s`.", constants.EnvVarTerraformCertificates),
			},
			attributeTerraformCertificatesBase64: {
				Type:        types.StringType,
				Optional:    true,
				Description: fmt.Sprintf("Base64 encoded TLS auth certificate. This can also be set with the environment variable `%s`.", constants.EnvVarTerraformCertificatesBase64),
			},
			attributeTerraformKey: {
				Type:        types.StringType,
				Optional:    true,
				Description: fmt.Sprintf("Path to Teleport auth key file. This can also be set with the environment variable `%s`.", constants.EnvVarTerraformKey),
			},
			attributeTerraformKeyBase64: {
				Type:        types.StringType,
				Sensitive:   true,
				Optional:    true,
				Description: fmt.Sprintf("Base64 encoded TLS auth key. This can also be set with the environment variable `%s`.", constants.EnvVarTerraformKeyBase64),
			},
			attributeTerraformRootCertificates: {
				Type:        types.StringType,
				Optional:    true,
				Description: fmt.Sprintf("Path to Teleport Root CA. This can also be set with the environment variable `%s`.", constants.EnvVarTerraformRootCertificates),
			},
			attributeTerraformRootCertificatesBase64: {
				Type:        types.StringType,
				Optional:    true,
				Description: fmt.Sprintf("Base64 encoded Root CA. This can also be set with the environment variable `%s`.", constants.EnvVarTerraformRootCertificatesBase64),
			},
			attributeTerraformProfileName: {
				Type:        types.StringType,
				Optional:    true,
				Description: fmt.Sprintf("Teleport profile name. This can also be set with the environment variable `%s`.", constants.EnvVarTerraformProfileName),
			},
			attributeTerraformProfilePath: {
				Type:        types.StringType,
				Optional:    true,
				Description: fmt.Sprintf("Teleport profile path. This can also be set with the environment variable `%s`.", constants.EnvVarTerraformProfilePath),
			},
			attributeTerraformIdentityFilePath: {
				Type:        types.StringType,
				Optional:    true,
				Description: fmt.Sprintf("Teleport identity file path. This can also be set with the environment variable `%s`.", constants.EnvVarTerraformIdentityFilePath),
			},
			attributeTerraformIdentityFile: {
				Type:        types.StringType,
				Sensitive:   true,
				Optional:    true,
				Description: fmt.Sprintf("Teleport identity file content. This can also be set with the environment variable `%s`.", constants.EnvVarTerraformIdentityFile),
			},
			attributeTerraformIdentityFileBase64: {
				Type:        types.StringType,
				Sensitive:   true,
				Optional:    true,
				Description: fmt.Sprintf("Teleport identity file content base64 encoded. This can also be set with the environment variable `%s`.", constants.EnvVarTerraformIdentityFileBase64),
			},
			attributeTerraformRetryBaseDuration: {
				Type:        types.StringType,
				Sensitive:   false,
				Optional:    true,
				Description: fmt.Sprintf("Retry algorithm when the API returns 'not found': base duration between retries (https://pkg.go.dev/time#ParseDuration). This can also be set with the environment variable `%s`.", constants.EnvVarTerraformRetryBaseDuration),
			},
			attributeTerraformRetryCapDuration: {
				Type:        types.StringType,
				Sensitive:   false,
				Optional:    true,
				Description: fmt.Sprintf("Retry algorithm when the API returns 'not found': max duration between retries (https://pkg.go.dev/time#ParseDuration). This can also be set with the environment variable `%s`.", constants.EnvVarTerraformRetryCapDuration),
			},
			attributeTerraformRetryMaxTries: {
				Type:        types.StringType,
				Sensitive:   false,
				Optional:    true,
				Description: fmt.Sprintf("Retry algorithm when the API returns 'not found': max tries. This can also be set with the environment variable `%s`.", constants.EnvVarTerraformRetryMaxTries),
			},
			attributeTerraformDialTimeoutDuration: {
				Type:        types.StringType,
				Sensitive:   false,
				Optional:    true,
				Description: fmt.Sprintf("DialTimeout sets timeout when trying to connect to the server. This can also be set with the environment variable `%s`.", constants.EnvVarTerraformDialTimeoutDuration),
			},
			attributeTerraformJoinMethod: {
				Type:        types.StringType,
				Sensitive:   false,
				Optional:    true,
				Description: fmt.Sprintf("Enables the native Terraform MachineID support. When set, Terraform uses MachineID to securely join the Teleport cluster and obtain credentials. See [the join method reference](../join-methods.mdx) for possible values. You must use [a delegated join method](../join-methods.mdx#secret-vs-delegated). This can also be set with the environment variable `%s`.", constants.EnvVarTerraformJoinMethod),
			},
			attributeTerraformJoinToken: {
				Type:        types.StringType,
				Sensitive:   false,
				Optional:    true,
				Description: fmt.Sprintf("Name of the token used for the native MachineID joining. This value is not sensitive for [delegated join methods](../join-methods.mdx#secret-vs-delegated). This can also be set with the environment variable `%s`.", constants.EnvVarTerraformJoinToken),
			},
			attributeTerraformJoinAudienceTag: {
				Type:        types.StringType,
				Sensitive:   false,
				Optional:    true,
				Description: fmt.Sprintf("Name of the optional audience tag used for native Machine ID joining with the `terraform` method. This can also be set with the environment variable `%s`.", constants.EnvVarTerraformCloudJoinAudienceTag),
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
	p.configureLog()

	// We wrap the provider's context into a cancellable one.
	// This allows us to cancel the context and properly close the client and any background task potentially running
	// (e.g. MachineID bot renewing creds). This is required during the tests as the provider is run multiple times.
	// You can cancel the context by calling Provider.Close()
	ctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel

	var config providerData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	addr := stringFromConfigOrEnv(config.Addr, constants.EnvVarTerraformAddress, "")
	retryBaseDurationStr := stringFromConfigOrEnv(config.RetryBaseDuration, constants.EnvVarTerraformRetryBaseDuration, "1s")
	retryCapDurationStr := stringFromConfigOrEnv(config.RetryCapDuration, constants.EnvVarTerraformRetryCapDuration, "5s")
	maxTriesStr := stringFromConfigOrEnv(config.RetryMaxTries, constants.EnvVarTerraformRetryMaxTries, "10")
	dialTimeoutDurationStr := stringFromConfigOrEnv(config.DialTimeoutDuration, constants.EnvVarTerraformDialTimeoutDuration, "30s")

	if !p.validateAddr(addr, resp) {
		return
	}

	slog.DebugContext(ctx, "Using Teleport address", "addr", addr)

	dialTimeoutDuration, err := time.ParseDuration(dialTimeoutDurationStr)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to parse Dial Timeout Duration Cap Duration",
			fmt.Sprintf(
				"Please check if %s (or %s) is set correctly. Error: %s",
				attributeTerraformDialTimeoutDuration, constants.EnvVarTerraformDialTimeoutDuration, err,
			),
		)
		return
	}

	activeSources, diags := supportedCredentialSources.ActiveSources(ctx, config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clientConfig := client.Config{
		Addrs:       []string{addr},
		DialTimeout: dialTimeoutDuration,
		DialOpts: []grpc.DialOption{
			grpc.WithReturnConnectionError(),
			grpc.WithDefaultCallOptions(
				grpc.WaitForReady(true),
			),
		},
	}

	clt, diags := activeSources.BuildClient(ctx, clientConfig, config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !p.checkTeleportVersion(ctx, clt, resp) {
		return
	}

	retryBaseDuration, err := time.ParseDuration(retryBaseDurationStr)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to parse Retry Base Duration",
			fmt.Sprintf(
				"Please check if %s (or %s) is set correctly. Error: %s",
				attributeTerraformRetryBaseDuration, constants.EnvVarTerraformRetryBaseDuration, err,
			),
		)
		return
	}

	retryCapDuration, err := time.ParseDuration(retryCapDurationStr)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to parse Retry Cap Duration",
			fmt.Sprintf(
				"Please check if %s (or %s) is set correctly. Error: %s",
				attributeTerraformRetryCapDuration, constants.EnvVarTerraformRetryCapDuration, err,
			),
		)
		return
	}

	maxTries, err := strconv.ParseUint(maxTriesStr, 10, 32)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to parse Retry Max Tries",
			fmt.Sprintf(
				"Please check if %s (or %s) is set correctly. Error: %s",
				attributeTerraformRetryMaxTries, constants.EnvVarTerraformRetryMaxTries, err,
			),
		)
		return
	}

	p.RetryConfig = RetryConfig{
		Base:     retryBaseDuration,
		Cap:      retryCapDuration,
		MaxTries: int(maxTries),
	}
	p.Client = clt
	p.configured = true
}

// checkTeleportVersion ensures that Teleport version is at least minServerVersion
func (p *Provider) checkTeleportVersion(ctx context.Context, client *client.Client, resp *tfsdk.ConfigureProviderResponse) bool {
	slog.DebugContext(ctx, "Checking Teleport server version")
	pong, err := client.Ping(ctx)
	if err != nil {
		if trace.IsNotImplemented(err) {
			resp.Diagnostics.AddError(
				"Teleport version is too old!",
				fmt.Sprintf("Server version must be at least %s", minServerVersion),
			)
			return false
		}
		slog.DebugContext(ctx, "Teleport version check error", "error", err)
		resp.Diagnostics.AddError("Unable to get Teleport server version!", "Unable to get Teleport server version!")
		return false
	}
	err = utils.CheckMinVersion(pong.ServerVersion, minServerVersion)
	if err != nil {
		slog.DebugContext(ctx, "Teleport version check error", "error", err)
		resp.Diagnostics.AddError("Teleport version check error!", err.Error())
		return false
	}
	return true
}

// stringFromConfigOrEnv returns value from config or from env var if config value is empty, default otherwise
func stringFromConfigOrEnv(value types.String, env string, def string) string {
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
			fmt.Sprintf("Please, specify either %s in provider configuration, or the %s environment variable",
				attributeTerraformAddress, constants.EnvVarTerraformAddress),
		)
		return false
	}

	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		slog.DebugContext(context.Background(), "Teleport address format error", "error", err, "addr", addr)
		resp.Diagnostics.AddError(
			"Invalid Teleport address format",
			fmt.Sprintf("Teleport address must be specified as host:port. Got %q", addr),
		)
		return false
	}
	return true
}

// TODO(hugoShaka): fix logging in a future release by converting to tflog.

// configureLog configures logging
func (p *Provider) configureLog() {
	level := slog.LevelError
	// Get Terraform log level
	switch strings.ToLower(os.Getenv("TF_LOG")) {
	case "panic", "fatal", "error":
		level = slog.LevelError
	case "warn", "warning":
		level = slog.LevelWarn
	case "info":
		level = slog.LevelInfo
	case "debug":
		level = slog.LevelDebug
	case "trace":
		level = logutils.TraceLevel
	}

	_, _, err := logutils.Initialize(logutils.Config{
		Severity: level.String(),
		Format:   "text",
	})
	if err != nil {
		return
	}

	// Show GRPC debug logs only if TF_LOG=DEBUG
	if level <= slog.LevelDebug {
		grpclog.SetLoggerV2(grpclog.NewLoggerV2(os.Stderr, os.Stderr, os.Stderr))
	}
}

// GetResources returns the map of provider resources
func (p *Provider) GetResources(_ context.Context) (map[string]tfsdk.ResourceType, diag.Diagnostics) {
	return map[string]tfsdk.ResourceType{
		"teleport_app":                        resourceTeleportAppType{},
		"teleport_auth_preference":            resourceTeleportAuthPreferenceType{},
		"teleport_cluster_maintenance_config": resourceTeleportClusterMaintenanceConfigType{},
		"teleport_cluster_networking_config":  resourceTeleportClusterNetworkingConfigType{},
		"teleport_database":                   resourceTeleportDatabaseType{},
		"teleport_dynamic_windows_desktop":    resourceTeleportDynamicWindowsDesktopType{},
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
		"teleport_installer":                  resourceTeleportInstallerType{},
		"teleport_access_monitoring_rule":     resourceTeleportAccessMonitoringRuleType{},
		"teleport_static_host_user":           resourceTeleportStaticHostUserType{},
		"teleport_workload_identity":          resourceTeleportWorkloadIdentityType{},
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
		"teleport_dynamic_windows_desktop":    dataSourceTeleportDynamicWindowsDesktopType{},
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
		"teleport_installer":                  dataSourceTeleportInstallerType{},
		"teleport_access_monitoring_rule":     dataSourceTeleportAccessMonitoringRuleType{},
		"teleport_static_host_user":           dataSourceTeleportStaticHostUserType{},
		"teleport_workload_identity":          dataSourceTeleportWorkloadIdentityType{},
	}, nil
}

// Close closes the provider's client and cancels its context.
// This is needed in the tests to avoid accumulating clients and running out of file descriptors.
func (p *Provider) Close() error {
	var err error
	if p.Client != nil {
		err = p.Client.Close()
	}
	if p.cancel != nil {
		p.cancel()
	}
	return err
}
