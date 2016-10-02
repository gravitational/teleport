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

package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend/etcdbk"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v2"
)

// Config structure is used to initialize _all_ services Teleporot can run.
// Some settings are globl (like DataDir) while others are grouped into
// sections, like AuthConfig
type Config struct {
	// DataDir provides directory where teleport stores it's permanent state
	// (in case of auth server backed by BoltDB) or local state, e.g. keys
	DataDir string

	// Hostname is a node host name
	Hostname string

	// Token is used to register this Teleport instance with the auth server
	Token string

	// AuthServers is a list of auth servers nodes, proxies and peer auth servers
	// connect to
	AuthServers []utils.NetAddr

	// Identities is an optional list of pre-generated key pairs
	// for teleport roles, this is helpful when server is preconfigured
	Identities []*auth.Identity

	// AdvertiseIP is used to "publish" an alternative IP address this node
	// can be reached on, if running behind NAT
	AdvertiseIP net.IP

	// SSH role an SSH endpoint server
	SSH SSHConfig

	// Auth server authentication and authorizatin server config
	Auth AuthConfig

	// Keygen points to a key generator implementation
	Keygen auth.Authority

	// Proxy is SSH proxy that manages incoming and outbound connections
	// via multiple reverse tunnels
	Proxy ProxyConfig

	// HostUUID is a unique UUID of this host (it will be known via this UUID within
	// a teleport cluster). It's automatically generated on 1st start
	HostUUID string

	// Console writer to speak to a user
	Console io.Writer

	// ReverseTunnels is a list of reverse tunnels to create on the
	// first cluster start
	ReverseTunnels []services.ReverseTunnel

	// OIDCConnectors is a list of trusted OpenID Connect identity providers
	OIDCConnectors []services.OIDCConnector

	// PidFile is a full path of the PID file for teleport daemon
	PIDFile string

	// Trust is a service that manages users and credentials
	Trust services.Trust

	// Lock is a distributed or local lock service
	Lock services.Lock

	// Presence service is a discovery and hearbeat tracker
	Presence services.Presence

	// Provisioner is a service that keeps track of provisioning tokens
	Provisioner services.Provisioner

	// Trust is a service that manages users and credentials
	Identity services.Identity

	// SeedConfig tells teleport to treat its start-up configuration as initial
	// "seed" configuration on 1st start.
	SeedConfig bool
}

// ApplyToken assigns a given token to all internal services but only if token
// is not an empty string.
//
// Returns 'true' if token was modified
func (cfg *Config) ApplyToken(token string) bool {
	if token != "" {
		cfg.Token = token
		return true
	}
	return false
}

// ConfigureBolt configures Bolt back-ends with a data dir.
func (cfg *Config) ConfigureBolt() {
	a := &cfg.Auth

	if a.EventsBackend.Type == teleport.BoltBackendType {
		a.EventsBackend.Params = boltParams(cfg.DataDir, defaults.EventsBoltFile)
	}
	if a.KeysBackend.Type == teleport.BoltBackendType {
		a.KeysBackend.Params = boltParams(cfg.DataDir, defaults.KeysBoltFile)
	}
	if a.RecordsBackend.Type == teleport.BoltBackendType {
		a.RecordsBackend.Params = boltParams(cfg.DataDir, defaults.RecordsBoltFile)
	}
}

// ConfigureETCD configures ETCD backend (still uses BoltDB for some cases)
func (cfg *Config) ConfigureETCD(etcdCfg etcdbk.Config) error {
	a := &cfg.Auth

	params, err := etcdParams(etcdCfg)
	if err != nil {
		return trace.Wrap(err)
	}
	a.KeysBackend.Type = teleport.ETCDBackendType
	a.KeysBackend.Params = params

	// We can't store records and events in ETCD
	a.EventsBackend.Type = teleport.BoltBackendType
	a.EventsBackend.Params = boltParams(cfg.DataDir, defaults.EventsBoltFile)

	a.RecordsBackend.Type = teleport.BoltBackendType
	a.RecordsBackend.Params = boltParams(cfg.DataDir, defaults.RecordsBoltFile)
	return nil
}

// RoleConfig is a config for particular Teleport role
func (cfg *Config) RoleConfig() RoleConfig {
	return RoleConfig{
		DataDir:     cfg.DataDir,
		HostUUID:    cfg.HostUUID,
		HostName:    cfg.Hostname,
		AuthServers: cfg.AuthServers,
		Auth:        cfg.Auth,
		Console:     cfg.Console,
	}
}

// DebugDumpToYAML is useful for debugging: it dumps the Config structure into
// a string
func (cfg *Config) DebugDumpToYAML() string {
	shallow := *cfg
	// do not copy sensitive data to stdout
	shallow.Identities = nil
	shallow.Auth.Authorities = nil
	out, err := yaml.Marshal(shallow)
	if err != nil {
		return err.Error()
	}
	return string(out)
}

// ProxyConfig configures proy service
type ProxyConfig struct {
	// Enabled turns proxy role on or off for this process
	Enabled bool

	// DisableWebUI allows to turn off serving the Web UI
	DisableWebUI bool

	// ReverseTunnelListenAddr is address where reverse tunnel dialers connect to
	ReverseTunnelListenAddr utils.NetAddr

	// WebAddr is address for web portal of the proxy
	WebAddr utils.NetAddr

	// SSHAddr is address of ssh proxy
	SSHAddr utils.NetAddr

	// TLSKey is a base64 encoded private key used by web portal
	TLSKey string

	// TLSCert is a base64 encoded certificate used by web portal
	TLSCert string

	Limiter limiter.LimiterConfig
}

// AuthConfig is a configuration of the auth server
type AuthConfig struct {
	// Enabled turns auth role on or off for this process
	Enabled bool

	// SSHAddr is the listening address of SSH tunnel to HTTP service
	SSHAddr utils.NetAddr

	// Authorities is a set of trusted certificate authorities
	// that will be added by this auth server on the first start
	Authorities []services.CertAuthority

	// DomainName is a name that identifies this authority and all
	// host nodes in the cluster that will share this authority domain name
	// as a base name, e.g. if authority domain name is example.com,
	// all nodes in the cluster will have UUIDs in the form: <uuid>.example.com
	DomainName string

	// StaticTokens are pre-defined host provisioning tokens supplied via config file for
	// environments where paranoid security is not needed
	StaticTokens []services.ProvisionToken

	// KeysBackend configures backend that stores auth keys, certificates, tokens ...
	KeysBackend struct {
		// Type is a backend type - etcd or boltdb
		Type string
		// Params is map with backend specific parameters
		Params string
	}

	// EventsBackend configures backend that stores cluster events (login attempts, etc)
	EventsBackend struct {
		// Type is a backend type, etcd or bolt
		Type string
		// Params is map with backend specific parameters
		Params string
	}

	// RecordsBackend configures backend that stores live SSH sessions recordings
	RecordsBackend struct {
		// Type is a backend type, currently only bolt
		Type string
		// Params is map with backend specific parameters
		Params string
	}

	Limiter limiter.LimiterConfig

	// NoAudit, when set to true, disables session recording and event audit
	NoAudit bool
}

// SSHConfig configures SSH server node role
type SSHConfig struct {
	Enabled   bool
	Addr      utils.NetAddr
	Shell     string
	Limiter   limiter.LimiterConfig
	Labels    map[string]string
	CmdLabels services.CommandLabels
}

// MakeDefaultConfig creates a new Config structure and populates it with defaults
func MakeDefaultConfig() (config *Config) {
	config = &Config{}
	ApplyDefaults(config)
	return config
}

// ApplyDefaults applies default values to the existing config structure
func ApplyDefaults(cfg *Config) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
		log.Errorf("Failed to determine hostname: %v", err)
	}
	cfg.SeedConfig = false

	// defaults for the auth service:
	cfg.Auth.Enabled = true
	cfg.Auth.SSHAddr = *defaults.AuthListenAddr()
	cfg.Auth.EventsBackend.Type = defaults.BackendType
	cfg.Auth.EventsBackend.Params = boltParams(defaults.DataDir, defaults.EventsBoltFile)
	cfg.Auth.KeysBackend.Type = defaults.BackendType
	cfg.Auth.KeysBackend.Params = boltParams(defaults.DataDir, defaults.KeysBoltFile)
	cfg.Auth.RecordsBackend.Type = defaults.BackendType
	cfg.Auth.RecordsBackend.Params = boltParams(defaults.DataDir, defaults.RecordsBoltFile)
	defaults.ConfigureLimiter(&cfg.Auth.Limiter)

	// defaults for the SSH proxy service:
	cfg.Proxy.Enabled = true
	cfg.Proxy.SSHAddr = *defaults.ProxyListenAddr()
	cfg.Proxy.WebAddr = *defaults.ProxyWebListenAddr()
	cfg.Proxy.ReverseTunnelListenAddr = *defaults.ReverseTunnellListenAddr()
	defaults.ConfigureLimiter(&cfg.Proxy.Limiter)

	// defaults for the SSH service:
	cfg.SSH.Enabled = true
	cfg.SSH.Addr = *defaults.SSHServerListenAddr()
	cfg.SSH.Shell = defaults.DefaultShell
	defaults.ConfigureLimiter(&cfg.SSH.Limiter)

	// global defaults
	cfg.Hostname = hostname
	cfg.DataDir = defaults.DataDir
	cfg.Console = os.Stdout
}

// Generates a string accepted by the BoltDB driver, like this:
// `{"path": "/var/lib/teleport/records.db"}`
func boltParams(storagePath, dbFile string) string {
	return fmt.Sprintf(`{"path": "%s"}`, filepath.Join(storagePath, dbFile))
}

// etcdParams generates a string accepted by the ETCD driver, like this:
func etcdParams(cfg etcdbk.Config) (string, error) {
	out, err := json.Marshal(cfg)
	if err != nil { // don't know what to do seriously
		return "", trace.Wrap(err)
	}
	return string(out), nil
}
