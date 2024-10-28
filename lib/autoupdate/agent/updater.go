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

package agent

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
	"strings"
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
	cdnURITemplate = "https://cdn.teleport.dev/teleport{{if .Enterprise}}-ent{{end}}-v{{.Version}}-{{.OS}}-{{.Arch}}{{if .FIPS}}-fips{{end}}-bin.tar.gz"
	// reservedFreeDisk is the minimum required free space left on disk during downloads.
	// TODO(sclevine): This value is arbitrary and could be replaced by, e.g., min(1%, 200mb) in the future
	//   to account for a range of disk sizes.
	reservedFreeDisk = 10_000_000 // 10 MB
)

const (
	// updateConfigName specifies the name of the file inside versionsDirName containing configuration for the teleport update.
	updateConfigName = "update.yaml"

	// UpdateConfig metadata
	updateConfigVersion = "v1"
	updateConfigKind    = "update_config"
)

// UpdateConfig describes the update.yaml file schema.
type UpdateConfig struct {
	// Version of the configuration file
	Version string `yaml:"version"`
	// Kind of configuration file (always "update_config")
	Kind string `yaml:"kind"`
	// Spec contains user-specified configuration.
	Spec UpdateSpec `yaml:"spec"`
	// Status contains state configuration.
	Status UpdateStatus `yaml:"status"`
}

// UpdateSpec describes the spec field in update.yaml.
type UpdateSpec struct {
	// Proxy address
	Proxy string `yaml:"proxy"`
	// Group specifies the update group identifier for the agent.
	Group string `yaml:"group"`
	// URLTemplate for the Teleport tgz download URL.
	URLTemplate string `yaml:"url_template"`
	// Enabled controls whether auto-updates are enabled.
	Enabled bool `yaml:"enabled"`
}

// UpdateStatus describes the status field in update.yaml.
type UpdateStatus struct {
	// ActiveVersion is the currently active Teleport version.
	ActiveVersion string `yaml:"active_version"`
	// BackupVersion is the last working version of Teleport.
	BackupVersion string `yaml:"backup_version"`
}

// NewLocalUpdater returns a new Updater that auto-updates local
// installations of the Teleport agent.
// The AutoUpdater uses an HTTP client with sane defaults for downloads, and
// will not fill disk to within 10 MB of available capacity.
func NewLocalUpdater(cfg LocalUpdaterConfig) (*Updater, error) {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tr, err := libdefaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tr.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		RootCAs:            certPool,
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   cfg.DownloadTimeout,
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	if cfg.LinkDir == "" {
		cfg.LinkDir = "/usr/local"
	}
	if cfg.VersionsDir == "" {
		cfg.VersionsDir = filepath.Join(libdefaults.DataDir, "versions")
	}
	return &Updater{
		Log:                cfg.Log,
		Pool:               certPool,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		ConfigPath:         filepath.Join(cfg.VersionsDir, updateConfigName),
		Installer: &LocalInstaller{
			InstallDir:     cfg.VersionsDir,
			LinkBinDir:     filepath.Join(cfg.LinkDir, "bin"),
			LinkServiceDir: filepath.Join(cfg.LinkDir, "lib", "systemd", "system"),
			HTTP:           client,
			Log:            cfg.Log,

			ReservedFreeTmpDisk:     reservedFreeDisk,
			ReservedFreeInstallDisk: reservedFreeDisk,
		},
	}, nil
}

// LocalUpdaterConfig specifies configuration for managing local agent auto-updates.
type LocalUpdaterConfig struct {
	// Log contains a slog logger.
	// Defaults to slog.Default() if nil.
	Log *slog.Logger
	// InsecureSkipVerify turns off TLS certificate verification.
	InsecureSkipVerify bool
	// DownloadTimeout is a timeout for file download requests.
	// Defaults to no timeout.
	DownloadTimeout time.Duration
	// VersionsDir for installing Teleport (usually /var/lib/teleport/versions).
	VersionsDir string
	// LinkDir for installing Teleport (usually /usr/local).
	LinkDir string
}

// Updater implements the agent-local logic for Teleport agent auto-updates.
type Updater struct {
	// Log contains a logger.
	Log *slog.Logger
	// Pool used for requests to the Teleport web API.
	Pool *x509.CertPool
	// InsecureSkipVerify skips TLS verification.
	InsecureSkipVerify bool
	// ConfigPath contains the path to the agent auto-updates configuration.
	ConfigPath string
	// Installer manages installations of the Teleport agent.
	Installer Installer
}

// Installer provides an API for installing Teleport agents.
type Installer interface {
	// Install the Teleport agent at version from the download template.
	// This function must be idempotent.
	Install(ctx context.Context, version, template string, flags InstallFlags) error
	// Link the Teleport agent at version into the system location.
	// This function must be idempotent.
	Link(ctx context.Context, version string) error
	// List the installed versions of Teleport.
	List(ctx context.Context) (versions []string, err error)
	// Remove the Teleport agent at version.
	// This function must be idempotent.
	Remove(ctx context.Context, version string) error
}

// InstallFlags sets flags for the Teleport installation
type InstallFlags int

const (
	// FlagEnterprise installs enterprise Teleport
	FlagEnterprise InstallFlags = 1 << iota
	// FlagFIPS installs FIPS Teleport
	FlagFIPS
)

// OverrideConfig contains overrides for individual update operations.
// If validated, these overrides may be persisted to disk.
type OverrideConfig struct {
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

// Enable enables agent updates and attempts an initial update.
// If the initial update succeeds, auto-updates are enabled and the configuration is persisted.
// Otherwise, the auto-updates configuration is not changed.
// This function is idempotent.
func (u *Updater) Enable(ctx context.Context, override OverrideConfig) error {
	// Read configuration from update.yaml and override any new values passed as flags.
	cfg, err := u.readConfig(u.ConfigPath)
	if err != nil {
		return trace.Errorf("failed to read %s: %w", updateConfigName, err)
	}
	if override.Proxy != "" {
		cfg.Spec.Proxy = override.Proxy
	}
	if override.Group != "" {
		cfg.Spec.Group = override.Group
	}
	if override.URLTemplate != "" {
		cfg.Spec.URLTemplate = override.URLTemplate
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

	desiredVersion := override.ForceVersion
	if desiredVersion == "" {
		resp, err := webclient.Find(&webclient.Config{
			Context:   ctx,
			ProxyAddr: addr.Addr,
			Insecure:  u.InsecureSkipVerify,
			Timeout:   30 * time.Second,
			//Group:     cfg.Spec.Group, // TODO(sclevine): add web API for verssion
			Pool: u.Pool,
		})
		if err != nil {
			return trace.Errorf("failed to request version from proxy: %w", err)
		}
		desiredVersion, _ = "16.3.0", resp // TODO(sclevine): add web API for version
		//desiredVersion := resp.AutoUpdate.AgentVersion
	}

	if desiredVersion == "" {
		return trace.Errorf("agent version not available from Teleport cluster")
	}
	switch cfg.Status.BackupVersion {
	case "", desiredVersion, cfg.Status.ActiveVersion:
	default:
		if desiredVersion == cfg.Status.ActiveVersion {
			// Keep backup version if we are only verifying active version
			break
		}
		err := u.Installer.Remove(ctx, cfg.Status.BackupVersion)
		if err != nil {
			// this could happen if it was already removed due to a failed installation
			u.Log.WarnContext(ctx, "Failed to remove backup version of Teleport before new install.", "error", err)
		}
	}
	// If the active version and target don't match, kick off upgrade.
	template := cfg.Spec.URLTemplate
	if template == "" {
		template = cdnURITemplate
	}
	err = u.Installer.Install(ctx, desiredVersion, template, 0) // TODO(sclevine): add web API for flags
	if err != nil {
		return trace.Errorf("failed to install: %w", err)
	}
	err = u.Installer.Link(ctx, desiredVersion)
	if err != nil {
		return trace.Errorf("failed to link: %w", err)
	}
	if cfg.Status.ActiveVersion != desiredVersion {
		cfg.Status.BackupVersion = cfg.Status.ActiveVersion
		cfg.Status.ActiveVersion = desiredVersion
		u.Log.InfoContext(ctx, "Target version successfully installed.", "version", desiredVersion)
	} else {
		u.Log.InfoContext(ctx, "Target version successfully validated.", "version", desiredVersion)
	}
	if v := cfg.Status.BackupVersion; v != "" {
		u.Log.InfoContext(ctx, "Backup version set.", "version", v)
	}

	versions, err := u.Installer.List(ctx)
	if err != nil {
		return trace.Errorf("failed to list installed versions: %w", err)
	}
	if n := len(versions); n > 2 {
		u.Log.WarnContext(ctx, "More than 2 versions of Teleport installed. Version directory may need cleanup to save space.", "count", n)
	}

	// Always write the configuration file if enable succeeds.
	if err := u.writeConfig(u.ConfigPath, cfg); err != nil {
		return trace.Errorf("failed to write %s: %w", updateConfigName, err)
	}
	u.Log.InfoContext(ctx, "Configuration updated.")
	return nil
}

func validateUpdatesSpec(spec *UpdateSpec) error {
	if spec.URLTemplate != "" &&
		!strings.HasPrefix(strings.ToLower(spec.URLTemplate), "https://") {
		return trace.Errorf("Teleport download URL must use TLS (https://)")
	}

	if spec.Proxy == "" {
		return trace.Errorf("Teleport proxy URL must be specified with --proxy or present in %s", updateConfigName)
	}
	return nil
}

// Disable disables agent auto-updates.
// This function is idempotent.
func (u *Updater) Disable(ctx context.Context) error {
	cfg, err := u.readConfig(u.ConfigPath)
	if err != nil {
		return trace.Errorf("failed to read %s: %w", updateConfigName, err)
	}
	if !cfg.Spec.Enabled {
		u.Log.InfoContext(ctx, "Automatic updates already disabled.")
		return nil
	}
	cfg.Spec.Enabled = false
	if err := u.writeConfig(u.ConfigPath, cfg); err != nil {
		return trace.Errorf("failed to write %s: %w", updateConfigName, err)
	}
	return nil
}

// readConfig reads UpdateConfig from a file.
func (*Updater) readConfig(path string) (*UpdateConfig, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &UpdateConfig{
			Version: updateConfigVersion,
			Kind:    updateConfigKind,
		}, nil
	}
	if err != nil {
		return nil, trace.Errorf("failed to open: %w", err)
	}
	defer f.Close()
	var cfg UpdateConfig
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, trace.Errorf("failed to parse: %w", err)
	}
	if k := cfg.Kind; k != updateConfigKind {
		return nil, trace.Errorf("invalid kind %q", k)
	}
	if v := cfg.Version; v != updateConfigVersion {
		return nil, trace.Errorf("invalid version %q", v)
	}
	return &cfg, nil
}

// writeConfig writes UpdateConfig to a file atomically, ensuring the file cannot be corrupted.
func (*Updater) writeConfig(filename string, cfg *UpdateConfig) error {
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
