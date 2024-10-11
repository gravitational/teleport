/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package autoupdate

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/renameio/v2"
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/api/client/webclient"
	libdefaults "github.com/gravitational/teleport/lib/defaults"
	libutils "github.com/gravitational/teleport/lib/utils"
)

const (
	// cdnURITemplate is the default template for the Teleport tgz download.
	cdnURITemplate = "https://cdn.teleport.dev/teleport-v{{.Version}}-{{.OS}}-{{.Arch}}-bin.tar.gz"
	// configFileName specifies the name of the file inside versionsDirName containing configuration for the teleport update
	configFileName = "update.yaml"
)

const (
	agentUpdateConfigVersion = "v1"
	agentUpdateConfigKind    = "update_config"
)

// AgentUpdateConfig describes the update.yaml file schema.
type AgentUpdateConfig struct {
	// Version of the configuration file
	Version string `yaml:"version"`
	// Kind of configuration file (always "update_config")
	Kind string `yaml:"kind"`
	// Spec contains user-specified configuration.
	Spec AgentUpdateSpec `yaml:"spec"`
	// Status contains state configuration.
	Status AgentUpdateStatus `yaml:"status"`
}

// AgentUpdateSpec describes the spec field in update.yaml.
type AgentUpdateSpec struct {
	// Proxy address
	Proxy string `yaml:"proxy"`
	// Group update identifier
	Group string `yaml:"group"`
	// URLTemplate for the Teleport tgz download URL.
	URLTemplate string `yaml:"url_template"`
	// Enabled controls whether auto-updates are enabled.
	Enabled bool `yaml:"enabled"`
}

// AgentUpdateStatus describes the status field in update.yaml.
type AgentUpdateStatus struct {
	// ActiveVersion is the currently active Teleport version.
	ActiveVersion string `yaml:"active_version"`
}

func NewAgentUpdater(cfg AgentConfig) (*AgentUpdater, error) {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tr, err := libdefaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tr.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: cfg.DownloadInsecure,
		RootCAs:            certPool,
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   cfg.DownloadTimeout,
	}
	return &AgentUpdater{
		Log:         slog.Default(),
		HTTP:        client,
		Pool:        certPool,
		VersionsDir: cfg.VersionsDir,
		Installer: &AgentInstaller{
			VersionsDir:    cfg.VersionsDir,
			DownloadClient: client,
		},
	}, nil
}

type AgentUpdater struct {
	Log         *slog.Logger
	HTTP        *http.Client
	Pool        *x509.CertPool
	VersionsDir string
	Installer   TeleportInstaller
}

type AgentConfig struct {
	// DownloadInsecure turns off TLS certificate verification for download requests when enabled.
	DownloadInsecure bool
	// DownloadTimeout is a timeout for file download requests.
	DownloadTimeout time.Duration
	// VersionsDir for installing Teleport (usually /var/lib/teleport/versions)
	VersionsDir string
}

type TeleportInstaller interface {
	Install(ctx context.Context, version, template string) error
	Remove(ctx context.Context, version string) error
}

type AgentUserConfig struct {
	// Proxy address, scheme and port optional.
	// Overrides existing value if specified.
	Proxy string
	// Group identifier for updates (e.g., staging)
	// Overrides existing value if specified.
	Group string
	// URLTemplate for the Teleport tgz download URL
	// Overrides existing value if specified.
	URLTemplate string
	// ForceVersion to the specified version.
	ForceVersion string
}

// Enable enables agent updates.
func (u *AgentUpdater) Enable(ctx context.Context, userCfg AgentUserConfig) error {
	// Read configuration from updates.yaml and override any new values passed as flags.
	updatePath := filepath.Join(u.VersionsDir, configFileName)
	cfg, err := u.readConfig(updatePath)
	if err != nil {
		return trace.Wrap(err)
	}
	if userCfg.Proxy != "" {
		cfg.Spec.Proxy = userCfg.Proxy
	}
	if userCfg.Group != "" {
		cfg.Spec.Group = userCfg.Group
	}
	if userCfg.URLTemplate != "" {
		cfg.Spec.URLTemplate = userCfg.URLTemplate
	}
	cfg.Spec.Enabled = true
	if err := validateUpdatesSpec(&cfg.Spec); err != nil {
		return trace.Wrap(err)
	}

	// Lookup target version from the proxy.
	addr, err := libutils.ParseAddr(cfg.Spec.Proxy)
	if err != nil {
		return trace.Errorf("failed to parse proxy server address: %w", err)
	}

	desiredVersion := userCfg.ForceVersion
	if desiredVersion == "" {
		resp, err := webclient.Find(&webclient.Config{
			Context:   ctx,
			ProxyAddr: addr.Addr,
			Timeout:   30 * time.Second,
			//Group:     cfg.Spec.Group, // TODO(sclevine): add web API
			Pool: u.Pool,
		})
		if err != nil {
			return trace.Errorf("failed to request version from proxy: %w", err)
		}
		desiredVersion, _ = "16.3.0", resp // TODO(sclevine): add web API
		//desiredVersion := resp.AgentVersion
	}

	// If the active version and target don't match, kick off upgrade.
	if cfg.Status.ActiveVersion != desiredVersion {
		template := cfg.Spec.URLTemplate
		if template == "" {
			template = cdnURITemplate
		}
		// Create /var/lib/teleport/versions/X.Y.Z if it does not exist.
		err = u.Installer.Install(ctx, desiredVersion, template)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Status.ActiveVersion = desiredVersion
	}

	// Always write the configuration file if enable succeeds.
	if err := u.writeConfig(updatePath, cfg); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func validateUpdatesSpec(spec *AgentUpdateSpec) error {
	if spec.Proxy == "" {
		return trace.Errorf("proxy URL must be specified with --proxy or present in updates.yaml")
	}
	return nil
}

// Disable disables agent updates.
func (u *AgentUpdater) Disable(ctx context.Context) error {
	updatePath := filepath.Join(u.VersionsDir, configFileName)
	cfg, err := u.readConfig(updatePath)
	if err != nil {
		return trace.Errorf("failed to read updates.yaml: %w", err)
	}
	if !cfg.Spec.Enabled {
		u.Log.InfoContext(ctx, "Automatic updates already disabled")
		return nil
	}
	cfg.Spec.Enabled = false
	if err := u.writeConfig(updatePath, cfg); err != nil {
		return trace.Errorf("failed to write updates.yaml: %w", err)
	}
	return nil
}

// readConfig reads update.yaml
func (*AgentUpdater) readConfig(path string) (*AgentUpdateConfig, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &AgentUpdateConfig{
			Version: agentUpdateConfigVersion,
			Kind:    agentUpdateConfigKind,
		}, nil
	}
	if err != nil {
		return nil, trace.Errorf("failed to open: %w", err)
	}
	defer f.Close()
	var cfg AgentUpdateConfig
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, trace.Errorf("failed to parse: %w", err)
	}
	if k := cfg.Kind; k != agentUpdateConfigKind {
		return nil, trace.Errorf("invalid kind %q", k)
	}
	if v := cfg.Version; v != agentUpdateConfigVersion {
		return nil, trace.Errorf("invalid version %q", v)
	}
	return &cfg, nil
}

// writeConfig writes update.yaml atomically, ensuring the file cannot be corrupted.
func (*AgentUpdater) writeConfig(filename string, cfg *AgentUpdateConfig) error {
	opts := []renameio.Option{
		renameio.WithPermissions(0755),
		renameio.WithExistingPermissions(),
	}
	t, err := renameio.NewPendingFile(filename, opts...)
	if err != nil {
		return trace.Wrap(err)
	}
	defer t.Cleanup()
	err = yaml.NewEncoder(t).Encode(cfg)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(t.CloseAtomicallyReplace())
}
