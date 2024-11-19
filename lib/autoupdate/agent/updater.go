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
	"github.com/gravitational/teleport/lib/modules"
	libutils "github.com/gravitational/teleport/lib/utils"
)

const (
	// DefaultLinkDir is the default location where Teleport is linked.
	DefaultLinkDir = "/usr/local"
	// DefaultSystemDir is the location where packaged Teleport binaries and services are installed.
	DefaultSystemDir = "/usr/local/teleport-system"
)

const (
	// cdnURITemplate is the default template for the Teleport tgz download.
	cdnURITemplate = "https://cdn.teleport.dev/teleport{{if .Enterprise}}-ent{{end}}-v{{.Version}}-{{.OS}}-{{.Arch}}{{if .FIPS}}-fips{{end}}-bin.tar.gz"
	// reservedFreeDisk is the minimum required free space left on disk during downloads.
	// TODO(sclevine): This value is arbitrary and could be replaced by, e.g., min(1%, 200mb) in the future
	//   to account for a range of disk sizes.
	reservedFreeDisk = 10_000_000 // 10 MB
)

// Log keys
const (
	targetVersionKey = "target_version"
	activeVersionKey = "active_version"
	backupVersionKey = "backup_version"
	errorKey         = "error"
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
		cfg.LinkDir = DefaultLinkDir
	}
	if cfg.SystemDir == "" {
		cfg.SystemDir = DefaultSystemDir
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
			InstallDir: cfg.VersionsDir,
			LinkBinDir: filepath.Join(cfg.LinkDir, "bin"),
			// For backwards-compatibility with symlinks created by package-based installs, we always
			// link into /lib/systemd/system, even though, e.g., /usr/local/lib/systemd/system would work.
			LinkServiceDir:          filepath.Join("/", serviceDir),
			SystemBinDir:            filepath.Join(cfg.SystemDir, "bin"),
			SystemServiceDir:        filepath.Join(cfg.SystemDir, serviceDir),
			HTTP:                    client,
			Log:                     cfg.Log,
			ReservedFreeTmpDisk:     reservedFreeDisk,
			ReservedFreeInstallDisk: reservedFreeDisk,
		},
		Process: &SystemdService{
			ServiceName: "teleport.service",
			PIDPath:     "/run/teleport.pid",
			Log:         cfg.Log,
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
	// SystemDir for package-installed Teleport installations (usually /usr/local/teleport-system).
	SystemDir string
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
	// Process manages a running instance of Teleport.
	Process Process
}

// Installer provides an API for installing Teleport agents.
type Installer interface {
	// Install the Teleport agent at version from the download template.
	// Install must be idempotent.
	Install(ctx context.Context, version, template string, flags InstallFlags) error
	// Link the Teleport agent at the specified version of Teleport into the linking locations.
	// The revert function must restore the previous linking, returning false on any failure.
	// Link must be idempotent. Link's revert function must be idempotent.
	Link(ctx context.Context, version string) (revert func(context.Context) bool, err error)
	// LinkSystem links the system installation of Teleport into the linking locations.
	// The revert function must restore the previous linking, returning false on any failure.
	// LinkSystem must be idempotent. LinkSystem's revert function must be idempotent.
	LinkSystem(ctx context.Context) (revert func(context.Context) bool, err error)
	// TryLink links the specified version of Teleport into the linking locations.
	// Unlike Link, TryLink will fail if existing links to other locations are present.
	// TryLink must be idempotent.
	TryLink(ctx context.Context, version string) error
	// TryLinkSystem links the system installation of Teleport into the linking locations.
	// Unlike LinkSystem, TryLinkSystem will fail if existing links to other locations are present.
	// TryLinkSystem must be idempotent.
	TryLinkSystem(ctx context.Context) error
	// List the installed versions of Teleport.
	List(ctx context.Context) (versions []string, err error)
	// Remove the Teleport agent at version.
	// Must return ErrLinked if unable to remove due to being linked.
	// Remove must be idempotent.
	Remove(ctx context.Context, version string) error
}

var (
	// ErrLinked is returned when a linked version cannot be operated on.
	ErrLinked = errors.New("version is linked")
	// ErrNotNeeded is returned when the operation is not needed.
	ErrNotNeeded = errors.New("not needed")
	// ErrNotSupported is returned when the operation is not supported on the platform.
	ErrNotSupported = errors.New("not supported on this platform")
)

// Process provides an API for interacting with a running Teleport process.
type Process interface {
	// Reload must reload the Teleport process as gracefully as possible.
	// If the process is not healthy after reloading, Reload must return an error.
	// If the process did not require reloading, Reload must return ErrNotNeeded.
	// E.g., if the process is not enabled, or it was already reloaded after the last Sync.
	// If the type implementing Process does not support the system process manager,
	// Reload must return ErrNotSupported.
	Reload(ctx context.Context) error
	// Sync must validate and synchronize process configuration.
	// After the linked Teleport installation is changed, failure to call Sync without
	// error before Reload may result in undefined behavior.
	// If the type implementing Process does not support the system process manager,
	// Sync must return ErrNotSupported.
	Sync(ctx context.Context) error
}

// InstallFlags sets flags for the Teleport installation
type InstallFlags int

// TODO(sclevine): add flags for need_restart and selinux config
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
	// ForceFlags in installed Teleport.
	ForceFlags InstallFlags
}

// Enable enables agent updates and attempts an initial update.
// If the initial update succeeds, auto-updates are enabled and the configuration is persisted.
// Otherwise, the auto-updates configuration is not changed.
// This function is idempotent.
func (u *Updater) Enable(ctx context.Context, override OverrideConfig) error {
	// Read configuration from update.yaml and override any new values passed as flags.
	cfg, err := readConfig(u.ConfigPath)
	if err != nil {
		return trace.Errorf("failed to read %s: %w", updateConfigName, err)
	}
	if err := validateConfigSpec(&cfg.Spec, override); err != nil {
		return trace.Wrap(err)
	}
	if cfg.Spec.Proxy == "" {
		return trace.Errorf("Teleport proxy URL must be specified with --proxy or present in %s", updateConfigName)
	}

	// Lookup target version from the proxy.

	addr, err := libutils.ParseAddr(cfg.Spec.Proxy)
	if err != nil {
		return trace.Errorf("failed to parse proxy server address: %w", err)
	}
	targetVersion := override.ForceVersion
	flags := override.ForceFlags
	if targetVersion == "" {
		resp, err := webclient.Find(&webclient.Config{
			Context:     ctx,
			ProxyAddr:   addr.Addr,
			Insecure:    u.InsecureSkipVerify,
			Timeout:     30 * time.Second,
			UpdateGroup: cfg.Spec.Group,
			Pool:        u.Pool,
		})
		if err != nil {
			return trace.Errorf("failed to request version from proxy: %w", err)
		}
		targetVersion = resp.AutoUpdate.AgentVersion
		switch resp.Edition {
		case modules.BuildEnterprise:
			flags |= FlagEnterprise
		case modules.BuildOSS, modules.BuildCommunity:
		default:
			u.Log.WarnContext(ctx, "Unknown edition detected, defaulting to community.", "edition", resp.Edition)
		}
		if resp.FIPS {
			flags |= FlagFIPS
		}
	}

	if targetVersion == "" {
		return trace.Errorf("agent version not available from Teleport cluster")
	}

	if err := u.update(ctx, cfg, targetVersion, flags); err != nil {
		return trace.Wrap(err)
	}

	// Always write the configuration file if enable succeeds.

	cfg.Spec.Enabled = true
	if err := writeConfig(u.ConfigPath, cfg); err != nil {
		return trace.Errorf("failed to write %s: %w", updateConfigName, err)
	}
	u.Log.InfoContext(ctx, "Configuration updated.")
	return nil
}

// Disable disables agent auto-updates.
// This function is idempotent.
func (u *Updater) Disable(ctx context.Context) error {
	cfg, err := readConfig(u.ConfigPath)
	if err != nil {
		return trace.Errorf("failed to read %s: %w", updateConfigName, err)
	}
	if !cfg.Spec.Enabled {
		u.Log.InfoContext(ctx, "Automatic updates already disabled.")
		return nil
	}
	cfg.Spec.Enabled = false
	if err := writeConfig(u.ConfigPath, cfg); err != nil {
		return trace.Errorf("failed to write %s: %w", updateConfigName, err)
	}
	return nil
}

// Update initiates an agent update.
// If the update succeeds, the new installed version is marked as active.
// Otherwise, the auto-updates configuration is not changed.
// Unlike Enable, Update will not validate or repair the current version.
// This function is idempotent.
func (u *Updater) Update(ctx context.Context) error {
	// Read configuration from update.yaml and override any new values passed as flags.
	cfg, err := readConfig(u.ConfigPath)
	if err != nil {
		return trace.Errorf("failed to read %s: %w", updateConfigName, err)
	}
	if err := validateConfigSpec(&cfg.Spec, OverrideConfig{}); err != nil {
		return trace.Wrap(err)
	}
	activeVersion := cfg.Status.ActiveVersion
	if !cfg.Spec.Enabled {
		u.Log.InfoContext(ctx, "Automatic updates disabled.", activeVersionKey, activeVersion)
		return nil
	}
	if cfg.Spec.Proxy == "" {
		return trace.Errorf("Teleport proxy URL must be present in %s", updateConfigName)
	}

	// Lookup target version from the proxy.

	addr, err := libutils.ParseAddr(cfg.Spec.Proxy)
	if err != nil {
		return trace.Errorf("failed to parse proxy server address: %w", err)
	}
	resp, err := webclient.Find(&webclient.Config{
		Context:     ctx,
		ProxyAddr:   addr.Addr,
		Insecure:    u.InsecureSkipVerify,
		Timeout:     30 * time.Second,
		UpdateGroup: cfg.Spec.Group,
		Pool:        u.Pool,
	})
	if err != nil {
		return trace.Errorf("failed to request version from proxy: %w", err)
	}
	targetVersion := resp.AutoUpdate.AgentVersion
	var flags InstallFlags
	switch resp.Edition {
	case modules.BuildEnterprise:
		flags |= FlagEnterprise
	case modules.BuildOSS, modules.BuildCommunity:
	default:
		u.Log.WarnContext(ctx, "Unknown edition detected, defaulting to community.", "edition", resp.Edition)
	}
	if resp.FIPS {
		flags |= FlagFIPS
	}

	if !resp.AutoUpdate.AgentAutoUpdate {
		switch targetVersion {
		case "":
			u.Log.WarnContext(ctx, "Cannot determine target agent version. Waiting for both version and update window.")
		case activeVersion:
			u.Log.InfoContext(ctx, "Teleport is up-to-date. Update window is not active.", activeVersionKey, activeVersion)
		default:
			u.Log.InfoContext(ctx, "Update available, but update window is not active.", targetVersionKey, targetVersion, activeVersionKey, activeVersion)
		}
		return nil
	}

	switch targetVersion {
	case "":
		u.Log.ErrorContext(ctx, "Update window is active, but target version is not available.", activeVersionKey, activeVersion)
		return trace.Errorf("target version missing")
	case activeVersion:
		u.Log.InfoContext(ctx, "Teleport is up-to-date. Update window is active, but no action is needed.", activeVersionKey, activeVersion)
		return nil
	default:
		u.Log.InfoContext(ctx, "Update available. Initiating update.", targetVersionKey, targetVersion, activeVersionKey, activeVersion)
	}

	jitterSec := resp.AutoUpdate.AgentUpdateJitterSeconds
	time.Sleep(time.Duration(jitterSec) * time.Second)

	if err := u.update(ctx, cfg, targetVersion, flags); err != nil {
		return trace.Wrap(err)
	}

	// Write the configuration file if update succeeds.

	if err := writeConfig(u.ConfigPath, cfg); err != nil {
		return trace.Errorf("failed to write %s: %w", updateConfigName, err)
	}
	u.Log.InfoContext(ctx, "Configuration updated.")
	return nil
}

func (u *Updater) update(ctx context.Context, cfg *UpdateConfig, targetVersion string, flags InstallFlags) error {
	activeVersion := cfg.Status.ActiveVersion
	switch v := cfg.Status.BackupVersion; v {
	case "", targetVersion, activeVersion:
	default:
		if targetVersion == activeVersion {
			// Keep backup version if we are only verifying active version
			break
		}
		err := u.Installer.Remove(ctx, v)
		if err != nil {
			// this could happen if it was already removed due to a failed installation
			u.Log.WarnContext(ctx, "Failed to remove backup version of Teleport before new install.", errorKey, err, backupVersionKey, v)
		}
	}

	// Install the desired version (or validate existing installation)

	template := cfg.Spec.URLTemplate
	if template == "" {
		template = cdnURITemplate
	}
	err := u.Installer.Install(ctx, targetVersion, template, flags)
	if err != nil {
		return trace.Errorf("failed to install: %w", err)
	}
	revert, err := u.Installer.Link(ctx, targetVersion)
	if err != nil {
		return trace.Errorf("failed to link: %w", err)
	}

	// If we fail to revert after this point, the next update/enable will
	// fix the link to restore the active version.

	// Sync process configuration after linking.

	if err := u.Process.Sync(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			return trace.Errorf("sync canceled")
		}
		// If sync fails, we may have left the host in a bad state, so we revert linking and re-Sync.
		u.Log.ErrorContext(ctx, "Reverting symlinks due to invalid configuration.")
		if ok := revert(ctx); !ok {
			u.Log.ErrorContext(ctx, "Failed to revert Teleport symlinks. Installation likely broken.")
		} else if err := u.Process.Sync(ctx); err != nil {
			u.Log.ErrorContext(ctx, "Failed to sync configuration after failed restart.", errorKey, err)
		} else {
			u.Log.WarnContext(ctx, "Teleport updater encountered a configuration error and successfully reverted the installation.")
		}

		return trace.Errorf("failed to validate configuration for new version %q of Teleport: %w", targetVersion, err)
	}

	// Restart Teleport if necessary.

	if cfg.Status.ActiveVersion != targetVersion {
		u.Log.InfoContext(ctx, "Target version successfully installed.", targetVersionKey, targetVersion)
		if err := u.Process.Reload(ctx); err != nil && !errors.Is(err, ErrNotNeeded) {
			if errors.Is(err, context.Canceled) {
				return trace.Errorf("reload canceled")
			}
			// If reloading Teleport at the new version fails, revert, resync, and reload.
			u.Log.ErrorContext(ctx, "Reverting symlinks due to failed restart.")
			if ok := revert(ctx); !ok {
				u.Log.ErrorContext(ctx, "Failed to revert Teleport symlinks to older version. Installation likely broken.")
			} else if err := u.Process.Sync(ctx); err != nil {
				u.Log.ErrorContext(ctx, "Invalid configuration found after reverting Teleport to older version. Installation likely broken.", errorKey, err)
			} else if err := u.Process.Reload(ctx); err != nil && !errors.Is(err, ErrNotNeeded) {
				u.Log.ErrorContext(ctx, "Failed to revert Teleport to older version. Installation likely broken.", errorKey, err)
			} else {
				u.Log.WarnContext(ctx, "Teleport updater encountered an error during the update and successfully reverted the installation.")
			}

			return trace.Errorf("failed to start new version %q of Teleport: %w", targetVersion, err)
		}
		cfg.Status.BackupVersion = cfg.Status.ActiveVersion
		cfg.Status.ActiveVersion = targetVersion
	} else {
		u.Log.InfoContext(ctx, "Target version successfully validated.", targetVersionKey, targetVersion)
	}
	if v := cfg.Status.BackupVersion; v != "" {
		u.Log.InfoContext(ctx, "Backup version set.", backupVersionKey, v)
	}

	// Check if manual cleanup might be needed.

	versions, err := u.Installer.List(ctx)
	if err != nil {
		return trace.Errorf("failed to list installed versions: %w", err)
	}
	if n := len(versions); n > 2 {
		u.Log.WarnContext(ctx, "More than 2 versions of Teleport installed. Version directory may need cleanup to save space.", "count", n)
	}

	return nil
}

// readConfig reads UpdateConfig from a file.
func readConfig(path string) (*UpdateConfig, error) {
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
func writeConfig(filename string, cfg *UpdateConfig) error {
	opts := []renameio.Option{
		renameio.WithPermissions(configFileMode),
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

func validateConfigSpec(spec *UpdateSpec, override OverrideConfig) error {
	if override.Proxy != "" {
		spec.Proxy = override.Proxy
	}
	if override.Group != "" {
		spec.Group = override.Group
	}
	if override.URLTemplate != "" {
		spec.URLTemplate = override.URLTemplate
	}
	if spec.URLTemplate != "" &&
		!strings.HasPrefix(strings.ToLower(spec.URLTemplate), "https://") {
		return trace.Errorf("Teleport download URL must use TLS (https://)")
	}
	return nil
}

// LinkPackage creates links from the system (package) installation of Teleport, if they are needed.
// LinkPackage returns nils and warns if an auto-updates version is already linked, but auto-updates is disabled.
// LinkPackage returns an error only if an unknown version of Teleport is present (e.g., manually copied files).
// This function is idempotent.
func (u *Updater) LinkPackage(ctx context.Context) error {
	cfg, err := readConfig(u.ConfigPath)
	if err != nil {
		return trace.Errorf("failed to read %s: %w", updateConfigName, err)
	}
	if err := validateConfigSpec(&cfg.Spec, OverrideConfig{}); err != nil {
		return trace.Wrap(err)
	}
	activeVersion := cfg.Status.ActiveVersion
	if cfg.Spec.Enabled {
		u.Log.InfoContext(ctx, "Automatic updates enabled. Skipping system package link.", activeVersionKey, activeVersion)
		return nil
	}
	// If an active version is set, but auto-updates is disabled, try to link the system installation in case the config is stale.
	// If any links are present, this will return ErrLinked and not create any system links.
	// This state is important to log as a warning,
	if err := u.Installer.TryLinkSystem(ctx); errors.Is(err, ErrLinked) {
		u.Log.WarnContext(ctx, "Automatic updates disabled, but a non-package version of Teleport is linked.", activeVersionKey, activeVersion)
		return nil
	} else if err != nil {
		return trace.Errorf("failed to link system package installation: %w", err)
	}
	// TODO(sclevine): only if systemd files change
	if err := u.Process.Sync(ctx); err != nil {
		return trace.Errorf("failed to validate configuration for packaged installation of Teleport: %w", err)
	}
	u.Log.InfoContext(ctx, "Successfully linked system package installation.")
	return nil
}
