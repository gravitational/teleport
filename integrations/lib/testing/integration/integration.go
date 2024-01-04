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

package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/go-version"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/integrations/lib/tctl"
	"github.com/gravitational/teleport/integrations/lib/tsh"
)

const IntegrationAdminRole = "integration-admin"
const DefaultLicensePath = "/var/lib/teleport/license.pem"

var regexpVersion = regexp.MustCompile(`^Teleport( Enterprise)? ([^ ]+)`)

type Integration struct {
	mu    sync.Mutex
	paths struct {
		BinPaths
		license string
	}
	workDir string
	cleanup []func() error
	version Version
	token   string
	caPin   string
}

type BinPaths struct {
	Teleport string
	Tctl     string
	Tsh      string
}

type Addr struct {
	Host string
	Port string
}

type Auth interface {
	AuthAddr() Addr
}

type Service interface {
	Run(context.Context) error
	WaitReady(ctx context.Context) (bool, error)
	Err() error
	Shutdown(context.Context) error
}

type Version struct {
	*version.Version
	IsEnterprise bool
}

type SignTLSPaths struct {
	CertPath   string
	KeyPath    string
	RootCAPath string
}

const serviceShutdownTimeout = 10 * time.Second

func requireBinaryVersion(ctx context.Context, path string, targetVersion *version.Version) error {
	v, err := getBinaryVersion(ctx, path)
	if err != nil {
		return trace.Wrap(err, "failed to get %s version", filepath.Base(path))
	}
	if !targetVersion.Equal(v.Version) {
		return trace.Errorf("%s version %s does not match target version %s", filepath.Base(path), v.Version, targetVersion)
	}

	return nil
}

// New initializes a Teleport installation.
func New(ctx context.Context, paths BinPaths, licenseStr string) (*Integration, error) {
	var err error
	log := logger.Get(ctx)

	var integration Integration
	integration.paths.BinPaths = paths
	initialized := false
	defer func() {
		if !initialized {
			integration.Close()
		}
	}()

	log.Debug("Creating test working dir")
	integration.workDir, err = os.MkdirTemp("", "teleport-plugins-integration-*")
	if err != nil {
		return nil, trace.Wrap(err, "failed to initialize work directory")
	}
	integration.registerCleanup(func() error { return os.RemoveAll(integration.workDir) })
	log.Debugf("Test working dir is %s", integration.workDir)

	teleportVersion, err := getBinaryVersion(ctx, integration.paths.Teleport)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get teleport version")
	}

	if err = requireBinaryVersion(ctx, integration.paths.Tctl, teleportVersion.Version); err != nil {
		return nil, trace.Wrap(err, "tctl version check")
	}

	if err = requireBinaryVersion(ctx, integration.paths.Tsh, teleportVersion.Version); err != nil {
		return nil, trace.Wrap(err, "tsh version check")
	}

	if teleportVersion.IsEnterprise {
		if licenseStr == "" {
			return nil, trace.Errorf("%s appears to be an Enterprise binary but license path is not specified", integration.paths.Teleport)
		}
		if strings.HasPrefix(licenseStr, "-----BEGIN CERTIFICATE-----") || strings.Contains(licenseStr, "\n") {
			// If it looks like a license file content lets write it to temporary file.
			log.Debug("License is given as a string, writing it to a file")
			licenseFile, err := integration.tempFile("license-*.pem")
			if err != nil {
				return nil, trace.Wrap(err, "failed to write license file")
			}
			if _, err := licenseFile.WriteString(licenseStr); err != nil {
				return nil, trace.Wrap(err, "failed to write license file")
			}
			if err := licenseFile.Close(); err != nil {
				return nil, trace.Wrap(err, "failed to write license file")
			}
			integration.paths.license = licenseFile.Name()
		} else if licenseStr != "" {
			integration.paths.license = licenseStr
			if !fileExists(integration.paths.license) {
				return nil, trace.NotFound("license file not found")
			}
		}
	}

	integration.version = teleportVersion

	tokenBytes := make([]byte, 16)
	_, err = rand.Read(tokenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	integration.token = hex.EncodeToString(tokenBytes)

	initialized = true
	return &integration, nil
}

// NewFromEnv initializes Teleport installation reading binary paths from environment variables such as
// TELEPORT_BINARY, TELEPORT_BINARY_TCTL or just PATH.
func NewFromEnv(ctx context.Context) (*Integration, error) {
	var err error

	licenseStr, ok := os.LookupEnv("TELEPORT_ENTERPRISE_LICENSE")
	if !ok && fileExists(DefaultLicensePath) {
		licenseStr = DefaultLicensePath
	}

	var paths BinPaths

	if version := os.Getenv("TELEPORT_GET_VERSION"); version == "" {
		paths = BinPaths{
			Teleport: os.Getenv("TELEPORT_BINARY"),
			Tctl:     os.Getenv("TELEPORT_BINARY_TCTL"),
			Tsh:      os.Getenv("TELEPORT_BINARY_TSH"),
		}

		// Look up binaries either in file system or in PATH.

		if paths.Teleport == "" {
			paths.Teleport = "teleport"
		}
		if paths.Teleport, err = exec.LookPath(paths.Teleport); err != nil {
			return nil, trace.Wrap(err)
		}

		if paths.Tctl == "" {
			paths.Tctl = "tctl"
		}
		if paths.Tctl, err = exec.LookPath(paths.Tctl); err != nil {
			return nil, trace.Wrap(err)
		}

		if paths.Tsh == "" {
			paths.Tsh = "tsh"
		}
		if paths.Tsh, err = exec.LookPath(paths.Tsh); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		_, goFile, _, ok := runtime.Caller(0)
		if !ok {
			return nil, trace.Errorf("failed to get caller information")
		}
		// Use GHA temp directory by default
		outDir := os.Getenv("RUNNER_TEMP")
		if outDir == "" {
			outDir = path.Join(path.Dir(goFile), "..", "..", "..") // gravitational/teleport repo root
		}
		outDir = path.Join(outDir, ".teleport")
		if licenseStr != "" {
			paths, err = GetEnterprise(ctx, version, outDir)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		} else {
			paths, err = GetOSS(ctx, version, outDir)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}

	return New(ctx, paths, licenseStr)
}

// Close stops all the spawned processes and does a cleanup.
func (integration *Integration) Close() {
	integration.mu.Lock()
	cleanup := integration.cleanup
	integration.cleanup = nil
	integration.mu.Unlock()

	for idx := range cleanup {
		if err := cleanup[len(cleanup)-idx-1](); err != nil {
			logger.Standard().WithError(trace.Wrap(err)).Error("Cleanup operation failed")
		}
	}
}

// Version returns an auth server version.
func (integration *Integration) Version() Version {
	return integration.version
}

type AuthServiceOption func(yaml string) string

func WithCache() AuthServiceOption {
	return func(yaml string) string {
		return strings.ReplaceAll(yaml, "{{TELEPORT_CACHE_ENABLED}}", "true")
	}
}

// NewAuthService creates a new auth server instance.
func (integration *Integration) NewAuthService(opts ...AuthServiceOption) (*AuthService, error) {
	dataDir, err := integration.tempDir("data-auth-*")
	if err != nil {
		return nil, trace.Wrap(err, "failed to initialize data directory")
	}

	configFile, err := integration.tempFile("teleport-auth-*.yaml")
	if err != nil {
		return nil, trace.Wrap(err, "failed to write config file")
	}

	yaml := teleportAuthYAML
	for _, o := range opts {
		yaml = o(yaml)
	}

	yaml = strings.ReplaceAll(yaml, "{{TELEPORT_DATA_DIR}}", dataDir)
	yaml = strings.ReplaceAll(yaml, "{{TELEPORT_CACHE_ENABLED}}", "false")
	yaml = strings.ReplaceAll(yaml, "{{TELEPORT_LICENSE_FILE}}", integration.paths.license)
	yaml = strings.ReplaceAll(yaml, "{{TELEPORT_AUTH_TOKEN}}", integration.token)
	if _, err := configFile.WriteString(yaml); err != nil {
		return nil, trace.Wrap(err, "failed to write config file")
	}
	if err := configFile.Close(); err != nil {
		return nil, trace.Wrap(err, "failed to write config file")
	}

	auth := newAuthService(integration.paths.Teleport, configFile.Name())
	integration.registerService(auth)

	return auth, nil
}

// NewProxyService creates a new auth server instance.
func (integration *Integration) NewProxyService(auth Auth) (*ProxyService, error) {
	dataDir, err := integration.tempDir("data-proxy-*")
	if err != nil {
		return nil, trace.Wrap(err, "failed to initialize data directory")
	}

	configFile, err := integration.tempFile("teleport-proxy-*.yaml")
	if err != nil {
		return nil, trace.Wrap(err, "failed to write config file")
	}

	yaml := strings.ReplaceAll(teleportProxyYAML, "{{TELEPORT_DATA_DIR}}", dataDir)
	yaml = strings.ReplaceAll(yaml, "{{TELEPORT_AUTH_SERVER}}", auth.AuthAddr().String())
	yaml = strings.ReplaceAll(yaml, "{{TELEPORT_AUTH_TOKEN}}", integration.token)
	yaml = strings.ReplaceAll(yaml, "{{TELEPORT_AUTH_CA_PIN}}", integration.caPin)
	webListenAddr, err := getFreeTCPPort()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	yaml = strings.ReplaceAll(yaml, "{{PROXY_WEB_LISTEN_ADDR}}", webListenAddr.String())
	yaml = strings.ReplaceAll(yaml, "{{PROXY_WEB_LISTEN_PORT}}", webListenAddr.Port)
	tunListenAddr, err := getFreeTCPPort()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	yaml = strings.ReplaceAll(yaml, "{{PROXY_TUN_LISTEN_ADDR}}", tunListenAddr.String())
	yaml = strings.ReplaceAll(yaml, "{{PROXY_TUN_LISTEN_PORT}}", tunListenAddr.Port)

	if _, err := configFile.WriteString(yaml); err != nil {
		return nil, trace.Wrap(err, "failed to write config file")
	}
	if err := configFile.Close(); err != nil {
		return nil, trace.Wrap(err, "failed to write config file")
	}

	proxy := newProxyService(integration.paths.Teleport, configFile.Name())
	integration.registerService(proxy)
	return proxy, nil
}

// NewSSHService creates a new auth server instance.
func (integration *Integration) NewSSHService(auth Auth) (*SSHService, error) {
	dataDir, err := integration.tempDir("data-ssh-*")
	if err != nil {
		return nil, trace.Wrap(err, "failed to initialize data directory")
	}

	configFile, err := integration.tempFile("teleport-ssh-*.yaml")
	if err != nil {
		return nil, trace.Wrap(err, "failed to write config file")
	}
	yaml := strings.ReplaceAll(teleportSSHYAML, "{{TELEPORT_DATA_DIR}}", dataDir)
	yaml = strings.ReplaceAll(yaml, "{{TELEPORT_AUTH_SERVER}}", auth.AuthAddr().String())
	yaml = strings.ReplaceAll(yaml, "{{TELEPORT_AUTH_TOKEN}}", integration.token)
	yaml = strings.ReplaceAll(yaml, "{{TELEPORT_AUTH_CA_PIN}}", integration.caPin)
	sshListenAddr, err := getFreeTCPPort()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	yaml = strings.ReplaceAll(yaml, "{{SSH_LISTEN_ADDR}}", sshListenAddr.String())
	yaml = strings.ReplaceAll(yaml, "{{SSH_LISTEN_PORT}}", sshListenAddr.Port)

	if _, err := configFile.WriteString(yaml); err != nil {
		return nil, trace.Wrap(err, "failed to write config file")
	}
	if err := configFile.Close(); err != nil {
		return nil, trace.Wrap(err, "failed to write config file")
	}

	ssh := newSSHService(integration.paths.Teleport, configFile.Name())
	integration.registerService(ssh)
	return ssh, nil
}

func (integration *Integration) Bootstrap(ctx context.Context, auth *AuthService, resources []types.Resource) error {
	return integration.tctl(auth).Create(ctx, resources)
}

// NewClient builds an API client for a given user.
func (integration *Integration) NewClient(ctx context.Context, auth *AuthService, userName string) (*Client, error) {
	outPath, err := integration.Sign(ctx, auth, userName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return integration.NewSignedClient(ctx, auth, outPath, userName)
}

// NewSignedClient builds a client for a given user given the identity file.
func (integration *Integration) NewSignedClient(ctx context.Context, auth Auth, identityPath, userName string) (*Client, error) {
	apiClient, err := client.New(ctx, client.Config{
		InsecureAddressDiscovery: true,
		Addrs:                    []string{auth.AuthAddr().String()},
		Credentials:              []client.Credentials{client.LoadIdentityFile(identityPath)},
		DialOpts: []grpc.DialOption{
			grpc.WithReturnConnectionError(),
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := &Client{Client: apiClient}
	integration.registerCleanup(client.Close)
	return client, nil
}

func (integration *Integration) MakeAdmin(ctx context.Context, auth *AuthService, userName string) (*Client, error) {
	var bootstrap Bootstrap
	if _, err := bootstrap.AddRole(IntegrationAdminRole, types.RoleSpecV6{
		Allow: types.RoleConditions{
			NodeLabels: types.Labels{types.Wildcard: utils.Strings{types.Wildcard}},
			Rules: []types.Rule{
				{
					Resources: []string{"*"},
					Verbs:     []string{"*"},
				},
			},
		},
	}); err != nil {
		return nil, trace.Wrap(err, fmt.Sprintf("failed to initialize %s role", IntegrationAdminRole))
	}
	if _, err := bootstrap.AddUserWithRoles(userName, IntegrationAdminRole); err != nil {
		return nil, trace.Wrap(err, fmt.Sprintf("failed to initialize %s user", userName))
	}
	if err := integration.Bootstrap(ctx, auth, bootstrap.Resources()); err != nil {
		return nil, trace.Wrap(err, fmt.Sprintf("failed to bootstrap admin user %s", userName))
	}
	return integration.NewClient(ctx, auth, userName)
}

// Sign generates a credentials file for the user and returns an identity file path.
func (integration *Integration) Sign(ctx context.Context, auth *AuthService, userName string) (string, error) {
	outFile, err := integration.tempFile(fmt.Sprintf("credentials-%s-*", userName))
	if err != nil {
		return "", trace.Wrap(err)
	}
	if err := outFile.Close(); err != nil {
		return "", trace.Wrap(err)
	}
	outPath := outFile.Name()
	if err := integration.tctl(auth).Sign(ctx, userName, "file", outPath); err != nil {
		return "", trace.Wrap(err)
	}
	return outPath, nil
}

// SignTLS generates a set of files to be used for generating the TLS Config: Cert, Key and RootCAs
func (integration *Integration) SignTLS(ctx context.Context, auth *AuthService, userName string) (*SignTLSPaths, error) {
	outFile, err := integration.tempFile(fmt.Sprintf("credentials-%s-*", userName))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := outFile.Close(); err != nil {
		return nil, trace.Wrap(err)
	}
	outPath := outFile.Name()
	if err := integration.tctl(auth).Sign(ctx, userName, "tls", outPath); err != nil {
		return nil, trace.Wrap(err)
	}

	return &SignTLSPaths{
		CertPath:   outPath + ".crt",
		KeyPath:    outPath + ".key",
		RootCAPath: outPath + ".cas",
	}, nil
}

// SetCAPin sets integration with the auth service's CA Pin.
func (integration *Integration) SetCAPin(ctx context.Context, auth *AuthService) error {
	if integration.caPin != "" {
		return nil
	}

	if ready, err := auth.WaitReady(ctx); err != nil {
		return trace.Wrap(err)
	} else if !ready {
		return trace.Wrap(auth.Err())
	}

	caPin, err := integration.tctl(auth).GetCAPin(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	integration.caPin = caPin
	return nil
}

// NewTsh makes a new tsh runner.
func (integration *Integration) NewTsh(proxyAddr, identityPath string) tsh.Tsh {
	return tsh.Tsh{
		Path:     integration.paths.Tsh,
		Proxy:    proxyAddr,
		Identity: identityPath,
		Insecure: true,
	}
}

func getBinaryVersion(ctx context.Context, binaryPath string) (Version, error) {
	cmd := exec.CommandContext(ctx, binaryPath, "version")
	logger.Get(ctx).Debugf("Running %s", cmd)
	out, err := cmd.Output()
	if err != nil {
		return Version{}, trace.Wrap(err)
	}
	submatch := regexpVersion.FindStringSubmatch(string(out))
	if submatch == nil {
		return Version{}, trace.Wrap(err)
	}

	version, err := version.NewVersion(submatch[2])
	if err != nil {
		return Version{}, trace.Wrap(err)
	}

	return Version{Version: version, IsEnterprise: submatch[1] != ""}, nil
}

func (integration *Integration) tctl(auth *AuthService) tctl.Tctl {
	return tctl.Tctl{
		Path:       integration.paths.Tctl,
		AuthServer: auth.AuthAddr().String(),
		ConfigPath: auth.ConfigPath(),
	}
}

func (integration *Integration) registerCleanup(fn func() error) {
	integration.mu.Lock()
	defer integration.mu.Unlock()
	integration.cleanup = append(integration.cleanup, fn)
}

func (integration *Integration) registerService(service Service) {
	integration.registerCleanup(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), serviceShutdownTimeout+10*time.Millisecond)
		defer cancel()
		return service.Shutdown(ctx)
	})
}

func (integration *Integration) tempFile(pattern string) (*os.File, error) {
	file, err := os.CreateTemp(integration.workDir, pattern)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	integration.registerCleanup(func() error { return os.Remove(file.Name()) })
	return file, trace.Wrap(err)
}

func (integration *Integration) tempDir(pattern string) (string, error) {
	dir, err := os.MkdirTemp(integration.workDir, pattern)
	if err != nil {
		return "", trace.Wrap(err)
	}
	integration.registerCleanup(func() error { return os.RemoveAll(dir) })
	return dir, nil
}

func getFreeTCPPort() (Addr, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return Addr{}, trace.Wrap(err)
	}
	if err := listener.Close(); err != nil {
		return Addr{}, trace.Wrap(err)
	}
	addrStr := listener.Addr().String()
	parts := strings.SplitN(addrStr, ":", 2)
	return Addr{Host: parts[0], Port: parts[1]}, nil
}

func (addr Addr) IsEmpty() bool {
	return addr.Host == "" && addr.Port == ""
}

func (addr Addr) String() string {
	return fmt.Sprintf("%s:%s", addr.Host, addr.Port)
}
