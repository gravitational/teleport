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
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	libdefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	libutils "github.com/gravitational/teleport/lib/utils"
)

const (
	// DefaultLinkDir is the default location where Teleport is linked.
	DefaultLinkDir = "/usr/local"
	// DefaultSystemDir is the location where packaged Teleport binaries and services are installed.
	DefaultSystemDir = "/usr/local/teleport-system"
	// VersionsDirName specifies the name of the subdirectory inside the Teleport data dir for storing Teleport versions.
	VersionsDirName = "versions"
	// BinaryName specifies the name of the updater binary.
	BinaryName = "teleport-update"
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
	if cfg.DataDir == "" {
		cfg.DataDir = libdefaults.DataDir
	}
	installDir := filepath.Join(cfg.DataDir, VersionsDirName)
	if err := os.MkdirAll(installDir, systemDirMode); err != nil {
		return nil, trace.Errorf("failed to create install directory: %w", err)
	}
	return &Updater{
		Log:                cfg.Log,
		Pool:               certPool,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		ConfigPath:         filepath.Join(installDir, updateConfigName),
		Installer: &LocalInstaller{
			InstallDir: installDir,
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
		Setup: func(ctx context.Context) error {
			name := filepath.Join(cfg.LinkDir, "bin", BinaryName)
			if cfg.SelfSetup && runtime.GOOS == constants.LinuxOS {
				name = "/proc/self/exe"
			}
			cmd := exec.CommandContext(ctx, name,
				"--data-dir", cfg.DataDir,
				"--link-dir", cfg.LinkDir,
				"setup")
			cmd.Stderr = os.Stderr
			cmd.Stdout = os.Stdout
			cfg.Log.InfoContext(ctx, "Executing new teleport-update binary to update configuration.")
			defer cfg.Log.InfoContext(ctx, "Finished executing new teleport-update binary.")
			err := cmd.Run()
			if cmd.ProcessState.ExitCode() == CodeNotSupported {
				return ErrNotSupported
			}
			return trace.Wrap(err)
		},
		Revert: func(ctx context.Context) error {
			return trace.Wrap(Setup(ctx, cfg.Log, cfg.LinkDir, cfg.DataDir))
		},
		Teardown: func(ctx context.Context) error {
			return trace.Wrap(Teardown(ctx, cfg.Log, cfg.LinkDir, cfg.DataDir))
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
	// DataDir for Teleport (usually /var/lib/teleport).
	DataDir string
	// LinkDir for installing Teleport (usually /usr/local).
	LinkDir string
	// SystemDir for package-installed Teleport installations (usually /usr/local/teleport-system).
	SystemDir string
	// SelfSetup mode for using the current version of the teleport-update to setup the update service.
	SelfSetup bool
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
	// Setup installs the Teleport updater service using the linked installation.
	Setup func(ctx context.Context) error
	// Revert installs the Teleport updater service using the running installation.
	Revert func(ctx context.Context) error
	// Teardown removes all traces of the updater and all managed installations.
	Teardown func(ctx context.Context) error
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
	// TryLinkSystem links the system (package) installation of Teleport into the linking locations.
	// Unlike LinkSystem, TryLinkSystem will fail if existing links to other locations are present.
	// TryLinkSystem must be idempotent.
	TryLinkSystem(ctx context.Context) error
	// Unlink unlinks the specified version of Teleport from the linking locations.
	// Unlink must be idempotent.
	Unlink(ctx context.Context, version string) error
	// UnlinkSystem unlinks the system (package) installation of Teleport from the linking locations.
	// UnlinkSystem must be idempotent.
	UnlinkSystem(ctx context.Context) error
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
	// ErrNoBinaries is returned when no binaries are available to be linked.
	ErrNoBinaries = errors.New("no binaries available to link")
)

const (
	// CodeNotSupported is returned when the operation is not supported on the platform.
	CodeNotSupported = 3
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
	// IsEnabled must return true if the Process is running or is configured to run.
	// If the type implementing Process does not support the system process manager,
	// Sync must return ErrNotSupported.
	IsEnabled(ctx context.Context) (bool, error)
}

// TODO(sclevine): add support for need_restart and selinux config

// OverrideConfig contains overrides for individual update operations.
// If validated, these overrides may be persisted to disk.
type OverrideConfig struct {
	UpdateSpec
	// ForceVersion to the specified version.
	ForceVersion string
	// ForceFlags in installed Teleport.
	ForceFlags InstallFlags
}

// Install attempts an initial installation of Teleport.
// If the initial installation succeeds, the override configuration is persisted.
// Otherwise, the configuration is not changed.
// This function is idempotent.
func (u *Updater) Install(ctx context.Context, override OverrideConfig) error {
	// Read configuration from update.yaml and override any new values passed as flags.
	cfg, err := readConfig(u.ConfigPath)
	if err != nil {
		return trace.Errorf("failed to read %s: %w", updateConfigName, err)
	}
	if err := validateConfigSpec(&cfg.Spec, override); err != nil {
		return trace.Wrap(err)
	}

	activeVersion := cfg.Status.ActiveVersion
	skipVersion := cfg.Status.SkipVersion

	// Lookup target version from the proxy.

	resp, err := u.find(ctx, cfg)
	if err != nil {
		return trace.Wrap(err)
	}
	targetVersion := resp.TargetVersion
	flags := resp.Flags
	flags |= override.ForceFlags
	if override.ForceVersion != "" {
		targetVersion = override.ForceVersion
	}

	switch targetVersion {
	case "":
		return trace.Errorf("agent version not available from Teleport cluster")
	case skipVersion:
		u.Log.WarnContext(ctx, "Target version was previously marked as broken. Retrying update.", targetVersionKey, targetVersion, activeVersionKey, activeVersion)
	default:
		u.Log.InfoContext(ctx, "Initiating installation.", targetVersionKey, targetVersion, activeVersionKey, activeVersion)
	}

	if err := u.update(ctx, cfg, targetVersion, flags); err != nil {
		return trace.Wrap(err)
	}
	if targetVersion == cfg.Status.SkipVersion {
		cfg.Status.SkipVersion = ""
	}

	// Only write the configuration file if the initial update succeeds.
	// Note: skip_version is never set on failed enable, only failed update.

	if err := writeConfig(u.ConfigPath, cfg); err != nil {
		return trace.Errorf("failed to write %s: %w", updateConfigName, err)
	}
	u.Log.InfoContext(ctx, "Configuration updated.")
	return nil
}

// Remove removes everything created by the updater.
// Before attempting this, Remove attempts to gracefully recover the system-packaged version of Teleport (if present).
// This function is idempotent.
func (u *Updater) Remove(ctx context.Context) error {
	cfg, err := readConfig(u.ConfigPath)
	if err != nil {
		return trace.Errorf("failed to read %s: %w", updateConfigName, err)
	}
	if err := validateConfigSpec(&cfg.Spec, OverrideConfig{}); err != nil {
		return trace.Wrap(err)
	}
	activeVersion := cfg.Status.ActiveVersion
	if activeVersion == "" {
		u.Log.InfoContext(ctx, "No installation of Teleport managed by the updater. Removing updater configuration.")
		if err := u.Teardown(ctx); err != nil {
			return trace.Wrap(err)
		}
		u.Log.InfoContext(ctx, "Automatic update configuration for Teleport successfully uninstalled.")
		return nil
	}

	revert, err := u.Installer.LinkSystem(ctx)
	if errors.Is(err, ErrNoBinaries) {
		u.Log.InfoContext(ctx, "Updater-managed installation of Teleport detected. Attempting to unlink and remove.")
		ok, err := u.Process.IsEnabled(ctx)
		if err != nil && !errors.Is(err, ErrNotSupported) {
			return trace.Wrap(err)
		}
		if ok {
			return trace.Errorf("refusing to remove active installation of Teleport, please disable Teleport first")
		}
		if err := u.Installer.Unlink(ctx, activeVersion); err != nil {
			return trace.Wrap(err)
		}
		u.Log.InfoContext(ctx, "Teleport uninstalled.", "version", activeVersion)
		if err := u.Teardown(ctx); err != nil {
			return trace.Wrap(err)
		}
		u.Log.InfoContext(ctx, "Automatic update configuration for Teleport successfully uninstalled.")
		return nil
	}
	if err != nil {
		return trace.Errorf("failed to link: %w", err)
	}

	u.Log.InfoContext(ctx, "Updater-managed installation of Teleport detected. Restoring packaged version of Teleport before removing.")

	revertConfig := func(ctx context.Context) bool {
		if ok := revert(ctx); !ok {
			u.Log.ErrorContext(ctx, "Failed to revert Teleport symlinks. Installation likely broken.")
			return false
		}
		if err := u.Process.Sync(ctx); err != nil {
			u.Log.ErrorContext(ctx, "Failed to revert systemd configuration after failed restart.", errorKey, err)
			return false
		}
		return true
	}

	// Sync systemd.

	err = u.Process.Sync(ctx)
	if errors.Is(err, ErrNotSupported) {
		u.Log.WarnContext(ctx, "Not syncing systemd configuration because systemd is not running.")
	} else if errors.Is(err, context.Canceled) {
		return trace.Errorf("sync canceled")
	} else if err != nil {
		// If sync fails, we may have left the host in a bad state, so we revert linking and re-Sync.
		u.Log.ErrorContext(ctx, "Reverting symlinks due to invalid configuration.")
		if ok := revertConfig(ctx); ok {
			u.Log.WarnContext(ctx, "Teleport updater encountered a configuration error and successfully reverted the installation.")
		}
		return trace.Errorf("failed to validate configuration for system package version of Teleport: %w", err)
	}

	// Restart Teleport.

	u.Log.InfoContext(ctx, "Teleport package successfully restored.")
	err = u.Process.Reload(ctx)
	if errors.Is(err, context.Canceled) {
		return trace.Errorf("reload canceled")
	}
	if err != nil &&
		!errors.Is(err, ErrNotNeeded) && // no output if restart not needed
		!errors.Is(err, ErrNotSupported) { // already logged above for Sync

		// If reloading Teleport at the new version fails, revert and reload.
		u.Log.ErrorContext(ctx, "Reverting symlinks due to failed restart.")
		if ok := revertConfig(ctx); ok {
			if err := u.Process.Reload(ctx); err != nil && !errors.Is(err, ErrNotNeeded) {
				u.Log.ErrorContext(ctx, "Failed to reload Teleport after reverting. Installation likely broken.", errorKey, err)
			} else {
				u.Log.WarnContext(ctx, "Teleport updater detected an error with the new installation and successfully reverted it.")
			}
		}
		return trace.Errorf("failed to start system package version of Teleport: %w", err)
	}
	u.Log.InfoContext(ctx, "Auto-updating Teleport removed and replaced by Teleport packaged.", "version", activeVersion)
	if err := u.Teardown(ctx); err != nil {
		return trace.Wrap(err)
	}
	u.Log.InfoContext(ctx, "Auto-update configuration for Teleport successfully uninstalled.")
	return nil
}

// Status returns all available local and remote fields related to agent auto-updates.
func (u *Updater) Status(ctx context.Context) (Status, error) {
	var out Status
	// Read configuration from update.yaml.
	cfg, err := readConfig(u.ConfigPath)
	if err != nil {
		return out, trace.Errorf("failed to read %s: %w", updateConfigName, err)
	}
	if err := validateConfigSpec(&cfg.Spec, OverrideConfig{}); err != nil {
		return out, trace.Wrap(err)
	}
	out.UpdateSpec = cfg.Spec
	out.UpdateStatus = cfg.Status

	// Lookup target version from the proxy.
	resp, err := u.find(ctx, cfg)
	if err != nil {
		return out, trace.Wrap(err)
	}
	out.FindResp = resp
	return out, nil
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

// Unpin allows the current version to be changed by Update.
// This function is idempotent.
func (u *Updater) Unpin(ctx context.Context) error {
	cfg, err := readConfig(u.ConfigPath)
	if err != nil {
		return trace.Errorf("failed to read %s: %w", updateConfigName, err)
	}
	if !cfg.Spec.Pinned {
		u.Log.InfoContext(ctx, "Current version not pinned.", activeVersionKey, cfg.Status.ActiveVersion)
		return nil
	}
	cfg.Spec.Pinned = false
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
	skipVersion := cfg.Status.SkipVersion
	if !cfg.Spec.Enabled {
		u.Log.InfoContext(ctx, "Automatic updates disabled.", activeVersionKey, activeVersion)
		return nil
	}

	resp, err := u.find(ctx, cfg)
	if err != nil {
		return trace.Wrap(err)
	}
	targetVersion := resp.TargetVersion

	if cfg.Spec.Pinned {
		switch targetVersion {
		case activeVersion:
			u.Log.InfoContext(ctx, "Teleport is up-to-date. Installation is pinned to prevent future updates.", activeVersionKey, activeVersion)
		default:
			u.Log.InfoContext(ctx, "Teleport version is pinned. Skipping update.", targetVersionKey, targetVersion, activeVersionKey, activeVersion)
		}
		return nil
	}

	if !resp.InWindow {
		switch targetVersion {
		case "":
			u.Log.WarnContext(ctx, "Cannot determine target agent version. Waiting for both version and update window.")
		case activeVersion:
			u.Log.InfoContext(ctx, "Teleport is up-to-date. Update window is not active.", activeVersionKey, activeVersion)
		case skipVersion:
			u.Log.InfoContext(ctx, "Update available, but the new version is marked as broken. Update window is not active.", targetVersionKey, targetVersion, activeVersionKey, activeVersion)
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
	case skipVersion:
		u.Log.InfoContext(ctx, "Update available, but the new version is marked as broken. Skipping update during the update window.", targetVersionKey, targetVersion, activeVersionKey, activeVersion)
		return nil
	default:
		u.Log.InfoContext(ctx, "Update available. Initiating update.", targetVersionKey, targetVersion, activeVersionKey, activeVersion)
	}
	time.Sleep(resp.Jitter)

	updateErr := u.update(ctx, cfg, targetVersion, resp.Flags)
	writeErr := writeConfig(u.ConfigPath, cfg)
	if writeErr != nil {
		writeErr = trace.Errorf("failed to write %s: %w", updateConfigName, writeErr)
	} else {
		u.Log.InfoContext(ctx, "Configuration updated.")
	}
	return trace.NewAggregate(updateErr, writeErr)
}

func (u *Updater) find(ctx context.Context, cfg *UpdateConfig) (FindResp, error) {
	if cfg.Spec.Proxy == "" {
		return FindResp{}, trace.Errorf("Teleport proxy URL must be specified with --proxy or present in %s", updateConfigName)
	}
	addr, err := libutils.ParseAddr(cfg.Spec.Proxy)
	if err != nil {
		return FindResp{}, trace.Errorf("failed to parse proxy server address: %w", err)
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
		return FindResp{}, trace.Errorf("failed to request version from proxy: %w", err)
	}
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
	jitterSec := resp.AutoUpdate.AgentUpdateJitterSeconds
	return FindResp{
		TargetVersion: resp.AutoUpdate.AgentVersion,
		Flags:         flags,
		InWindow:      resp.AutoUpdate.AgentAutoUpdate,
		Jitter:        time.Duration(jitterSec) * time.Second,
	}, nil
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

	// Install and link the desired version (or validate existing installation)

	template := cfg.Spec.URLTemplate
	if template == "" {
		template = cdnURITemplate
	}
	err := u.Installer.Install(ctx, targetVersion, template, flags)
	if err != nil {
		return trace.Errorf("failed to install: %w", err)
	}

	// TODO(sclevine): if the target version has fewer binaries, this will
	//  leave old binaries linked. This may prevent the installation from
	//  being removed. To fix this, we should look for orphaned binaries
	//  and remove them, or alternatively, attempt to remove extra versions.

	revert, err := u.Installer.Link(ctx, targetVersion)
	if err != nil {
		return trace.Errorf("failed to link: %w", err)
	}

	// If we fail to revert after this point, the next update/enable will
	// fix the link to restore the active version.

	revertConfig := func(ctx context.Context) bool {
		cfg.Status.SkipVersion = targetVersion
		if ok := revert(ctx); !ok {
			u.Log.ErrorContext(ctx, "Failed to revert Teleport symlinks. Installation likely broken.")
			return false
		}
		if err := u.Revert(ctx); err != nil {
			u.Log.ErrorContext(ctx, "Failed to revert configuration after failed restart.", errorKey, err)
			return false
		}
		return true
	}

	// Setup teleport-updater configuration and sync systemd.

	err = u.Setup(ctx)
	if errors.Is(err, ErrNotSupported) {
		u.Log.WarnContext(ctx, "Not syncing systemd configuration because systemd is not running.")
	} else if errors.Is(err, context.Canceled) {
		return trace.Errorf("sync canceled")
	} else if err != nil {
		// If sync fails, we may have left the host in a bad state, so we revert linking and re-Sync.
		u.Log.ErrorContext(ctx, "Reverting symlinks due to invalid configuration.")
		if ok := revertConfig(ctx); ok {
			u.Log.WarnContext(ctx, "Teleport updater encountered a configuration error and successfully reverted the installation.")
		}
		return trace.Errorf("failed to validate configuration for new version %q of Teleport: %w", targetVersion, err)
	}

	// Restart Teleport if necessary.

	if cfg.Status.ActiveVersion != targetVersion {
		u.Log.InfoContext(ctx, "Target version successfully installed.", targetVersionKey, targetVersion)
		err = u.Process.Reload(ctx)
		if errors.Is(err, context.Canceled) {
			return trace.Errorf("reload canceled")
		}
		if err != nil &&
			!errors.Is(err, ErrNotNeeded) && // no output if restart not needed
			!errors.Is(err, ErrNotSupported) { // already logged above for Sync

			// If reloading Teleport at the new version fails, revert and reload.
			u.Log.ErrorContext(ctx, "Reverting symlinks due to failed restart.")
			if ok := revertConfig(ctx); ok {
				if err := u.Process.Reload(ctx); err != nil && !errors.Is(err, ErrNotNeeded) {
					u.Log.ErrorContext(ctx, "Failed to reload Teleport after reverting. Installation likely broken.", errorKey, err)
				} else {
					u.Log.WarnContext(ctx, "Teleport updater detected an error with the new installation and successfully reverted it.")
				}
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
		u.Log.ErrorContext(ctx, "Failed to read installed versions.", errorKey, err)
	} else if n := len(versions); n > 2 {
		u.Log.WarnContext(ctx, "More than 2 versions of Teleport installed. Version directory may need cleanup to save space.", "count", n)
	}
	return nil
}

// LinkPackage creates links from the system (package) installation of Teleport, if they are needed.
// LinkPackage returns nil and warns if an auto-updates version is already linked, but auto-updates is disabled.
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
		u.Log.InfoContext(ctx, "Automatic updates is enabled. Skipping system package link.", activeVersionKey, activeVersion)
		return nil
	}
	if cfg.Spec.Pinned {
		u.Log.InfoContext(ctx, "Automatic update version is pinned. Skipping system package link.", activeVersionKey, activeVersion)
		return nil
	}
	// If an active version is set, but auto-updates is disabled, try to link the system installation in case the config is stale.
	// If any links are present, this will return ErrLinked and not create any system links.
	// This state is important to log as a warning,
	if err := u.Installer.TryLinkSystem(ctx); errors.Is(err, ErrLinked) {
		u.Log.WarnContext(ctx, "Automatic updates is disabled, but a non-package version of Teleport is linked.", activeVersionKey, activeVersion)
		return nil
	} else if err != nil {
		return trace.Errorf("failed to link system package installation: %w", err)
	}
	if err := u.Process.Sync(ctx); err != nil {
		return trace.Errorf("failed to sync systemd configuration: %w", err)
	}
	u.Log.InfoContext(ctx, "Successfully linked system package installation.")
	return nil
}

// UnlinkPackage removes links from the system (package) installation of Teleport, if they are present.
// This function is idempotent.
func (u *Updater) UnlinkPackage(ctx context.Context) error {
	if err := u.Installer.UnlinkSystem(ctx); err != nil {
		return trace.Errorf("failed to unlink system package installation: %w", err)
	}
	if err := u.Process.Sync(ctx); err != nil {
		return trace.Errorf("failed to sync systemd configuration: %w", err)
	}
	u.Log.InfoContext(ctx, "Successfully unlinked system package installation.")
	return nil
}
