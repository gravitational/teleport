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

// Package config provides facilities for configuring Teleport daemons
// including
//   - parsing YAML configuration
//   - parsing CLI flags
package config

import (
	"crypto/x509"
	"errors"
	"io"
	"log/slog"
	"maps"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/go-ldap/ldap/v3"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	dtconfig "github.com/gravitational/teleport/lib/devicetrust/config"
	"github.com/gravitational/teleport/lib/integrations/externalauditstorage/easconfig"
	"github.com/gravitational/teleport/lib/integrations/samlidp/samlidpconfig"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// CommandLineFlags stores command line flag values, it's a much simplified subset
// of Teleport configuration (which is fully expressed via YAML config file)
type CommandLineFlags struct {
	// --name flag
	NodeName string
	// --auth-server flag
	AuthServerAddr []string
	// --token flag
	AuthToken string
	// --join-method flag
	JoinMethod string
	// CAPins are the SKPI hashes of the CAs used to verify the Auth Server.
	CAPins []string
	// --listen-ip flag
	ListenIP net.IP
	// --advertise-ip flag
	AdvertiseIP string
	// --config flag
	ConfigFile string
	// --apply-on-startup contains the path of a YAML manifest whose resources should be
	// applied on startup. Unlike the bootstrap flag, the resources are always applied,
	// even if the cluster is already initialized. Existing resources will be updated.
	ApplyOnStartupFile string
	// Bootstrap flag contains a YAML file that defines a set of resources to bootstrap
	// a cluster.
	BootstrapFile string
	// ConfigString is a base64 encoded configuration string
	// set by --config-string or TELEPORT_CONFIG environment variable
	ConfigString string
	// --roles flag
	Roles string
	// -d flag
	Debug bool

	// --insecure-no-tls flag
	DisableTLS bool

	// --labels flag
	Labels string
	// --pid-file flag
	PIDFile string
	// DiagnosticAddr is listen address for diagnostic endpoint
	DiagnosticAddr string
	// PermitUserEnvironment enables reading of ~/.tsh/environment
	// when creating a new session.
	PermitUserEnvironment bool

	// Insecure mode is controlled by --insecure flag and in this mode
	// Teleport won't check certificates when connecting to trusted clusters
	// It's useful for learning Teleport (following quick starts, etc).
	InsecureMode bool

	// FIPS mode means Teleport starts in a FedRAMP/FIPS 140-2 compliant
	// configuration.
	FIPS bool

	// SkipVersionCheck allows Teleport to connect to auth servers that
	// have an earlier major version number.
	SkipVersionCheck bool

	// AppName is the name of the application to proxy.
	AppName string

	// AppURI is the internal address of the application to proxy.
	AppURI string

	// AppCloud is set if application is proxying Cloud API
	AppCloud string

	// AppPublicAddr is the public address of the application to proxy.
	AppPublicAddr string

	// DatabaseName is the name of the database to proxy.
	DatabaseName string
	// DatabaseDescription is a free-form database description.
	DatabaseDescription string
	// DatabaseProtocol is the type of the proxied database e.g. postgres or mysql.
	DatabaseProtocol string
	// DatabaseURI is the address to connect to the proxied database.
	DatabaseURI string
	// DatabaseCACertFile is the database CA cert path.
	DatabaseCACertFile string
	// DatabaseAWSRegion is an optional database cloud region e.g. when using AWS RDS.
	DatabaseAWSRegion string
	// DatabaseAWSAccountID is an optional AWS account ID e.g. when using Keyspaces.
	DatabaseAWSAccountID string
	// DatabaseAWSAssumeRoleARN is an optional AWS IAM role ARN to assume when accessing the database.
	DatabaseAWSAssumeRoleARN string
	// DatabaseAWSExternalID is an optional AWS external ID used to enable assuming an AWS role across accounts.
	DatabaseAWSExternalID string
	// DatabaseAWSRedshiftClusterID is Redshift cluster identifier.
	DatabaseAWSRedshiftClusterID string
	// DatabaseAWSRDSInstanceID is RDS instance identifier.
	DatabaseAWSRDSInstanceID string
	// DatabaseAWSRDSClusterID is RDS cluster (Aurora) cluster identifier.
	DatabaseAWSRDSClusterID string
	// DatabaseAWSElastiCacheGroupID is the ElastiCache replication group identifier.
	DatabaseAWSElastiCacheGroupID string
	// DatabaseAWSMemoryDBClusterName is the MemoryDB cluster name.
	DatabaseAWSMemoryDBClusterName string
	// DatabaseAWSSessionTags is the AWS STS session tags.
	DatabaseAWSSessionTags string
	// DatabaseGCPProjectID is GCP Cloud SQL project identifier.
	DatabaseGCPProjectID string
	// DatabaseGCPInstanceID is GCP Cloud SQL instance identifier.
	DatabaseGCPInstanceID string
	// DatabaseADKeytabFile is the path to Kerberos keytab file.
	DatabaseADKeytabFile string
	// DatabaseADKrb5File is the path to krb5.conf file.
	DatabaseADKrb5File string
	// DatabaseADDomain is the Active Directory domain for authentication.
	DatabaseADDomain string
	// DatabaseADSPN is the database Service Principal Name.
	DatabaseADSPN string
	// DatabaseMySQLServerVersion is the MySQL server version reported to a client
	// if the value cannot be obtained from the database.
	DatabaseMySQLServerVersion string

	// ProxyServer is the url of the proxy server to connect to.
	ProxyServer string
	// OpenSSHConfigPath is the path of the file to write agentless configuration to.
	OpenSSHConfigPath string
	// RestartOpenSSH indicates whether openssh should be restarted or not.
	RestartOpenSSH bool
	// RestartCommand is the command to use when restarting sshd
	RestartCommand string
	// CheckCommand is the command to use when checking sshd config validity
	CheckCommand string
	// Address is the ip address of the OpenSSH node.
	Address string
	// AdditionalPrincipals is a list of additional principals to include in the SSH cert.
	AdditionalPrincipals string
	// Directory to store
	DataDir string

	// IntegrationConfDeployServiceIAMArguments contains the arguments of
	// `teleport integration configure deployservice-iam` command
	IntegrationConfDeployServiceIAMArguments IntegrationConfDeployServiceIAM

	// IntegrationConfEICEIAMArguments contains the arguments of
	// `teleport integration configure eice-iam` command
	IntegrationConfEICEIAMArguments IntegrationConfEICEIAM

	// IntegrationConfEKSIAMArguments contains the arguments of
	// `teleport integration configure eks-iam` command
	IntegrationConfEKSIAMArguments IntegrationConfEKSIAM

	// IntegrationConfAWSOIDCIdPArguments contains the arguments of
	// `teleport integration configure awsoidc-idp` command
	IntegrationConfAWSOIDCIdPArguments IntegrationConfAWSOIDCIdP

	// IntegrationConfListDatabasesIAMArguments contains the arguments of
	// `teleport integration configure listdatabases-iam` command
	IntegrationConfListDatabasesIAMArguments IntegrationConfListDatabasesIAM

	// IntegrationConfExternalAuditStorageArguments contains the arguments of the
	// `teleport integration configure externalauditstorage` command
	IntegrationConfExternalAuditStorageArguments easconfig.ExternalAuditStorageConfiguration

	// IntegrationConfAccessGraphAWSSyncArguments contains the arguments of
	// `teleport integration configure access-graph aws-iam` command
	IntegrationConfAccessGraphAWSSyncArguments IntegrationConfAccessGraphAWSSync

	// IntegrationConfSAMLIdPGCPWorkforceArguments contains the arguments of
	// `teleport integration configure samlidp gcp-workforce` command
	IntegrationConfSAMLIdPGCPWorkforceArguments samlidpconfig.GCPWorkforceAPIParams
}

// IntegrationConfAccessGraphAWSSync contains the arguments of
// `teleport integration configure access-graph aws-iam` command.
type IntegrationConfAccessGraphAWSSync struct {
	// Role is the AWS Role associated with the Integration
	Role string
}

// IntegrationConfDeployServiceIAM contains the arguments of
// `teleport integration configure deployservice-iam` command
type IntegrationConfDeployServiceIAM struct {
	// Cluster is the teleport cluster name.
	Cluster string
	// Name is the integration name.
	Name string
	// Region is the AWS Region used to set up the client.
	Region string
	// Role is the AWS Role associated with the Integration
	Role string
	// TaskRole is the AWS Role to be used by the deployed service.
	TaskRole string
}

// IntegrationConfEICEIAM contains the arguments of
// `teleport integration configure eice-iam` command
type IntegrationConfEICEIAM struct {
	// Region is the AWS Region used to set up the client.
	Region string
	// Role is the AWS Role associated with the Integration
	Role string
}

// IntegrationConfEKSIAM contains the arguments of
// `teleport integration configure eks-iam` command
type IntegrationConfEKSIAM struct {
	// Region is the AWS Region used to set up the client.
	Region string
	// Role is the AWS Role associated with the Integration
	Role string
}

// IntegrationConfAWSOIDCIdP contains the arguments of
// `teleport integration configure awsoidc-idp` command
type IntegrationConfAWSOIDCIdP struct {
	// Cluster is the teleport cluster name.
	Cluster string
	// Name is the integration name.
	Name string
	// Role is the AWS Role to associate with the Integration
	Role string
	// ProxyPublicURL is the IdP Issuer URL (Teleport Proxy Public Address).
	// Eg, https://<tenant>.teleport.sh
	ProxyPublicURL string

	// S3BucketURI is the S3 URI which contains the bucket name and prefix for the issuer.
	// Format: s3://<bucket-name>/<prefix>
	// Eg, s3://my-bucket/idp-teleport
	// This is used in two places:
	// - create openid configuration and jwks objects
	// - set up the issuer
	// The bucket must be public and will be created if it doesn't exist.
	//
	// If empty, the ProxyPublicAddress is used as issuer and no s3 objects are created.
	S3BucketURI string

	// S3JWKSContentsB64 must contain the public keys for the Issuer.
	// The contents must be Base64 encoded.
	// Eg. base64(`{"keys":[{"kty":"RSA","alg":"RS256","n":"<value of n>","e":"<value of e>","use":"sig","kid":""}]}`)
	S3JWKSContentsB64 string
}

// IntegrationConfListDatabasesIAM contains the arguments of
// `teleport integration configure listdatabases-iam` command
type IntegrationConfListDatabasesIAM struct {
	// Region is the AWS Region used to set up the client.
	Region string
	// Role is the AWS Role associated with the Integration
	Role string
}

// ReadConfigFile reads /etc/teleport.yaml (or whatever is passed via --config flag)
// and overrides values in 'cfg' structure
func ReadConfigFile(cliConfigPath string) (*FileConfig, error) {
	configFilePath := defaults.ConfigFilePath
	// --config tells us to use a specific conf. file:
	if cliConfigPath != "" {
		configFilePath = cliConfigPath
		if !utils.FileExists(configFilePath) {
			return nil, trace.NotFound("file %s is not found", configFilePath)
		}
	}
	// default config doesn't exist? quietly return:
	if !utils.FileExists(configFilePath) {
		log.Info("not using a config file")
		return nil, nil
	}
	log.Debug("reading config file: ", configFilePath)
	return ReadFromFile(configFilePath)
}

// ReadResources loads a set of resources from a file.
func ReadResources(filePath string) ([]types.Resource, error) {
	reader, err := utils.OpenFileAllowingUnsafeLinks(filePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer reader.Close()
	decoder := kyaml.NewYAMLOrJSONDecoder(reader, defaults.LookaheadBufSize)
	var resources []types.Resource
	for {
		var raw services.UnknownResource
		err := decoder.Decode(&raw)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, trace.Wrap(err)
		}
		rsc, err := services.UnmarshalResource(raw.Kind, raw.Raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, rsc)
	}
	return resources, nil
}

// ApplyFileConfig applies configuration from a YAML file to Teleport
// runtime config
func ApplyFileConfig(fc *FileConfig, cfg *servicecfg.Config) error {
	var err error

	// no config file? no problem
	if fc == nil {
		return nil
	}

	applyConfigVersion(fc, cfg)

	// merge file-based config with defaults in 'cfg'
	if fc.Auth.Disabled() {
		cfg.Auth.Enabled = false
	}
	if fc.SSH.Disabled() {
		cfg.SSH.Enabled = false
	}
	if fc.Proxy.Disabled() {
		cfg.Proxy.Enabled = false
	}
	if fc.Kube.Enabled() {
		cfg.Kube.Enabled = true
	}
	if fc.Apps.Disabled() {
		cfg.Apps.Enabled = false
	}
	if fc.Databases.Disabled() {
		cfg.Databases.Enabled = false
	}
	if fc.Metrics.Disabled() {
		cfg.Metrics.Enabled = false
	}
	if fc.WindowsDesktop.Disabled() {
		cfg.WindowsDesktop.Enabled = false
	}

	if fc.AccessGraph.Enabled {
		cfg.AccessGraph.Enabled = true
		if fc.AccessGraph.Endpoint == "" {
			return trace.BadParameter("access_graph.endpoint is required when access graph integration is enabled")
		}
		cfg.AccessGraph.Addr = fc.AccessGraph.Endpoint
		cfg.AccessGraph.CA = fc.AccessGraph.CA
		// TODO(tigrato): change this behavior when we drop support for plain text connections
		cfg.AccessGraph.Insecure = fc.AccessGraph.Insecure
	}

	applyString(fc.NodeName, &cfg.Hostname)

	// apply "advertise_ip" setting:
	advertiseIP := fc.AdvertiseIP
	if advertiseIP != "" {
		if _, _, err := utils.ParseAdvertiseAddr(advertiseIP); err != nil {
			return trace.Wrap(err)
		}
		cfg.AdvertiseIP = advertiseIP
	}
	cfg.PIDFile = fc.PIDFile

	if err := applyAuthOrProxyAddress(fc, cfg); err != nil {
		return trace.Wrap(err)
	}

	if err := applyTokenConfig(fc, cfg); err != nil {
		return trace.Wrap(err)
	}

	if fc.Global.DataDir != "" {
		cfg.DataDir = fc.Global.DataDir
		cfg.Auth.StorageConfig.Params["path"] = cfg.DataDir
	}

	// If a backend is specified, override the defaults.
	if fc.Storage.Type != "" {
		// If the alternative name "dir" is given, update it to "lite".
		if fc.Storage.Type == lite.AlternativeName {
			fc.Storage.Type = lite.GetName()
		}

		cfg.Auth.StorageConfig = fc.Storage
		// backend is specified, but no path is set, set a reasonable default
		_, pathSet := cfg.Auth.StorageConfig.Params[defaults.BackendPath]
		if cfg.Auth.StorageConfig.Type == lite.GetName() && !pathSet {
			if cfg.Auth.StorageConfig.Params == nil {
				cfg.Auth.StorageConfig.Params = make(backend.Params)
			}
			cfg.Auth.StorageConfig.Params[defaults.BackendPath] = filepath.Join(cfg.DataDir, defaults.BackendDir)
		}
	} else {
		// Set a reasonable default.
		cfg.Auth.StorageConfig.Params[defaults.BackendPath] = filepath.Join(cfg.DataDir, defaults.BackendDir)
	}

	// apply logger settings
	err = applyLogConfig(fc.Logger, cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	if fc.CachePolicy.TTL != "" {
		log.Warn("cache.ttl config option is deprecated and will be ignored, caches no longer attempt to anticipate resource expiration.")
	}
	if fc.CachePolicy.Type == memory.GetName() {
		log.Debugf("cache.type config option is explicitly set to %v.", memory.GetName())
	} else if fc.CachePolicy.Type != "" {
		log.Warn("cache.type config option is deprecated and will be ignored, caches are always in memory in this version.")
	}

	// apply cache policy for node and proxy
	cachePolicy, err := fc.CachePolicy.Parse()
	if err != nil {
		return trace.Wrap(err)
	}
	cfg.CachePolicy = *cachePolicy

	// Apply (TLS) cipher suites and (SSH) ciphers, KEX algorithms, and MAC
	// algorithms.
	if len(fc.CipherSuites) > 0 {
		cipherSuites, err := utils.CipherSuiteMapping(fc.CipherSuites)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.CipherSuites = cipherSuites
	}
	if fc.Ciphers != nil {
		cfg.Ciphers = fc.Ciphers
	}
	if fc.KEXAlgorithms != nil {
		cfg.KEXAlgorithms = fc.KEXAlgorithms
	}
	if fc.MACAlgorithms != nil {
		cfg.MACAlgorithms = fc.MACAlgorithms
	}
	if fc.CASignatureAlgorithm != nil {
		log.Warn("ca_signing_algo config option is deprecated and will be removed in a future release, Teleport defaults to rsa-sha2-512.")
	}

	// Read in how nodes will validate the CA. A single empty string in the file
	// conf should indicate no pins.
	if err = cfg.ApplyCAPins(fc.CAPin); err != nil {
		return trace.Wrap(err)
	}

	// Set diagnostic address
	if fc.DiagAddr != "" {
		// Validate address
		parsed, err := utils.ParseAddr(fc.DiagAddr)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.DiagnosticAddr = *parsed
	}

	// apply connection throttling:
	limiters := []*limiter.Config{
		&cfg.SSH.Limiter,
		&cfg.Auth.Limiter,
		&cfg.Proxy.Limiter,
		&cfg.Databases.Limiter,
		&cfg.Kube.Limiter,
		&cfg.WindowsDesktop.ConnLimiter,
	}
	for _, l := range limiters {
		if fc.Limits.MaxConnections > 0 {
			l.MaxConnections = fc.Limits.MaxConnections
		}
		if fc.Limits.MaxUsers > 0 {
			l.MaxNumberOfUsers = fc.Limits.MaxUsers
		}
		for _, rate := range fc.Limits.Rates {
			l.Rates = append(l.Rates, limiter.Rate{
				Period:  rate.Period,
				Average: rate.Average,
				Burst:   rate.Burst,
			})
		}
	}

	// Apply configuration for "auth_service", "proxy_service", "ssh_service",
	// and "app_service" if they are enabled.
	if fc.Auth.Enabled() {
		err = applyAuthConfig(fc, cfg)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if fc.Proxy.Enabled() {
		err = applyProxyConfig(fc, cfg)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if fc.SSH.Enabled() {
		err = applySSHConfig(fc, cfg)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if fc.Kube.Enabled() {
		if err := applyKubeConfig(fc, cfg); err != nil {
			return trace.Wrap(err)
		}
	}
	if fc.Apps.Enabled() {
		if err := applyAppsConfig(fc, cfg); err != nil {
			return trace.Wrap(err)
		}
	}
	if fc.Databases.Enabled() {
		if err := applyDatabasesConfig(fc, cfg); err != nil {
			return trace.Wrap(err)
		}
	}
	if fc.Metrics.Enabled() {
		if err := applyMetricsConfig(fc, cfg); err != nil {
			return trace.Wrap(err)
		}
	}
	if fc.WindowsDesktop.Enabled() {
		if err := applyWindowsDesktopConfig(fc, cfg); err != nil {
			return trace.Wrap(err)
		}
	}
	if fc.Tracing.Enabled() {
		if err := applyTracingConfig(fc, cfg); err != nil {
			return trace.Wrap(err)
		}
	}

	if fc.Discovery.Enabled() {
		if err := applyDiscoveryConfig(fc, cfg); err != nil {
			return trace.Wrap(err)
		}
	}

	if fc.Okta.Enabled() {
		if err := applyOktaConfig(fc, cfg); err != nil {
			return trace.Wrap(err)
		}
	}

	// Apply regardless of Jamf being enabled.
	// If a config is present, we want it to be valid.
	if err := applyJamfConfig(fc, cfg); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func applyAuthOrProxyAddress(fc *FileConfig, cfg *servicecfg.Config) error {
	switch cfg.Version {
	// For config versions v1 and v2, the auth_servers field can point to an auth
	// server or a proxy server
	case defaults.TeleportConfigVersionV1, defaults.TeleportConfigVersionV2:
		// config file has auth servers in there?
		if len(fc.AuthServers) > 0 {
			var parsedAddresses []utils.NetAddr

			for _, as := range fc.AuthServers {
				addr, err := utils.ParseHostPortAddr(as, defaults.AuthListenPort)
				if err != nil {
					return trace.Wrap(err)
				}

				parsedAddresses = append(parsedAddresses, *addr)
			}

			if err := cfg.SetAuthServerAddresses(parsedAddresses); err != nil {
				return trace.Wrap(err)
			}
		}

		if fc.AuthServer != "" {
			return trace.BadParameter("auth_server is supported from config version v3 onwards")
		}

		if fc.ProxyServer != "" {
			return trace.BadParameter("proxy_server is supported from config version v3 onwards")
		}

	// From v3 onwards, either auth_server or proxy_server should be set
	case defaults.TeleportConfigVersionV3:
		if len(fc.AuthServers) > 0 {
			return trace.BadParameter("config v3 has replaced auth_servers with either auth_server or proxy_server")
		}

		haveAuthServer := fc.AuthServer != ""
		haveProxyServer := fc.ProxyServer != ""

		if haveProxyServer && haveAuthServer {
			return trace.BadParameter("only one of auth_server or proxy_server should be set")
		}

		if haveAuthServer {
			addr, err := utils.ParseHostPortAddr(fc.AuthServer, defaults.AuthListenPort)
			if err != nil {
				return trace.Wrap(err)
			}

			cfg.SetAuthServerAddress(*addr)
		}

		if haveProxyServer {
			if fc.Proxy.Enabled() {
				return trace.BadParameter("proxy_server can not be specified when proxy service is enabled")
			}

			addr, err := utils.ParseHostPortAddr(fc.ProxyServer, defaults.HTTPListenPort)
			if err != nil {
				return trace.Wrap(err)
			}

			cfg.ProxyServer = *addr
		}
	}

	return nil
}

func applyLogConfig(loggerConfig Log, cfg *servicecfg.Config) error {
	logger := log.StandardLogger()

	var w io.Writer
	switch loggerConfig.Output {
	case "":
		w = os.Stderr
	case "stderr", "error", "2":
		w = os.Stderr
		cfg.Console = io.Discard // disable console printing
	case "stdout", "out", "1":
		w = os.Stdout
		cfg.Console = io.Discard // disable console printing
	case teleport.Syslog:
		w = os.Stderr
		sw, err := utils.NewSyslogWriter()
		if err != nil {
			logger.Errorf("Failed to switch logging to syslog: %v.", err)
			break
		}

		hook, err := utils.NewSyslogHook(sw)
		if err != nil {
			logger.Errorf("Failed to switch logging to syslog: %v.", err)
			break
		}

		logger.ReplaceHooks(make(log.LevelHooks))
		logger.AddHook(hook)
		w = sw
	default:
		// assume it's a file path:
		logFile, err := os.Create(loggerConfig.Output)
		if err != nil {
			return trace.Wrap(err, "failed to create the log file")
		}
		w = logFile
	}

	level := new(slog.LevelVar)
	switch strings.ToLower(loggerConfig.Severity) {
	case "", "info":
		logger.SetLevel(log.InfoLevel)
		level.Set(slog.LevelInfo)
	case "err", "error":
		logger.SetLevel(log.ErrorLevel)
		level.Set(slog.LevelError)
	case teleport.DebugLevel:
		logger.SetLevel(log.DebugLevel)
		level.Set(slog.LevelDebug)
	case "warn", "warning":
		logger.SetLevel(log.WarnLevel)
		level.Set(slog.LevelWarn)
	case "trace":
		logger.SetLevel(log.TraceLevel)
		level.Set(logutils.TraceLevel)
	default:
		return trace.BadParameter("unsupported logger severity: %q", loggerConfig.Severity)
	}

	configuredFields, err := logutils.ValidateFields(loggerConfig.Format.ExtraFields)
	if err != nil {
		return trace.Wrap(err)
	}

	// If syslog output has been configured and is supported by the operating system,
	// then the shared writer is not needed because the syslog writer is already
	// protected with a mutex.
	if len(logger.Hooks) == 0 {
		w = logutils.NewSharedWriter(w)
	}
	var slogLogger *slog.Logger
	switch strings.ToLower(loggerConfig.Format.Output) {
	case "":
		fallthrough // not set. defaults to 'text'
	case "text":
		enableColors := utils.IsTerminal(os.Stderr)
		formatter := &logutils.TextFormatter{
			ExtraFields:  configuredFields,
			EnableColors: enableColors,
		}

		if err := formatter.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

		logger.SetFormatter(formatter)
		// Disable writing output to stderr/stdout and syslog. The logging
		// hook will take care of writing the output to the correct location.
		if len(logger.Hooks) > 0 {
			logger.SetOutput(io.Discard)
		} else {
			logger.SetOutput(w)
		}

		slogLogger = slog.New(logutils.NewSlogTextHandler(w, logutils.SlogTextHandlerConfig{
			Level:            level,
			EnableColors:     enableColors,
			ConfiguredFields: configuredFields,
		}))
		slog.SetDefault(slogLogger)
	case "json":
		formatter := &logutils.JSONFormatter{
			ExtraFields: configuredFields,
		}

		if err := formatter.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

		logger.SetFormatter(formatter)
		// Disable writing output to stderr/stdout and syslog. The logging
		// hook will take care of writing the output to the correct location.
		if len(logger.Hooks) > 0 {
			logger.SetOutput(io.Discard)
		} else {
			logger.SetOutput(w)
		}

		slogLogger = slog.New(logutils.NewSlogJSONHandler(w, logutils.SlogJSONHandlerConfig{
			Level:            level,
			ConfiguredFields: configuredFields,
		}))
		slog.SetDefault(slogLogger)
	default:
		return trace.BadParameter("unsupported log output format : %q", loggerConfig.Format.Output)
	}

	cfg.Log = logger
	cfg.Logger = slogLogger
	cfg.LoggerLevel = level
	return nil
}

// applyAuthConfig applies file configuration for the "auth_service" section.
func applyAuthConfig(fc *FileConfig, cfg *servicecfg.Config) error {
	var err error

	if fc.Auth.KubeconfigFile != "" {
		warningMessage := "The auth_service no longer needs kubeconfig_file. It has " +
			"been moved to proxy_service section. This setting is ignored."
		log.Warning(warningMessage)
	}

	cfg.Auth.PROXYProtocolMode = multiplexer.PROXYProtocolUnspecified
	if fc.Auth.ProxyProtocol != "" {
		val := multiplexer.PROXYProtocolMode(fc.Auth.ProxyProtocol)
		if err := validatePROXYProtocolValue(val); err != nil {
			return trace.Wrap(err)
		}

		cfg.Auth.PROXYProtocolMode = val
	}

	if fc.Auth.ListenAddress != "" {
		addr, err := utils.ParseHostPortAddr(fc.Auth.ListenAddress, int(defaults.AuthListenPort))
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Auth.ListenAddr = *addr
		if len(cfg.AuthServerAddresses()) == 0 {
			cfg.SetAuthServerAddress(*addr)
		}
	}
	for _, t := range fc.Auth.ReverseTunnels {
		tun, err := t.ConvertAndValidate()
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.ReverseTunnels = append(cfg.ReverseTunnels, tun)
	}
	if len(fc.Auth.PublicAddr) != 0 {
		addrs, err := utils.AddrsFromStrings(fc.Auth.PublicAddr, defaults.AuthListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Auth.PublicAddrs = addrs
	}
	// read in cluster name from file configuration and create services.ClusterName
	cfg.Auth.ClusterName, err = fc.Auth.ClusterName.Parse()
	if err != nil {
		return trace.Wrap(err)
	}
	// read in static tokens from file configuration and create services.StaticTokens
	if fc.Auth.StaticTokens != nil {
		cfg.Auth.StaticTokens, err = fc.Auth.StaticTokens.Parse()
		if err != nil {
			return trace.Wrap(err)
		}
	}
	// read in and set authentication preferences
	if fc.Auth.Authentication != nil {
		cfg.Auth.Preference, err = fc.Auth.Authentication.Parse()
		if err != nil {
			return trace.Wrap(err)
		}
		if err := dtconfig.ValidateConfigAgainstModules(cfg.Auth.Preference.GetDeviceTrust()); err != nil {
			return trace.Wrap(err)
		}
	}

	if fc.Auth.MessageOfTheDay != "" {
		cfg.Auth.Preference.SetMessageOfTheDay(fc.Auth.MessageOfTheDay)
	}

	if fc.Auth.DisconnectExpiredCert != nil {
		cfg.Auth.Preference.SetOrigin(types.OriginConfigFile)
		cfg.Auth.Preference.SetDisconnectExpiredCert(fc.Auth.DisconnectExpiredCert.Value)
	}

	if fc.Auth.Assist != nil && fc.Auth.Assist.OpenAI != nil {
		keyPath := fc.Auth.Assist.OpenAI.APITokenPath
		key, err := os.ReadFile(keyPath)
		if err != nil {
			return trace.Errorf("failed to read OpenAI API key file: %w", err)
		}
		cfg.Auth.AssistAPIKey = strings.TrimSpace(string(key))

		if fc.Auth.Assist.CommandExecutionWorkers < 0 {
			return trace.BadParameter("command_execution_workers must not be negative")
		}
	}

	// Set cluster audit configuration from file configuration.
	auditConfigSpec, err := services.ClusterAuditConfigSpecFromObject(fc.Storage.Params)
	if err != nil {
		return trace.Wrap(err)
	}
	auditConfigSpec.Type = fc.Storage.Type
	cfg.Auth.AuditConfig, err = types.NewClusterAuditConfig(*auditConfigSpec)
	if err != nil {
		return trace.Wrap(err)
	}

	// Only override networking configuration if some of its fields are
	// specified in file configuration.
	if fc.Auth.hasCustomNetworkingConfig() {
		var assistCommandExecutionWorkers int32
		if fc.Auth.Assist != nil {
			assistCommandExecutionWorkers = fc.Auth.Assist.CommandExecutionWorkers
		}
		cfg.Auth.NetworkingConfig, err = types.NewClusterNetworkingConfigFromConfigFile(types.ClusterNetworkingConfigSpecV2{
			ClientIdleTimeout:             fc.Auth.ClientIdleTimeout,
			ClientIdleTimeoutMessage:      fc.Auth.ClientIdleTimeoutMessage,
			WebIdleTimeout:                fc.Auth.WebIdleTimeout,
			KeepAliveInterval:             fc.Auth.KeepAliveInterval,
			KeepAliveCountMax:             fc.Auth.KeepAliveCountMax,
			SessionControlTimeout:         fc.Auth.SessionControlTimeout,
			ProxyListenerMode:             fc.Auth.ProxyListenerMode,
			RoutingStrategy:               fc.Auth.RoutingStrategy,
			TunnelStrategy:                fc.Auth.TunnelStrategy,
			ProxyPingInterval:             fc.Auth.ProxyPingInterval,
			AssistCommandExecutionWorkers: assistCommandExecutionWorkers,
			CaseInsensitiveRouting:        fc.Auth.CaseInsensitiveRouting,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// Only override session recording configuration if either field is
	// specified in file configuration.
	if fc.Auth.hasCustomSessionRecording() {
		cfg.Auth.SessionRecordingConfig, err = types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
			Mode:                fc.Auth.SessionRecording,
			ProxyChecksHostKeys: fc.Auth.ProxyChecksHostKeys,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if err := applyKeyStoreConfig(fc, cfg); err != nil {
		return trace.Wrap(err)
	}

	// read in and set the license file path (not used in open-source version)
	switch licenseFile := fc.Auth.LicenseFile; {
	case licenseFile == "":
		cfg.Auth.LicenseFile = filepath.Join(cfg.DataDir, defaults.LicenseFile)
	case filepath.IsAbs(licenseFile):
		cfg.Auth.LicenseFile = licenseFile
	default:
		cfg.Auth.LicenseFile = filepath.Join(cfg.DataDir, licenseFile)
	}

	cfg.Auth.LoadAllCAs = fc.Auth.LoadAllCAs

	// Setting this to true at all times to allow self hosting
	// of plugins that were previously cloud only.
	cfg.Auth.HostedPlugins.Enabled = true
	cfg.Auth.HostedPlugins.OAuthProviders, err = fc.Auth.HostedPlugins.OAuthProviders.Parse()
	if err != nil {
		return trace.Wrap(err)
	}

	if fc.Auth.AccessMonitoring != nil {
		if err := fc.Auth.AccessMonitoring.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err, "failed to validate access monitoring config")
		}
		cfg.Auth.AccessMonitoring = fc.Auth.AccessMonitoring
	}

	return nil
}

func applyKeyStoreConfig(fc *FileConfig, cfg *servicecfg.Config) error {
	if fc.Auth.CAKeyParams == nil {
		return nil
	}
	if fc.Auth.CAKeyParams.PKCS11 != nil {
		if fc.Auth.CAKeyParams.GoogleCloudKMS != nil {
			return trace.BadParameter("cannot set both pkcs11 and gcp_kms in file config")
		}
		if fc.Auth.CAKeyParams.AWSKMS != nil {
			return trace.BadParameter("cannot set both pkcs11 and aws_kms in file config")
		}
		return trace.Wrap(applyPKCS11Config(fc.Auth.CAKeyParams.PKCS11, cfg))
	}
	if fc.Auth.CAKeyParams.GoogleCloudKMS != nil {
		if fc.Auth.CAKeyParams.AWSKMS != nil {
			return trace.BadParameter("cannot set both gpc_kms and aws_kms in file config")
		}
		return trace.Wrap(applyGoogleCloudKMSConfig(fc.Auth.CAKeyParams.GoogleCloudKMS, cfg))
	}
	if fc.Auth.CAKeyParams.AWSKMS != nil {
		return trace.Wrap(applyAWSKMSConfig(fc.Auth.CAKeyParams.AWSKMS, cfg))
	}
	return nil
}

func applyPKCS11Config(pkcs11Config *PKCS11, cfg *servicecfg.Config) error {
	if pkcs11Config.ModulePath != "" {
		fi, err := utils.StatFile(pkcs11Config.ModulePath)
		if err != nil {
			return trace.Wrap(err)
		}

		const worldWritableBits = 0o002
		if fi.Mode().Perm()&worldWritableBits != 0 {
			return trace.Errorf(
				"PKCS11 library (%s) must not be world-writable",
				pkcs11Config.ModulePath,
			)
		}

		cfg.Auth.KeyStore.PKCS11.Path = pkcs11Config.ModulePath
	}

	cfg.Auth.KeyStore.PKCS11.TokenLabel = pkcs11Config.TokenLabel
	cfg.Auth.KeyStore.PKCS11.SlotNumber = pkcs11Config.SlotNumber

	cfg.Auth.KeyStore.PKCS11.Pin = pkcs11Config.Pin
	if pkcs11Config.PinPath != "" {
		if pkcs11Config.Pin != "" {
			return trace.BadParameter("can not set both pin and pin_path")
		}

		fi, err := utils.StatFile(pkcs11Config.PinPath)
		if err != nil {
			return trace.Wrap(err)
		}

		const worldReadableBits = 0o004
		if fi.Mode().Perm()&worldReadableBits != 0 {
			return trace.Errorf(
				"HSM pin file (%s) must not be world-readable",
				pkcs11Config.PinPath,
			)
		}

		pinBytes, err := os.ReadFile(pkcs11Config.PinPath)
		if err != nil {
			return trace.Wrap(err)
		}
		pin := strings.TrimRight(string(pinBytes), "\r\n")
		cfg.Auth.KeyStore.PKCS11.Pin = pin
	}
	return nil
}

func applyGoogleCloudKMSConfig(kmsConfig *GoogleCloudKMS, cfg *servicecfg.Config) error {
	if kmsConfig.KeyRing == "" {
		return trace.BadParameter("must set keyring in ca_key_params.gcp_kms")
	}
	cfg.Auth.KeyStore.GCPKMS.KeyRing = kmsConfig.KeyRing
	if kmsConfig.ProtectionLevel == "" {
		return trace.BadParameter("must set protection_level in ca_key_params.gcp_kms")
	}
	cfg.Auth.KeyStore.GCPKMS.ProtectionLevel = kmsConfig.ProtectionLevel
	return nil
}

func applyAWSKMSConfig(kmsConfig *AWSKMS, cfg *servicecfg.Config) error {
	if kmsConfig.Account == "" {
		return trace.BadParameter("must set account in ca_key_params.aws_kms")
	}
	cfg.Auth.KeyStore.AWSKMS.AWSAccount = kmsConfig.Account
	if kmsConfig.Region == "" {
		return trace.BadParameter("must set region in ca_key_params.aws_kms")
	}
	cfg.Auth.KeyStore.AWSKMS.AWSRegion = kmsConfig.Region
	return nil
}

func validatePROXYProtocolValue(p multiplexer.PROXYProtocolMode) error {
	allowedOptions := []multiplexer.PROXYProtocolMode{multiplexer.PROXYProtocolOn, multiplexer.PROXYProtocolOff}

	if !slices.Contains(allowedOptions, p) {
		return trace.BadParameter("invalid 'proxy_protocol' value %q. Available options are: %v", p, allowedOptions)
	}
	return nil
}

// applyProxyConfig applies file configuration for the "proxy_service" section.
func applyProxyConfig(fc *FileConfig, cfg *servicecfg.Config) error {
	var err error

	cfg.Proxy.PROXYProtocolMode = multiplexer.PROXYProtocolUnspecified
	if fc.Proxy.ProxyProtocol != "" {
		val := multiplexer.PROXYProtocolMode(fc.Proxy.ProxyProtocol)
		if err := validatePROXYProtocolValue(val); err != nil {
			return trace.Wrap(err)
		}

		cfg.Proxy.PROXYProtocolMode = val
	}

	if fc.Proxy.ListenAddress != "" {
		addr, err := utils.ParseHostPortAddr(fc.Proxy.ListenAddress, defaults.SSHProxyListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.SSHAddr = *addr
	}
	if fc.Proxy.WebAddr != "" {
		addr, err := utils.ParseHostPortAddr(fc.Proxy.WebAddr, defaults.HTTPListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.WebAddr = *addr
	}
	if fc.Proxy.TunAddr != "" {
		addr, err := utils.ParseHostPortAddr(fc.Proxy.TunAddr, defaults.SSHProxyTunnelListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.ReverseTunnelListenAddr = *addr
	}
	if fc.Proxy.MySQLAddr != "" {
		addr, err := utils.ParseHostPortAddr(fc.Proxy.MySQLAddr, defaults.MySQLListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.MySQLAddr = *addr
	}
	if fc.Proxy.PostgresAddr != "" {
		addr, err := utils.ParseHostPortAddr(fc.Proxy.PostgresAddr, defaults.PostgresListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.PostgresAddr = *addr
	}
	if fc.Proxy.MongoAddr != "" {
		addr, err := utils.ParseHostPortAddr(fc.Proxy.MongoAddr, defaults.MongoListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.MongoAddr = *addr
	}
	if fc.Proxy.PeerAddr != "" {
		addr, err := utils.ParseHostPortAddr(fc.Proxy.PeerAddr, int(defaults.ProxyPeeringListenPort))
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.PeerAddress = *addr
	}

	if fc.Proxy.UI != nil {
		cfg.Proxy.UI = webclient.UIConfig(*fc.Proxy.UI)
	}

	if fc.Proxy.Assist != nil && fc.Proxy.Assist.OpenAI != nil {
		keyPath := fc.Proxy.Assist.OpenAI.APITokenPath
		key, err := os.ReadFile(keyPath)
		if err != nil {
			return trace.BadParameter("failed to read OpenAI API key file at path %s: %v",
				keyPath, trace.ConvertSystemError(err))
		} else {
			cfg.Proxy.AssistAPIKey = strings.TrimSpace(string(key))
		}
	}

	if fc.Proxy.MySQLServerVersion != "" {
		cfg.Proxy.MySQLServerVersion = fc.Proxy.MySQLServerVersion
	}

	if fc.Proxy.AutomaticUpgradesChannels != nil {
		cfg.Proxy.AutomaticUpgradesChannels = fc.Proxy.AutomaticUpgradesChannels
	} else {
		cfg.Proxy.AutomaticUpgradesChannels = make(automaticupgrades.Channels)
	}
	if err = cfg.Proxy.AutomaticUpgradesChannels.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating the automatic upgrades configuration")
	}

	// This is the legacy format. Continue to support it forever, but ideally
	// users now use the list format below.
	if fc.Proxy.KeyFile != "" || fc.Proxy.CertFile != "" {
		cfg.Proxy.KeyPairs = append(cfg.Proxy.KeyPairs, servicecfg.KeyPairPath{
			PrivateKey:  fc.Proxy.KeyFile,
			Certificate: fc.Proxy.CertFile,
		})
	}
	for _, p := range fc.Proxy.KeyPairs {
		// Check that the certificate exists on disk. This exists to provide the
		// user a sensible error message.
		if !utils.FileExists(p.PrivateKey) {
			return trace.Errorf("https private key does not exist: %s", p.PrivateKey)
		}
		if !utils.FileExists(p.Certificate) {
			return trace.Errorf("https cert does not exist: %s", p.Certificate)
		}

		// Read in certificate from disk. If Teleport finds a self-signed
		// certificate chain, log a warning, and then accept whatever certificate
		// was passed. If the certificate is not self-signed, verify the certificate
		// chain from leaf to root with the trust store on the computer so browsers
		// don't complain.
		certificateChain, err := utils.ReadCertificatesFromPath(p.Certificate)
		if err != nil {
			return trace.Wrap(err)
		}
		if utils.IsSelfSigned(certificateChain) {
			warningMessage := "Starting Teleport with a self-signed TLS certificate, this is " +
				"not safe for production clusters. Using a self-signed certificate opens " +
				"Teleport users to Man-in-the-Middle attacks."
			log.Warnf(warningMessage)
		} else {
			if err := utils.VerifyCertificateChain(certificateChain); err != nil {
				return trace.BadParameter("unable to verify HTTPS certificate chain in %v: %s",
					fc.Proxy.CertFile, utils.UserMessageFromError(err))
			}
		}

		cfg.Proxy.KeyPairs = append(cfg.Proxy.KeyPairs, servicecfg.KeyPairPath{
			PrivateKey:  p.PrivateKey,
			Certificate: p.Certificate,
		})
	}
	cfg.Proxy.KeyPairsReloadInterval = fc.Proxy.KeyPairsReloadInterval

	// apply kubernetes proxy config, by default kube proxy is disabled
	legacyKube := fc.Proxy.Kube.Configured() && fc.Proxy.Kube.Enabled()
	newKube := fc.Proxy.KubeAddr != "" || len(fc.Proxy.KubePublicAddr) > 0
	switch {
	case legacyKube && !newKube:
		cfg.Proxy.Kube.Enabled = true
		cfg.Proxy.Kube.LegacyKubeProxy = true
		if fc.Proxy.Kube.KubeconfigFile != "" {
			cfg.Proxy.Kube.KubeconfigPath = fc.Proxy.Kube.KubeconfigFile
		}
		if fc.Proxy.Kube.ListenAddress != "" {
			addr, err := utils.ParseHostPortAddr(fc.Proxy.Kube.ListenAddress, int(defaults.KubeListenPort))
			if err != nil {
				return trace.Wrap(err)
			}
			cfg.Proxy.Kube.ListenAddr = *addr
		}
		if len(fc.Proxy.Kube.PublicAddr) != 0 {
			addrs, err := utils.AddrsFromStrings(fc.Proxy.Kube.PublicAddr, defaults.KubeListenPort)
			if err != nil {
				return trace.Wrap(err)
			}
			cfg.Proxy.Kube.PublicAddrs = addrs
		}
	case !legacyKube && newKube:
		// New kubernetes format (kubernetes_service +
		// proxy_service.kube_listen_addr) is only relevant in the config file
		// format. Under the hood, we use the same cfg.Proxy.Kube field to
		// enable it.
		if len(fc.Proxy.KubePublicAddr) > 0 && fc.Proxy.KubeAddr == "" {
			return trace.BadParameter("kube_listen_addr must be set when kube_public_addr is set")
		}
		cfg.Proxy.Kube.Enabled = true
		addr, err := utils.ParseHostPortAddr(fc.Proxy.KubeAddr, int(defaults.KubeListenPort))
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.Kube.ListenAddr = *addr

		publicAddrs, err := utils.AddrsFromStrings(fc.Proxy.KubePublicAddr, defaults.KubeListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.Kube.PublicAddrs = publicAddrs
	case legacyKube && newKube:
		return trace.BadParameter("proxy_service should either set kube_listen_addr/kube_public_addr or kubernetes.enabled, not both; keep kubernetes.enabled if you don't enable kubernetes_service, or keep kube_listen_addr otherwise")
	case !legacyKube && !newKube:
		if fc.Version != "" && fc.Version != defaults.TeleportConfigVersionV1 {
			cfg.Proxy.Kube.Enabled = true
		}
	}
	if len(fc.Proxy.PublicAddr) != 0 {
		addrs, err := utils.AddrsFromStrings(fc.Proxy.PublicAddr, cfg.Proxy.WebAddr.Port(defaults.HTTPListenPort))
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.PublicAddrs = addrs
	}
	if len(fc.Proxy.SSHPublicAddr) != 0 {
		addrs, err := utils.AddrsFromStrings(fc.Proxy.SSHPublicAddr, defaults.SSHProxyListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.SSHPublicAddrs = addrs
	}
	if len(fc.Proxy.TunnelPublicAddr) != 0 {
		addrs, err := utils.AddrsFromStrings(fc.Proxy.TunnelPublicAddr, defaults.SSHProxyTunnelListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.TunnelPublicAddrs = addrs
	}
	if len(fc.Proxy.PostgresPublicAddr) != 0 {
		defaultPort := getPostgresDefaultPort(cfg)
		addrs, err := utils.AddrsFromStrings(fc.Proxy.PostgresPublicAddr, defaultPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.PostgresPublicAddrs = addrs
	}

	if len(fc.Proxy.MySQLPublicAddr) != 0 {
		if fc.Proxy.MySQLAddr == "" {
			return trace.BadParameter("mysql_listen_addr must be set when mysql_public_addr is set")
		}
		// MySQL proxy is listening on a separate port.
		addrs, err := utils.AddrsFromStrings(fc.Proxy.MySQLPublicAddr, defaults.MySQLListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.MySQLPublicAddrs = addrs
	}

	if len(fc.Proxy.MongoPublicAddr) != 0 {
		if fc.Proxy.MongoAddr == "" {
			return trace.BadParameter("mongo_listen_addr must be set when mongo_public_addr is set")
		}
		addrs, err := utils.AddrsFromStrings(fc.Proxy.MongoPublicAddr, defaults.MongoListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.MongoPublicAddrs = addrs
	}
	if fc.Proxy.PeerPublicAddr != "" {
		if fc.Proxy.PeerAddr == "" {
			return trace.BadParameter("peer_listen_addr must be set when peer_public_addr is set")
		}
		addr, err := utils.ParseHostPortAddr(fc.Proxy.PeerPublicAddr, int(defaults.ProxyPeeringListenPort))
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.PeerPublicAddr = *addr
	}

	cfg.Proxy.IdP.SAMLIdP.Enabled = fc.Proxy.IdP.SAMLIdP.Enabled()
	cfg.Proxy.IdP.SAMLIdP.BaseURL = fc.Proxy.IdP.SAMLIdP.BaseURL

	acme, err := fc.Proxy.ACME.Parse()
	if err != nil {
		return trace.Wrap(err)
	}
	cfg.Proxy.ACME = *acme
	cfg.Proxy.TrustXForwardedFor = fc.Proxy.TrustXForwardedFor.Value()
	return nil
}

func getPostgresDefaultPort(cfg *servicecfg.Config) int {
	if !cfg.Proxy.PostgresAddr.IsEmpty() {
		// If the proxy.PostgresAddr flag was provided return port
		// from PostgresAddr address or default PostgresListenPort.
		return cfg.Proxy.PostgresAddr.Port(defaults.PostgresListenPort)
	}
	// Postgres proxy is multiplexed on the web proxy port. If the proxy is
	// not specified here explicitly, prefer defaults in the following
	// order, depending on what's set:
	//   1. Web proxy public port
	//   2. Web proxy listen port
	//   3. Web proxy default listen port
	if len(cfg.Proxy.PublicAddrs) != 0 {
		return cfg.Proxy.PublicAddrs[0].Port(defaults.HTTPListenPort)
	}
	return cfg.Proxy.WebAddr.Port(defaults.HTTPListenPort)
}

func applyDefaultProxyListenerAddresses(cfg *servicecfg.Config) {
	// From v2 onwards if an address is not provided don't fall back to the default values.
	if cfg.Version != "" && cfg.Version != defaults.TeleportConfigVersionV1 {
		return
	}

	// For v1 configuration check if address was set in config file if
	// not fallback to the default listener address.
	if cfg.Proxy.WebAddr.IsEmpty() {
		cfg.Proxy.WebAddr = *defaults.ProxyWebListenAddr()
	}
	if cfg.Proxy.SSHAddr.IsEmpty() {
		cfg.Proxy.SSHAddr = *defaults.ProxyListenAddr()
	}
	if cfg.Proxy.ReverseTunnelListenAddr.IsEmpty() {
		cfg.Proxy.ReverseTunnelListenAddr = *defaults.ReverseTunnelListenAddr()
	}
	if cfg.Proxy.Kube.ListenAddr.IsEmpty() && cfg.Proxy.Kube.Enabled {
		cfg.Proxy.Kube.ListenAddr = *defaults.KubeProxyListenAddr()
	}
}

// applySSHConfig applies file configuration for the "ssh_service" section.
func applySSHConfig(fc *FileConfig, cfg *servicecfg.Config) (err error) {
	if fc.SSH.ListenAddress != "" {
		addr, err := utils.ParseHostPortAddr(fc.SSH.ListenAddress, int(defaults.SSHServerListenPort))
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.SSH.Addr = *addr
	}
	if fc.SSH.Labels != nil {
		cfg.SSH.Labels = maps.Clone(fc.SSH.Labels)
	}
	if fc.SSH.Commands != nil {
		cfg.SSH.CmdLabels = make(services.CommandLabels)
		for _, cmdLabel := range fc.SSH.Commands {
			cfg.SSH.CmdLabels[cmdLabel.Name] = &types.CommandLabelV2{
				Period:  types.NewDuration(cmdLabel.Period),
				Command: cmdLabel.Command,
				Result:  "",
			}
		}
	}
	if fc.SSH.Namespace != "" {
		cfg.SSH.Namespace = fc.SSH.Namespace
	}
	if fc.SSH.PermitUserEnvironment {
		cfg.SSH.PermitUserEnvironment = true
	}
	if fc.SSH.DisableCreateHostUser || runtime.GOOS != constants.LinuxOS {
		cfg.SSH.DisableCreateHostUser = true
		if runtime.GOOS != constants.LinuxOS {
			log.Debugln("Disabling host user creation as this feature is only available on Linux")
		}
	}
	if fc.SSH.PAM != nil {
		cfg.SSH.PAM = fc.SSH.PAM.Parse()

		// If PAM is enabled, make sure that Teleport was built with PAM support
		// and the PAM library was found at runtime.
		if cfg.SSH.PAM.Enabled {
			if !pam.BuildHasPAM() {
				errorMessage := "Unable to start Teleport: PAM was enabled in file configuration but this \n" +
					"Teleport binary was built without PAM support. To continue either download a \n" +
					"Teleport binary build with PAM support from https://goteleport.com/teleport \n" +
					"or disable PAM in file configuration."
				return trace.BadParameter(errorMessage)
			}
			if !pam.SystemHasPAM() {
				errorMessage := "Unable to start Teleport: PAM was enabled in file configuration but this \n" +
					"system does not have the needed PAM library installed. To continue either \n" +
					"install libpam or disable PAM in file configuration."
				return trace.BadParameter(errorMessage)
			}
		}
	}
	if len(fc.SSH.PublicAddr) != 0 {
		addrs, err := utils.AddrsFromStrings(fc.SSH.PublicAddr, defaults.SSHServerListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.SSH.PublicAddrs = addrs
	}
	if fc.SSH.BPF != nil {
		cfg.SSH.BPF = fc.SSH.BPF.Parse()
	}
	if fc.SSH.RestrictedSession != nil {
		log.Error("Restricted Sessions for SSH were removed in Teleport 15.")
	}

	cfg.SSH.AllowTCPForwarding = fc.SSH.AllowTCPForwarding()

	cfg.SSH.X11, err = fc.SSH.X11ServerConfig()
	if err != nil {
		return trace.Wrap(err)
	}

	cfg.SSH.AllowFileCopying = fc.SSH.SSHFileCopy()

	return nil
}

// getInstallerProxyAddr determines the address of the proxy for discovered
// nodes to connect to.
func getInstallerProxyAddr(installParams *InstallParams, fc *FileConfig) string {
	// Explicit proxy address.
	if installParams != nil && installParams.PublicProxyAddr != "" {
		return installParams.PublicProxyAddr
	}
	// Proxy address from config.
	if fc.ProxyServer != "" {
		return fc.ProxyServer
	}
	if fc.Proxy.Enabled() && len(fc.Proxy.PublicAddr) > 0 {
		return fc.Proxy.PublicAddr[0]
	}
	// Possible proxy address for v1/v2 config.
	if len(fc.AuthServers) > 0 {
		return fc.AuthServers[0]
	}
	// Probably not a proxy address, but we have nothing better.
	return fc.AuthServer
}

func applyDiscoveryConfig(fc *FileConfig, cfg *servicecfg.Config) error {
	cfg.Discovery.Enabled = fc.Discovery.Enabled()
	cfg.Discovery.DiscoveryGroup = fc.Discovery.DiscoveryGroup
	cfg.Discovery.PollInterval = fc.Discovery.PollInterval
	for _, matcher := range fc.Discovery.AWSMatchers {
		var err error
		var installParams *types.InstallerParams
		if matcher.InstallParams != nil {
			installParams, err = matcher.InstallParams.parse()
			if err != nil {
				return trace.Wrap(err)
			}
		}

		var assumeRole *types.AssumeRole
		if matcher.AssumeRoleARN != "" || matcher.ExternalID != "" {
			assumeRole = &types.AssumeRole{
				RoleARN:    matcher.AssumeRoleARN,
				ExternalID: matcher.ExternalID,
			}
		}

		for _, region := range matcher.Regions {
			if !awsutils.IsKnownRegion(region) {
				log.Warnf("AWS matcher uses unknown region %q. "+
					"There could be a typo in %q. "+
					"Ignore this message if this is a new AWS region that is unknown to the AWS SDK used to compile this binary. "+
					"Known regions are: %v.",
					region, region, awsutils.GetKnownRegions(),
				)
			}
		}

		serviceMatcher := types.AWSMatcher{
			Types:      matcher.Types,
			Regions:    matcher.Regions,
			AssumeRole: assumeRole,
			Tags:       matcher.Tags,
			Params:     installParams,
			SSM:        &types.AWSSSM{DocumentName: matcher.SSM.DocumentName},
		}
		if err := serviceMatcher.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

		cfg.Discovery.AWSMatchers = append(cfg.Discovery.AWSMatchers, serviceMatcher)
	}

	for _, matcher := range fc.Discovery.AzureMatchers {
		var installerParams *types.InstallerParams
		if slices.Contains(matcher.Types, types.AzureMatcherVM) {
			installerParams = &types.InstallerParams{
				PublicProxyAddr: getInstallerProxyAddr(matcher.InstallParams, fc),
			}
			if matcher.InstallParams != nil {
				installerParams.JoinMethod = matcher.InstallParams.JoinParams.Method
				installerParams.JoinToken = matcher.InstallParams.JoinParams.TokenName
				installerParams.ScriptName = matcher.InstallParams.ScriptName
				if matcher.InstallParams.Azure != nil {
					installerParams.Azure = &types.AzureInstallerParams{
						ClientID: matcher.InstallParams.Azure.ClientID,
					}
				}
			}
		}

		serviceMatcher := types.AzureMatcher{
			Subscriptions:  matcher.Subscriptions,
			ResourceGroups: matcher.ResourceGroups,
			Types:          matcher.Types,
			Regions:        matcher.Regions,
			ResourceTags:   matcher.ResourceTags,
			Params:         installerParams,
		}
		if err := serviceMatcher.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

		cfg.Discovery.AzureMatchers = append(cfg.Discovery.AzureMatchers, serviceMatcher)
	}

	for _, matcher := range fc.Discovery.GCPMatchers {
		var installerParams *types.InstallerParams
		if slices.Contains(matcher.Types, types.GCPMatcherCompute) {
			installerParams = &types.InstallerParams{
				PublicProxyAddr: getInstallerProxyAddr(matcher.InstallParams, fc),
			}
			if matcher.InstallParams != nil {
				installerParams.JoinMethod = matcher.InstallParams.JoinParams.Method
				installerParams.JoinToken = matcher.InstallParams.JoinParams.TokenName
				installerParams.ScriptName = matcher.InstallParams.ScriptName
			}
		}

		serviceMatcher := types.GCPMatcher{
			Types:           matcher.Types,
			Locations:       matcher.Locations,
			Labels:          matcher.Labels,
			Tags:            matcher.Tags,
			ProjectIDs:      matcher.ProjectIDs,
			ServiceAccounts: matcher.ServiceAccounts,
			Params:          installerParams,
		}
		if err := serviceMatcher.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

		cfg.Discovery.GCPMatchers = append(cfg.Discovery.GCPMatchers, serviceMatcher)
	}

	if len(fc.Discovery.KubernetesMatchers) > 0 {
		if fc.Discovery.DiscoveryGroup == "" {
			// TODO(anton): add link to documentation when it's available
			return trace.BadParameter(`parameter 'discovery_group' should be defined for discovery service if
kubernetes matchers are present`)
		}
	}
	for _, matcher := range fc.Discovery.KubernetesMatchers {
		serviceMatcher := types.KubernetesMatcher{
			Types:      matcher.Types,
			Namespaces: matcher.Namespaces,
			Labels:     matcher.Labels,
		}
		if err := serviceMatcher.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

		cfg.Discovery.KubernetesMatchers = append(cfg.Discovery.KubernetesMatchers, serviceMatcher)
	}

	if fc.Discovery.AccessGraph != nil {
		var tMatcher types.AccessGraphSync
		for _, awsMatcher := range fc.Discovery.AccessGraph.AWS {
			regions := awsMatcher.Regions
			if len(regions) == 0 {
				return trace.BadParameter("missing regions in access_graph")
			}
			var assumeRole *types.AssumeRole
			if awsMatcher.AssumeRoleARN != "" || awsMatcher.ExternalID != "" {
				assumeRole = &types.AssumeRole{
					RoleARN:    awsMatcher.AssumeRoleARN,
					ExternalID: awsMatcher.ExternalID,
				}
			}
			tMatcher.AWS = append(tMatcher.AWS, &types.AccessGraphAWSSync{
				Regions:    regions,
				AssumeRole: assumeRole,
			})
		}
		cfg.Discovery.AccessGraph = &tMatcher
	}

	return nil
}

// applyKubeConfig applies file configuration for the "kubernetes_service" section.
func applyKubeConfig(fc *FileConfig, cfg *servicecfg.Config) error {
	if fc.Kube.ListenAddress != "" {
		addr, err := utils.ParseHostPortAddr(fc.Kube.ListenAddress, int(defaults.SSHProxyListenPort))
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Kube.ListenAddr = addr
	}
	if len(fc.Kube.PublicAddr) != 0 {
		addrs, err := utils.AddrsFromStrings(fc.Kube.PublicAddr, defaults.KubeListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Kube.PublicAddrs = addrs
	}

	for _, matcher := range fc.Kube.ResourceMatchers {
		cfg.Kube.ResourceMatchers = append(cfg.Kube.ResourceMatchers,
			services.ResourceMatcher{
				Labels: matcher.Labels,
				AWS: services.ResourceMatcherAWS{
					AssumeRoleARN: matcher.AWS.AssumeRoleARN,
					ExternalID:    matcher.AWS.ExternalID,
				},
			})
	}

	if fc.Kube.KubeconfigFile != "" {
		cfg.Kube.KubeconfigPath = fc.Kube.KubeconfigFile
	}
	if fc.Kube.KubeClusterName != "" {
		cfg.Kube.KubeClusterName = fc.Kube.KubeClusterName
	}
	if fc.Kube.StaticLabels != nil {
		cfg.Kube.StaticLabels = maps.Clone(fc.Kube.StaticLabels)
	}
	if fc.Kube.DynamicLabels != nil {
		cfg.Kube.DynamicLabels = make(services.CommandLabels)
		for _, cmdLabel := range fc.Kube.DynamicLabels {
			cfg.Kube.DynamicLabels[cmdLabel.Name] = &types.CommandLabelV2{
				Period:  types.NewDuration(cmdLabel.Period),
				Command: cmdLabel.Command,
				Result:  "",
			}
		}
	}

	return nil
}

// applyDatabasesConfig applies file configuration for the "db_service" section.
func applyDatabasesConfig(fc *FileConfig, cfg *servicecfg.Config) error {
	cfg.Databases.Enabled = true
	for _, matcher := range fc.Databases.ResourceMatchers {
		cfg.Databases.ResourceMatchers = append(cfg.Databases.ResourceMatchers,
			services.ResourceMatcher{
				Labels: matcher.Labels,
				AWS: services.ResourceMatcherAWS{
					AssumeRoleARN: matcher.AWS.AssumeRoleARN,
					ExternalID:    matcher.AWS.ExternalID,
				},
			})
	}
	for _, matcher := range fc.Databases.AWSMatchers {
		cfg.Databases.AWSMatchers = append(cfg.Databases.AWSMatchers,
			types.AWSMatcher{
				Types:   matcher.Types,
				Regions: matcher.Regions,
				Tags:    matcher.Tags,
				AssumeRole: &types.AssumeRole{
					RoleARN:    matcher.AssumeRoleARN,
					ExternalID: matcher.ExternalID,
				},
			})
	}
	for _, matcher := range fc.Databases.AzureMatchers {
		cfg.Databases.AzureMatchers = append(cfg.Databases.AzureMatchers,
			types.AzureMatcher{
				Subscriptions:  matcher.Subscriptions,
				ResourceGroups: matcher.ResourceGroups,
				Types:          matcher.Types,
				Regions:        matcher.Regions,
				ResourceTags:   matcher.ResourceTags,
			})
	}
	for _, database := range fc.Databases.Databases {
		staticLabels := make(map[string]string)
		if database.StaticLabels != nil {
			staticLabels = database.StaticLabels
		}
		dynamicLabels := make(services.CommandLabels)
		if database.DynamicLabels != nil {
			for _, v := range database.DynamicLabels {
				dynamicLabels[v.Name] = &types.CommandLabelV2{
					Period:  types.NewDuration(v.Period),
					Command: v.Command,
					Result:  "",
				}
			}
		}

		caBytes, err := readCACert(database)
		if err != nil {
			return trace.Wrap(err)
		}

		db := servicecfg.Database{
			Name:          database.Name,
			Description:   database.Description,
			Protocol:      database.Protocol,
			URI:           database.URI,
			StaticLabels:  staticLabels,
			DynamicLabels: dynamicLabels,
			MySQL: servicecfg.MySQLOptions{
				ServerVersion: database.MySQL.ServerVersion,
			},
			TLS: servicecfg.DatabaseTLS{
				CACert:     caBytes,
				ServerName: database.TLS.ServerName,
				Mode:       servicecfg.TLSMode(database.TLS.Mode),
			},
			AdminUser: servicecfg.DatabaseAdminUser{
				Name:            database.AdminUser.Name,
				DefaultDatabase: database.AdminUser.DefaultDatabase,
			},
			Oracle: convOracleOptions(database.Oracle),
			AWS: servicecfg.DatabaseAWS{
				AccountID:     database.AWS.AccountID,
				AssumeRoleARN: database.AWS.AssumeRoleARN,
				ExternalID:    database.AWS.ExternalID,
				Region:        database.AWS.Region,
				SessionTags:   database.AWS.SessionTags,
				Redshift: servicecfg.DatabaseAWSRedshift{
					ClusterID: database.AWS.Redshift.ClusterID,
				},
				RedshiftServerless: servicecfg.DatabaseAWSRedshiftServerless{
					WorkgroupName: database.AWS.RedshiftServerless.WorkgroupName,
					EndpointName:  database.AWS.RedshiftServerless.EndpointName,
				},
				RDS: servicecfg.DatabaseAWSRDS{
					InstanceID: database.AWS.RDS.InstanceID,
					ClusterID:  database.AWS.RDS.ClusterID,
				},
				ElastiCache: servicecfg.DatabaseAWSElastiCache{
					ReplicationGroupID: database.AWS.ElastiCache.ReplicationGroupID,
				},
				MemoryDB: servicecfg.DatabaseAWSMemoryDB{
					ClusterName: database.AWS.MemoryDB.ClusterName,
				},
				SecretStore: servicecfg.DatabaseAWSSecretStore{
					KeyPrefix: database.AWS.SecretStore.KeyPrefix,
					KMSKeyID:  database.AWS.SecretStore.KMSKeyID,
				},
			},
			GCP: servicecfg.DatabaseGCP{
				ProjectID:  database.GCP.ProjectID,
				InstanceID: database.GCP.InstanceID,
			},
			AD: servicecfg.DatabaseAD{
				KeytabFile:  database.AD.KeytabFile,
				Krb5File:    database.AD.Krb5File,
				Domain:      database.AD.Domain,
				SPN:         database.AD.SPN,
				LDAPCert:    database.AD.LDAPCert,
				KDCHostName: database.AD.KDCHostName,
			},
			Azure: servicecfg.DatabaseAzure{
				ResourceID:    database.Azure.ResourceID,
				IsFlexiServer: database.Azure.IsFlexiServer,
			},
		}
		if err := db.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		cfg.Databases.Databases = append(cfg.Databases.Databases, db)
	}
	return nil
}

func convOracleOptions(o DatabaseOracle) servicecfg.OracleOptions {
	return servicecfg.OracleOptions{
		AuditUser: o.AuditUser,
	}
}

// readCACert reads database CA certificate from the config file.
// First 'tls.ca_cert_file` is being read, then deprecated 'ca_cert_file' if
// the first one is not set.
func readCACert(database *Database) ([]byte, error) {
	var (
		caBytes []byte
		err     error
	)
	if database.TLS.CACertFile != "" {
		caBytes, err = os.ReadFile(database.TLS.CACertFile)
		if err != nil {
			return nil, trace.ConvertSystemError(err)
		}
	}

	// ca_cert_file is deprecated, but we still support it.
	// Print a warning if the old field is still being used.
	if database.CACertFile != "" {
		if database.TLS.CACertFile != "" {
			// New and old fields are set. Ignore the old field.
			log.Warnf("Ignoring deprecated ca_cert_file in %s configuration; using tls.ca_cert_file.", database.Name)
		} else {
			// Only old field is set, inform about deprecation.
			log.Warnf("ca_cert_file is deprecated, please use tls.ca_cert_file instead for %s.", database.Name)

			caBytes, err = os.ReadFile(database.CACertFile)
			if err != nil {
				return nil, trace.ConvertSystemError(err)
			}
		}
	}

	return caBytes, nil
}

// applyAppsConfig applies file configuration for the "app_service" section.
func applyAppsConfig(fc *FileConfig, cfg *servicecfg.Config) error {
	// Apps are enabled.
	cfg.Apps.Enabled = true

	// Enable debugging application if requested.
	cfg.Apps.DebugApp = fc.Apps.DebugApp

	// Configure resource watcher selectors if present.
	for _, matcher := range fc.Apps.ResourceMatchers {
		if matcher.AWS.AssumeRoleARN != "" {
			return trace.NotImplemented("assume_role_arn is not supported for app resource matchers")
		}
		cfg.Apps.ResourceMatchers = append(cfg.Apps.ResourceMatchers,
			services.ResourceMatcher{
				Labels: matcher.Labels,
			})
	}

	// Loop over all apps and load app configuration.
	for _, application := range fc.Apps.Apps {
		// Parse the static labels of the application.
		staticLabels := make(map[string]string)
		if application.StaticLabels != nil {
			staticLabels = application.StaticLabels
		}

		// Parse the dynamic labels of the application.
		dynamicLabels := make(services.CommandLabels)
		if application.DynamicLabels != nil {
			for _, v := range application.DynamicLabels {
				dynamicLabels[v.Name] = &types.CommandLabelV2{
					Period:  types.NewDuration(v.Period),
					Command: v.Command,
				}
			}
		}

		// Add the application to the list of proxied applications.
		app := servicecfg.App{
			Name:               application.Name,
			Description:        application.Description,
			URI:                application.URI,
			PublicAddr:         application.PublicAddr,
			StaticLabels:       staticLabels,
			DynamicLabels:      dynamicLabels,
			InsecureSkipVerify: application.InsecureSkipVerify,
			Cloud:              application.Cloud,
		}
		if application.Rewrite != nil {
			// Parse http rewrite headers if there are any.
			headers, err := servicecfg.ParseHeaders(application.Rewrite.Headers)
			if err != nil {
				return trace.Wrap(err, "failed to parse headers rewrite configuration for app %q",
					application.Name)
			}
			app.Rewrite = &servicecfg.Rewrite{
				Redirect:  application.Rewrite.Redirect,
				Headers:   headers,
				JWTClaims: application.Rewrite.JWTClaims,
			}
		}
		if application.AWS != nil {
			app.AWS = &servicecfg.AppAWS{
				ExternalID: application.AWS.ExternalID,
			}
		}
		if err := app.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		cfg.Apps.Apps = append(cfg.Apps.Apps, app)
	}

	return nil
}

// applyMetricsConfig applies file configuration for the "metrics_service" section.
func applyMetricsConfig(fc *FileConfig, cfg *servicecfg.Config) error {
	// Metrics is enabled.
	cfg.Metrics.Enabled = true

	addr, err := utils.ParseHostPortAddr(fc.Metrics.ListenAddress, int(defaults.MetricsListenPort))
	if err != nil {
		return trace.Wrap(err)
	}
	cfg.Metrics.ListenAddr = addr

	cfg.Metrics.GRPCServerLatency = fc.Metrics.GRPCServerLatency
	cfg.Metrics.GRPCClientLatency = fc.Metrics.GRPCClientLatency

	if !fc.Metrics.MTLSEnabled() {
		return nil
	}

	cfg.Metrics.MTLS = true

	if len(fc.Metrics.KeyPairs) == 0 {
		return trace.BadParameter("at least one keypair should be provided when mtls is enabled in the metrics config")
	}

	if len(fc.Metrics.CACerts) == 0 {
		return trace.BadParameter("at least one CA cert should be provided when mtls is enabled in the metrics config")
	}

	for _, p := range fc.Metrics.KeyPairs {
		// Check that the certificate exists on disk. This exists to provide the
		// user a sensible error message.
		if !utils.FileExists(p.PrivateKey) {
			return trace.NotFound("metrics service private key does not exist: %s", p.PrivateKey)
		}
		if !utils.FileExists(p.Certificate) {
			return trace.NotFound("metrics service cert does not exist: %s", p.Certificate)
		}

		certificateChain, err := utils.ReadCertificatesFromPath(p.Certificate)
		if err != nil {
			return trace.Wrap(err)
		}

		if !utils.IsSelfSigned(certificateChain) {
			if err := utils.VerifyCertificateChain(certificateChain); err != nil {
				return trace.BadParameter("unable to verify the metrics service certificate chain in %v: %s",
					p.Certificate, utils.UserMessageFromError(err))
			}
		}

		cfg.Metrics.KeyPairs = append(cfg.Metrics.KeyPairs, servicecfg.KeyPairPath{
			PrivateKey:  p.PrivateKey,
			Certificate: p.Certificate,
		})
	}

	for _, caCert := range fc.Metrics.CACerts {
		// Check that the certificate exists on disk. This exists to provide the
		// user a sensible error message.
		if !utils.FileExists(caCert) {
			return trace.NotFound("metrics service ca cert does not exist: %s", caCert)
		}

		cfg.Metrics.CACerts = append(cfg.Metrics.CACerts, caCert)
	}

	return nil
}

// applyWindowsDesktopConfig applies file configuration for the "windows_desktop_service" section.
func applyWindowsDesktopConfig(fc *FileConfig, cfg *servicecfg.Config) error {
	if err := fc.WindowsDesktop.Check(); err != nil {
		return trace.Wrap(err)
	}

	cfg.WindowsDesktop.Enabled = true

	if fc.WindowsDesktop.ListenAddress != "" {
		listenAddr, err := utils.ParseHostPortAddr(fc.WindowsDesktop.ListenAddress, int(defaults.WindowsDesktopListenPort))
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.WindowsDesktop.ListenAddr = *listenAddr
	}

	for _, filter := range fc.WindowsDesktop.Discovery.Filters {
		if _, err := ldap.CompileFilter(filter); err != nil {
			return trace.BadParameter("WindowsDesktopService specifies invalid LDAP filter %q", filter)
		}
	}

	for _, attributeName := range fc.WindowsDesktop.Discovery.LabelAttributes {
		if !types.IsValidLabelKey(attributeName) {
			return trace.BadParameter("WindowsDesktopService specifies label_attribute %q which is not a valid label key", attributeName)
		}
	}

	cfg.WindowsDesktop.Discovery = servicecfg.LDAPDiscoveryConfig{
		BaseDN:          fc.WindowsDesktop.Discovery.BaseDN,
		Filters:         fc.WindowsDesktop.Discovery.Filters,
		LabelAttributes: fc.WindowsDesktop.Discovery.LabelAttributes,
	}

	var err error
	cfg.WindowsDesktop.PublicAddrs, err = utils.AddrsFromStrings(fc.WindowsDesktop.PublicAddr, defaults.WindowsDesktopListenPort)
	if err != nil {
		return trace.Wrap(err)
	}
	cfg.WindowsDesktop.ShowDesktopWallpaper = fc.WindowsDesktop.ShowDesktopWallpaper
	if len(fc.WindowsDesktop.ADHosts) > 0 {
		log.Warnln("hosts field is deprecated, prefer static_hosts instead")
	}
	if len(fc.WindowsDesktop.NonADHosts) > 0 {
		log.Warnln("non_ad_hosts field is deprecated, prefer static_hosts instead")
	}
	cfg.WindowsDesktop.StaticHosts, err = staticHostsWithAddress(fc.WindowsDesktop)
	if err != nil {
		return trace.Wrap(err)
	}
	if fc.WindowsDesktop.LDAP.DEREncodedCAFile != "" && fc.WindowsDesktop.LDAP.PEMEncodedCACert != "" {
		return trace.BadParameter("WindowsDesktopService can not use both der_ca_file and ldap_ca_cert")
	}

	var cert *x509.Certificate
	if fc.WindowsDesktop.LDAP.DEREncodedCAFile != "" {
		rawCert, err := os.ReadFile(fc.WindowsDesktop.LDAP.DEREncodedCAFile)
		if err != nil {
			return trace.WrapWithMessage(err, "loading the LDAP CA from file %v", fc.WindowsDesktop.LDAP.DEREncodedCAFile)
		}

		cert, err = x509.ParseCertificate(rawCert)
		if err != nil {
			return trace.WrapWithMessage(err, "parsing the LDAP root CA file %v", fc.WindowsDesktop.LDAP.DEREncodedCAFile)
		}
	}

	if fc.WindowsDesktop.LDAP.PEMEncodedCACert != "" {
		cert, err = tlsca.ParseCertificatePEM([]byte(fc.WindowsDesktop.LDAP.PEMEncodedCACert))
		if err != nil {
			return trace.WrapWithMessage(err, "parsing the LDAP root CA PEM cert")
		}
	}

	cfg.WindowsDesktop.LDAP = servicecfg.LDAPConfig{
		Addr:               fc.WindowsDesktop.LDAP.Addr,
		Username:           fc.WindowsDesktop.LDAP.Username,
		SID:                fc.WindowsDesktop.LDAP.SID,
		Domain:             fc.WindowsDesktop.LDAP.Domain,
		InsecureSkipVerify: fc.WindowsDesktop.LDAP.InsecureSkipVerify,
		ServerName:         fc.WindowsDesktop.LDAP.ServerName,
		CA:                 cert,
	}

	cfg.WindowsDesktop.PKIDomain = fc.WindowsDesktop.PKIDomain

	var hlrs []servicecfg.HostLabelRule
	for _, rule := range fc.WindowsDesktop.HostLabels {
		r, err := regexp.Compile(rule.Match)
		if err != nil {
			return trace.BadParameter("WindowsDesktopService specifies invalid regexp %q", rule.Match)
		}

		if len(rule.Labels) == 0 {
			return trace.BadParameter("WindowsDesktopService host regex %q has no labels", rule.Match)
		}

		for k := range rule.Labels {
			if !types.IsValidLabelKey(k) {
				return trace.BadParameter("WindowsDesktopService specifies invalid label %q", k)
			}
		}

		hlrs = append(hlrs, servicecfg.HostLabelRule{
			Regexp: r,
			Labels: rule.Labels,
		})
	}
	cfg.WindowsDesktop.HostLabels = servicecfg.NewHostLabelRules(hlrs...)

	if fc.WindowsDesktop.Labels != nil {
		cfg.WindowsDesktop.Labels = maps.Clone(fc.WindowsDesktop.Labels)
	}

	return nil
}

func staticHostsWithAddress(ws WindowsDesktopService) ([]servicecfg.WindowsHost, error) {
	var hostsWithAddress []servicecfg.WindowsHost
	var cfgHosts []WindowsHost
	cfgHosts = append(cfgHosts, ws.StaticHosts...)
	for _, host := range ws.NonADHosts {
		cfgHosts = append(cfgHosts, WindowsHost{
			Address: host,
			AD:      false,
		})
	}
	for _, host := range ws.ADHosts {
		cfgHosts = append(cfgHosts, WindowsHost{
			Address: host,
			AD:      true,
		})
	}
	for _, host := range cfgHosts {
		addr, err := utils.ParseHostPortAddr(host.Address, defaults.RDPListenPort)
		if err != nil {
			return nil, trace.BadParameter("invalid addr %q", host.Address)
		}
		hostsWithAddress = append(hostsWithAddress, servicecfg.WindowsHost{
			Name:    host.Name,
			Address: *addr,
			Labels:  host.Labels,
			AD:      host.AD,
		})
	}
	return hostsWithAddress, nil
}

// applyTracingConfig applies file configuration for the "tracing_service" section.
func applyTracingConfig(fc *FileConfig, cfg *servicecfg.Config) error {
	// Tracing is enabled.
	cfg.Tracing.Enabled = true

	if fc.Tracing.ExporterURL == "" {
		return trace.BadParameter("tracing_service is enabled but no exporter_url is specified")
	}

	cfg.Tracing.ExporterURL = fc.Tracing.ExporterURL
	cfg.Tracing.SamplingRate = float64(fc.Tracing.SamplingRatePerMillion) / 1_000_000.0

	for _, p := range fc.Tracing.KeyPairs {
		// Check that the certificate exists on disk. This exists to provide the
		// user a sensible error message.
		if !utils.FileExists(p.PrivateKey) {
			return trace.NotFound("tracing_service private key does not exist: %s", p.PrivateKey)
		}
		if !utils.FileExists(p.Certificate) {
			return trace.NotFound("tracing_service cert does not exist: %s", p.Certificate)
		}

		cfg.Tracing.KeyPairs = append(cfg.Tracing.KeyPairs, servicecfg.KeyPairPath{
			PrivateKey:  p.PrivateKey,
			Certificate: p.Certificate,
		})
	}

	for _, caCert := range fc.Tracing.CACerts {
		// Check that the certificate exists on disk. This exists to provide the
		// user a sensible error message.
		if !utils.FileExists(caCert) {
			return trace.NotFound("tracing_service ca cert does not exist: %s", caCert)
		}

		cfg.Tracing.CACerts = append(cfg.Tracing.CACerts, caCert)
	}

	return nil
}

// applyString takes 'src' and overwrites target with it, unless 'src' is empty
// returns 'True' if 'src' was not empty
func applyString(src string, target *string) bool {
	if src != "" {
		*target = src
		return true
	}
	return false
}

// applyConfigVersion applies config version from parsed file. If config version is not
// present the v1 version will be used as default.
func applyConfigVersion(fc *FileConfig, cfg *servicecfg.Config) {
	cfg.Version = defaults.TeleportConfigVersionV1
	if fc.Version != "" {
		cfg.Version = fc.Version
	}
}

// Configure merges command line arguments with what's in a configuration file
// with CLI commands taking precedence
func Configure(clf *CommandLineFlags, cfg *servicecfg.Config, legacyAppFlags bool) error {
	// pass the value of --insecure flag to the runtime
	lib.SetInsecureDevMode(clf.InsecureMode)

	// load /etc/teleport.yaml and apply it's values:
	fileConf, err := ReadConfigFile(clf.ConfigFile)
	if err != nil {
		return trace.Wrap(err)
	}
	// if configuration is passed as an environment variable,
	// try to decode it and override the config file
	if clf.ConfigString != "" {
		fileConf, err = ReadFromString(clf.ConfigString)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if clf.BootstrapFile != "" {
		resources, err := ReadResources(clf.BootstrapFile)
		if err != nil {
			return trace.Wrap(err)
		}
		if len(resources) < 1 {
			return trace.BadParameter("no resources found: %q", clf.BootstrapFile)
		}
		cfg.Auth.BootstrapResources = resources
	}

	if clf.ApplyOnStartupFile != "" {
		resources, err := ReadResources(clf.ApplyOnStartupFile)
		if err != nil {
			return trace.Wrap(err)
		}
		if len(resources) < 1 {
			return trace.BadParameter("no resources found: %q", clf.ApplyOnStartupFile)
		}
		cfg.Auth.ApplyOnStartupResources = resources
	}

	// Apply command line --debug flag to override logger severity.
	if clf.Debug {
		// If debug logging is requested and no file configuration exists, set the
		// log level right away. Otherwise allow the command line flag to override
		// logger severity in file configuration.
		if fileConf == nil {
			cfg.SetLogLevel(slog.LevelDebug)
		} else {
			if strings.ToLower(fileConf.Logger.Severity) != "trace" {
				fileConf.Logger.Severity = teleport.DebugLevel
			}
		}
	}

	// If this process is trying to join a cluster as an application service,
	// make sure application name and URI are provided.
	if slices.Contains(splitRoles(clf.Roles), defaults.RoleApp) {
		if (clf.AppName == "") && (clf.AppURI == "" && clf.AppCloud == "") {
			// TODO: remove legacyAppFlags once `teleport start --app-name` is removed.
			if legacyAppFlags {
				return trace.BadParameter("application name (--app-name) and URI (--app-uri) flags are both required to join application proxy to the cluster")
			}
			return trace.BadParameter("to join application proxy to the cluster provide application name (--name) and either URI (--uri) or Cloud type (--cloud)")
		}

		if clf.AppName == "" {
			if legacyAppFlags {
				return trace.BadParameter("application name (--app-name) is required to join application proxy to the cluster")
			}
			return trace.BadParameter("to join application proxy to the cluster provide application name (--name)")
		}

		if clf.AppURI == "" && clf.AppCloud == "" {
			if legacyAppFlags {
				return trace.BadParameter("URI (--app-uri) flag is required to join application proxy to the cluster")
			}
			return trace.BadParameter("to join application proxy to the cluster provide URI (--uri) or Cloud type (--cloud)")
		}
	}

	// If application name was specified on command line, add to file
	// configuration where it will be validated.
	if clf.AppName != "" {
		cfg.Apps.Enabled = true

		// Parse static and dynamic labels.
		static, dynamic, err := parseLabels(clf.Labels)
		if err != nil {
			return trace.BadParameter("labels invalid: %v", err)
		}

		// Create and validate application. If valid, add to list of applications.
		app := servicecfg.App{
			Name:          clf.AppName,
			URI:           clf.AppURI,
			Cloud:         clf.AppCloud,
			PublicAddr:    clf.AppPublicAddr,
			StaticLabels:  static,
			DynamicLabels: dynamic,
		}
		if err := app.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		cfg.Apps.Apps = append(cfg.Apps.Apps, app)
	}

	// If database name was specified on the command line, add to configuration.
	if clf.DatabaseName != "" {
		cfg.Databases.Enabled = true
		staticLabels, dynamicLabels, err := parseLabels(clf.Labels)
		if err != nil {
			return trace.Wrap(err)
		}
		var caBytes []byte
		if clf.DatabaseCACertFile != "" {
			caBytes, err = os.ReadFile(clf.DatabaseCACertFile)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		var sessionTags map[string]string
		if clf.DatabaseAWSSessionTags != "" {
			var err error
			sessionTags, err = client.ParseLabelSpec(clf.DatabaseAWSSessionTags)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		db := servicecfg.Database{
			Name:         clf.DatabaseName,
			Description:  clf.DatabaseDescription,
			Protocol:     clf.DatabaseProtocol,
			URI:          clf.DatabaseURI,
			StaticLabels: staticLabels,
			MySQL: servicecfg.MySQLOptions{
				ServerVersion: clf.DatabaseMySQLServerVersion,
			},
			DynamicLabels: dynamicLabels,
			TLS: servicecfg.DatabaseTLS{
				CACert: caBytes,
			},
			AWS: servicecfg.DatabaseAWS{
				Region:        clf.DatabaseAWSRegion,
				AccountID:     clf.DatabaseAWSAccountID,
				AssumeRoleARN: clf.DatabaseAWSAssumeRoleARN,
				ExternalID:    clf.DatabaseAWSExternalID,
				SessionTags:   sessionTags,
				Redshift: servicecfg.DatabaseAWSRedshift{
					ClusterID: clf.DatabaseAWSRedshiftClusterID,
				},
				RDS: servicecfg.DatabaseAWSRDS{
					InstanceID: clf.DatabaseAWSRDSInstanceID,
					ClusterID:  clf.DatabaseAWSRDSClusterID,
				},
				ElastiCache: servicecfg.DatabaseAWSElastiCache{
					ReplicationGroupID: clf.DatabaseAWSElastiCacheGroupID,
				},
				MemoryDB: servicecfg.DatabaseAWSMemoryDB{
					ClusterName: clf.DatabaseAWSMemoryDBClusterName,
				},
			},
			GCP: servicecfg.DatabaseGCP{
				ProjectID:  clf.DatabaseGCPProjectID,
				InstanceID: clf.DatabaseGCPInstanceID,
			},
			AD: servicecfg.DatabaseAD{
				KeytabFile: clf.DatabaseADKeytabFile,
				Krb5File:   clf.DatabaseADKrb5File,
				Domain:     clf.DatabaseADDomain,
				SPN:        clf.DatabaseADSPN,
			},
		}
		if err := db.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		cfg.Databases.Databases = append(cfg.Databases.Databases, db)
	}

	if err = ApplyFileConfig(fileConf, cfg); err != nil {
		return trace.Wrap(err)
	}

	// If FIPS mode is specified, validate Teleport configuration is FedRAMP/FIPS
	// 140-2 compliant.
	if clf.FIPS {
		// Make sure all cryptographic primitives are FIPS compliant.
		err = utils.UintSliceSubset(defaults.FIPSCipherSuites, cfg.CipherSuites)
		if err != nil {
			return trace.BadParameter("non-FIPS compliant TLS cipher suite selected: %v", err)
		}
		err = utils.StringSliceSubset(defaults.FIPSCiphers, cfg.Ciphers)
		if err != nil {
			return trace.BadParameter("non-FIPS compliant SSH cipher selected: %v", err)
		}
		err = utils.StringSliceSubset(defaults.FIPSKEXAlgorithms, cfg.KEXAlgorithms)
		if err != nil {
			return trace.BadParameter("non-FIPS compliant SSH kex algorithm selected: %v", err)
		}
		err = utils.StringSliceSubset(defaults.FIPSMACAlgorithms, cfg.MACAlgorithms)
		if err != nil {
			return trace.BadParameter("non-FIPS compliant SSH mac algorithm selected: %v", err)
		}

		// Make sure cluster settings are also FedRAMP/FIPS 140-2 compliant.
		if cfg.Auth.Enabled {
			// Only SSO based authentication is supported. The SSO provider is where
			// any FedRAMP/FIPS 140-2 compliance (like password complexity) should be
			// enforced.
			if cfg.Auth.Preference.GetAllowLocalAuth() {
				return trace.BadParameter("non-FIPS compliant authentication setting: \"local_auth\" must be false")
			}

			// If sessions are being recorded at the proxy host key checking must be
			// enabled. This make sure the host certificate key algorithm is FIPS
			// compliant.
			if services.IsRecordAtProxy(cfg.Auth.SessionRecordingConfig.GetMode()) &&
				!cfg.Auth.SessionRecordingConfig.GetProxyChecksHostKeys() {
				return trace.BadParameter("non-FIPS compliant proxy settings: \"proxy_checks_host_keys\" must be true")
			}
		}
	}

	// apply --skip-version-check flag.
	if clf.SkipVersionCheck {
		cfg.SkipVersionCheck = clf.SkipVersionCheck
	}

	// Apply diagnostic address flag.
	if clf.DiagnosticAddr != "" {
		addr, err := utils.ParseAddr(clf.DiagnosticAddr)
		if err != nil {
			return trace.Wrap(err, "failed to parse diag-addr")
		}
		cfg.DiagnosticAddr = *addr
	}

	// apply --insecure-no-tls flag:
	if clf.DisableTLS {
		cfg.Proxy.DisableTLS = clf.DisableTLS
	}

	// apply --debug flag to config:
	if clf.Debug {
		cfg.Console = io.Discard
		cfg.Debug = clf.Debug
	}

	// apply --roles flag:
	if clf.Roles != "" {
		if err := validateRoles(clf.Roles); err != nil {
			return trace.Wrap(err)
		}
		cfg.SSH.Enabled = strings.Contains(clf.Roles, defaults.RoleNode)
		cfg.Auth.Enabled = strings.Contains(clf.Roles, defaults.RoleAuthService)
		cfg.Proxy.Enabled = strings.Contains(clf.Roles, defaults.RoleProxy)
		cfg.Apps.Enabled = strings.Contains(clf.Roles, defaults.RoleApp)
		cfg.Databases.Enabled = strings.Contains(clf.Roles, defaults.RoleDatabase)
	}

	// apply --auth-server flag:
	if len(clf.AuthServerAddr) > 0 {
		if cfg.Auth.Enabled {
			log.Warnf("not starting the local auth service. --auth-server flag tells to connect to another auth server")
			cfg.Auth.Enabled = false
		}

		authServerAddresses := make([]utils.NetAddr, 0, len(clf.AuthServerAddr))
		for _, as := range clf.AuthServerAddr {
			addr, err := utils.ParseHostPortAddr(as, defaults.AuthListenPort)
			if err != nil {
				return trace.BadParameter("cannot parse auth server address: '%v'", as)
			}
			authServerAddresses = append(authServerAddresses, *addr)
		}

		if err := cfg.SetAuthServerAddresses(authServerAddresses); err != nil {
			return trace.Wrap(err)
		}
	}

	// apply --name flag:
	if clf.NodeName != "" {
		cfg.Hostname = clf.NodeName
	}

	// apply --pid-file flag
	if clf.PIDFile != "" {
		cfg.PIDFile = clf.PIDFile
	}

	if clf.AuthToken != "" {
		// store the value of the --token flag:
		cfg.SetToken(clf.AuthToken)
	}

	// Apply flags used for the node to validate the Auth Server.
	if err = cfg.ApplyCAPins(clf.CAPins); err != nil {
		return trace.Wrap(err)
	}

	// apply --listen-ip flag:
	if clf.ListenIP != nil {
		applyListenIP(clf.ListenIP, cfg)
	}

	// --advertise-ip flag
	if clf.AdvertiseIP != "" {
		if _, _, err := utils.ParseAdvertiseAddr(clf.AdvertiseIP); err != nil {
			return trace.Wrap(err)
		}
		cfg.AdvertiseIP = clf.AdvertiseIP
	}

	// apply --labels flag
	if err := parseLabelsApply(clf.Labels, &cfg.SSH); err != nil {
		return trace.Wrap(err)
	}

	// --pid-file:
	if clf.PIDFile != "" {
		cfg.PIDFile = clf.PIDFile
	}

	// auth_servers not configured, but the 'auth' is enabled (auth is on localhost)?
	if len(cfg.AuthServerAddresses()) == 0 && cfg.Auth.Enabled {
		cfg.SetAuthServerAddress(cfg.Auth.ListenAddr)
	}

	// add data_dir to the backend config:
	if cfg.Auth.StorageConfig.Params == nil {
		cfg.Auth.StorageConfig.Params = backend.Params{}
	}
	cfg.Auth.StorageConfig.Params["data_dir"] = cfg.DataDir
	// command line flag takes precedence over file config
	if clf.PermitUserEnvironment {
		cfg.SSH.PermitUserEnvironment = true
	}

	// set the default proxy listener addresses for config v1, if not already set
	applyDefaultProxyListenerAddresses(cfg)

	// not publicly documented or supported for now (thus the
	// "TELEPORT_UNSTABLE_" prefix); the group with an empty ID is a valid
	// group, and generation zero is a valid generation for any group
	cfg.Proxy.ProxyGroupID = os.Getenv("TELEPORT_UNSTABLE_PROXYGROUP_ID")
	if proxyGroupGeneration := os.Getenv("TELEPORT_UNSTABLE_PROXYGROUP_GEN"); proxyGroupGeneration != "" {
		cfg.Proxy.ProxyGroupGeneration, err = strconv.ParseUint(proxyGroupGeneration, 10, 64)
		if err != nil {
			return trace.Wrap(err, "invalid proxygroup generation %q: %v", proxyGroupGeneration, err)
		}
	}

	return nil
}

// ConfigureOpenSSH initializes a config from the commandline flags passed
func ConfigureOpenSSH(clf *CommandLineFlags, cfg *servicecfg.Config) error {
	// pass the value of --insecure flag to the runtime
	lib.SetInsecureDevMode(clf.InsecureMode)

	// Apply command line --debug flag to override logger severity.
	if clf.Debug {
		cfg.SetLogLevel(slog.LevelDebug)
		cfg.Debug = clf.Debug
	}

	if clf.AuthToken != "" {
		// store the value of the --token flag:
		cfg.SetToken(clf.AuthToken)
	}

	log.Debugf("Disabling all services, only the Teleport OpenSSH service can run during the `teleport join openssh` command")
	servicecfg.DisableLongRunningServices(cfg)

	cfg.DataDir = clf.DataDir
	cfg.Version = defaults.TeleportConfigVersionV3
	cfg.OpenSSH.SSHDConfigPath = clf.OpenSSHConfigPath
	cfg.OpenSSH.RestartSSHD = clf.RestartOpenSSH
	cfg.OpenSSH.RestartCommand = clf.RestartCommand
	cfg.OpenSSH.CheckCommand = clf.CheckCommand
	cfg.JoinMethod = types.JoinMethod(clf.JoinMethod)

	hostname, err := os.Hostname()
	if err != nil {
		return trace.Wrap(err)
	}

	cfg.Hostname = hostname
	cfg.OpenSSH.InstanceAddr = clf.Address
	cfg.OpenSSH.AdditionalPrincipals = []string{hostname, clf.Address}
	for _, principal := range strings.Split(clf.AdditionalPrincipals, ",") {
		if principal == "" {
			continue
		}
		cfg.OpenSSH.AdditionalPrincipals = append(cfg.OpenSSH.AdditionalPrincipals, principal)
	}
	cfg.OpenSSH.Labels, err = client.ParseLabelSpec(clf.Labels)
	if err != nil {
		return trace.Wrap(err)
	}

	proxyServer, err := utils.ParseAddr(clf.ProxyServer)
	if err != nil {
		return trace.Wrap(err)
	}
	cfg.SetAuthServerAddresses(nil)
	cfg.ProxyServer = *proxyServer

	return nil
}

// parseLabels parses the labels command line flag and returns static and
// dynamic labels.
func parseLabels(spec string) (map[string]string, services.CommandLabels, error) {
	// Base syntax parsing, the spec must be in the form of 'key=value,more="better"'.
	lmap, err := client.ParseLabelSpec(spec)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	static := make(map[string]string)
	dynamic := make(services.CommandLabels)

	if len(lmap) == 0 {
		return static, dynamic, nil
	}

	// Loop over all parsed labels and set either static or dynamic labels.
	for key, value := range lmap {
		dynamicLabel, err := isCmdLabelSpec(value)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		if dynamicLabel != nil {
			dynamic[key] = dynamicLabel
		} else {
			static[key] = value
		}
	}

	return static, dynamic, nil
}

// parseLabelsApply reads in the labels command line flag and tries to
// correctly populate static and dynamic labels for the SSH service.
func parseLabelsApply(spec string, sshConf *servicecfg.SSHConfig) error {
	if spec == "" {
		return nil
	}

	var err error
	sshConf.Labels, sshConf.CmdLabels, err = parseLabels(spec)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// isCmdLabelSpec tries to interpret a given string as a "command label" spec.
// A command label spec looks like [time_duration:command param1 param2 ...] where
// time_duration is in "1h2m1s" form.
//
// Example of a valid spec: "[1h:/bin/uname -m]"
func isCmdLabelSpec(spec string) (types.CommandLabel, error) {
	// command spec? (surrounded by brackets?)
	if len(spec) > 5 && spec[0] == '[' && spec[len(spec)-1] == ']' {
		invalidSpecError := trace.BadParameter(
			"invalid command label spec: '%s'", spec)
		spec = strings.Trim(spec, "[]")
		idx := strings.IndexRune(spec, ':')
		if idx < 0 {
			return nil, trace.Wrap(invalidSpecError)
		}
		periodSpec := spec[:idx]
		period, err := time.ParseDuration(periodSpec)
		if err != nil {
			return nil, trace.Wrap(invalidSpecError)
		}
		cmdSpec := spec[idx+1:]
		if len(cmdSpec) < 1 {
			return nil, trace.Wrap(invalidSpecError)
		}
		openQuote := false
		return &types.CommandLabelV2{
			Period: types.NewDuration(period),
			Command: strings.FieldsFunc(cmdSpec, func(c rune) bool {
				if c == '"' {
					openQuote = !openQuote
				}
				return unicode.IsSpace(c) && !openQuote
			}),
		}, nil
	}
	// not a valid spec
	return nil, nil
}

// applyListenIP replaces all 'listen addr' settings for all services with
// a given IP
func applyListenIP(ip net.IP, cfg *servicecfg.Config) {
	listeningAddresses := []*utils.NetAddr{
		&cfg.Auth.ListenAddr,
		&cfg.Auth.ListenAddr,
		&cfg.Proxy.SSHAddr,
		&cfg.Proxy.WebAddr,
		&cfg.SSH.Addr,
		&cfg.Proxy.ReverseTunnelListenAddr,
	}
	for _, addr := range listeningAddresses {
		replaceHost(addr, ip.String())
	}
}

// replaceHost takes utils.NetAddr and replaces the hostname in it, preserving
// the original port
func replaceHost(addr *utils.NetAddr, newHost string) {
	_, port, err := net.SplitHostPort(addr.Addr)
	if err != nil {
		log.Errorf("failed parsing address: '%v'", addr.Addr)
	}
	addr.Addr = net.JoinHostPort(newHost, port)
}

// validateRoles makes sure that value passed to the --roles flag is valid
func validateRoles(roles string) error {
	for _, role := range splitRoles(roles) {
		switch role {
		case defaults.RoleAuthService,
			defaults.RoleNode,
			defaults.RoleProxy,
			defaults.RoleApp,
			defaults.RoleDatabase,
			defaults.RoleWindowsDesktop,
			defaults.RoleDiscovery:
		default:
			return trace.Errorf("unknown role: '%s'", role)
		}
	}
	return nil
}

// splitRoles splits in the format roles expects.
func splitRoles(roles string) []string {
	return strings.Split(roles, ",")
}

// applyTokenConfig applies the auth_token and join_params to the config
func applyTokenConfig(fc *FileConfig, cfg *servicecfg.Config) error {
	if fc.AuthToken != "" {
		if fc.JoinParams != (JoinParams{}) {
			return trace.BadParameter("only one of auth_token or join_params should be set")
		}

		cfg.JoinMethod = types.JoinMethodToken
		cfg.SetToken(fc.AuthToken)

		return nil
	}

	if fc.JoinParams != (JoinParams{}) {
		cfg.SetToken(fc.JoinParams.TokenName)

		if err := types.ValidateJoinMethod(fc.JoinParams.Method); err != nil {
			return trace.Wrap(err)
		}

		cfg.JoinMethod = fc.JoinParams.Method

		if fc.JoinParams.Azure != (AzureJoinParams{}) {
			cfg.JoinParams = servicecfg.JoinParams{
				Azure: servicecfg.AzureJoinParams{
					ClientID: fc.JoinParams.Azure.ClientID,
				},
			}
		}
	}

	return nil
}

func applyOktaConfig(fc *FileConfig, cfg *servicecfg.Config) error {
	if fc.Okta.APIEndpoint == "" {
		return trace.BadParameter("okta_service is enabled but no api_endpoint is specified")
	}
	if fc.Okta.APITokenPath == "" {
		return trace.BadParameter("okta_service is enabled but no api_token_path is specified")
	}

	// Make sure the URL is valid
	url, err := url.Parse(fc.Okta.APIEndpoint)
	if err != nil {
		return trace.NewAggregate(trace.BadParameter("malformed URL %s", fc.Okta.APIEndpoint), err)
	}

	if url.Host == "" {
		return trace.BadParameter("api_endpoint has no host")
	} else if url.Scheme == "" {
		return trace.BadParameter("api_endpoint has no scheme")
	}

	// Make sure the API token exists.
	if _, err := utils.StatFile(fc.Okta.APITokenPath); err != nil {
		return trace.NewAggregate(trace.BadParameter("error trying to find file %s", fc.Okta.APITokenPath), err)
	}

	syncSettings, err := fc.Okta.Sync.Parse()
	if err != nil {
		return trace.Wrap(err)
	}

	// For backwards compatibility, if SyncPeriod is specified, use that in the sync settings.
	if syncSettings.AppGroupSyncPeriod == 0 {
		syncSettings.AppGroupSyncPeriod = fc.Okta.SyncPeriod
	}

	cfg.Okta.Enabled = fc.Okta.Enabled()
	cfg.Okta.APIEndpoint = fc.Okta.APIEndpoint
	cfg.Okta.APITokenPath = fc.Okta.APITokenPath
	cfg.Okta.SyncPeriod = fc.Okta.SyncPeriod
	cfg.Okta.SyncSettings = *syncSettings
	return nil
}

func applyJamfConfig(fc *FileConfig, cfg *servicecfg.Config) error {
	// Ignore empty configs, validate and transform anything else.
	if reflect.DeepEqual(fc.Jamf, JamfService{}) {
		return nil
	}

	jamfSpec, err := fc.Jamf.toJamfSpecV1()
	if err != nil {
		return trace.Wrap(err)
	}

	cfg.Jamf = servicecfg.JamfConfig{
		Spec:       jamfSpec,
		ExitOnSync: fc.Jamf.ExitOnSync,
	}
	return nil
}
