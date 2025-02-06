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
	"slices"
	"time"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/autoupdate"
	"github.com/gravitational/teleport/lib/client/debug"
	libdefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	libutils "github.com/gravitational/teleport/lib/utils"
)

const (
	// DefaultLinkDir is the default location where Teleport is linked.
	DefaultLinkDir = "/usr/local/bin"
	// BinaryName specifies the name of the updater binary.
	BinaryName = "teleport-update"
)

const (
	// defaultSystemDir is the location where packaged Teleport binaries and services are installed.
	defaultSystemDir = "/opt/teleport/system"
	// reservedFreeDisk is the minimum required free space left on disk during downloads.
	// TODO(sclevine): This value is arbitrary and could be replaced by, e.g., min(1%, 200mb) in the future
	//   to account for a range of disk sizes.
	reservedFreeDisk = 10_000_000
	// debugSocketFileName is the name of Teleport's debug socket in the data dir.
	debugSocketFileName = "debug.sock" // 10 MB
)

// Log keys
const (
	targetKey = "target_version"
	activeKey = "active_version"
	backupKey = "backup_version"
	errorKey  = "error"
)

// NewLocalUpdater returns a new Updater that auto-updates local
// installations of the Teleport agent.
// The AutoUpdater uses an HTTP client with sane defaults for downloads, and
// will not fill disk to within 10 MB of available capacity.
func NewLocalUpdater(cfg LocalUpdaterConfig, ns *Namespace) (*Updater, error) {
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
	if cfg.SystemDir == "" {
		cfg.SystemDir = defaultSystemDir
	}
	validator := Validator{Log: cfg.Log}
	debugClient := debug.NewClient(filepath.Join(ns.dataDir, debugSocketFileName))
	return &Updater{
		Log:                cfg.Log,
		Pool:               certPool,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		UpdateConfigPath:   ns.updaterConfigFile,
		TeleportConfigPath: ns.configFile,
		Installer: &LocalInstaller{
			InstallDir:              ns.versionsDir,
			LinkBinDir:              ns.linkDir,
			CopyServiceFile:         ns.serviceFile,
			SystemBinDir:            filepath.Join(cfg.SystemDir, "bin"),
			SystemServiceFile:       filepath.Join(cfg.SystemDir, serviceDir, serviceName),
			HTTP:                    client,
			Log:                     cfg.Log,
			ReservedFreeTmpDisk:     reservedFreeDisk,
			ReservedFreeInstallDisk: reservedFreeDisk,
			TransformService:        ns.replaceTeleportService,
			ValidateBinary:          validator.IsBinary,
			Template:                autoupdate.DefaultCDNURITemplate,
		},
		Process: &SystemdService{
			ServiceName: filepath.Base(ns.serviceFile),
			PIDPath:     ns.pidFile,
			Ready:       debugClient,
			Log:         cfg.Log,
		},
		ReexecSetup: func(ctx context.Context, reload bool) error {
			name := ns.updaterBinFile
			if cfg.SelfSetup && runtime.GOOS == constants.LinuxOS {
				name = "/proc/self/exe"
			}
			args := []string{
				"--data-dir", ns.dataDir,
				"--link-dir", ns.linkDir,
				"--install-suffix", ns.name,
				"--log-format", cfg.LogFormat,
			}
			if cfg.Debug {
				args = append(args, "--debug")
			}
			args = append(args, "setup")
			if reload {
				args = append(args, "--reload")
			}
			cmd := exec.CommandContext(ctx, name, args...)
			cmd.Stderr = os.Stderr
			cmd.Stdout = os.Stdout
			cfg.Log.InfoContext(ctx, "Executing new teleport-update binary to update configuration.")
			defer cfg.Log.InfoContext(ctx, "Finished executing new teleport-update binary.")
			return trace.Wrap(cmd.Run())
		},
		SetupNamespace:    ns.Setup,
		TeardownNamespace: ns.Teardown,
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
	// SystemDir for package-installed Teleport installations (usually /opt/teleport/system).
	SystemDir string
	// SelfSetup mode for using the current version of the teleport-update to setup the update service.
	SelfSetup bool
	// Debug logs enabled.
	Debug bool
	// LogFormat controls the format of logging. Can be either `json` or `text`.
	LogFormat string
}

// Updater implements the agent-local logic for Teleport agent auto-updates.
type Updater struct {
	// Log contains a logger.
	Log *slog.Logger
	// Pool used for requests to the Teleport web API.
	Pool *x509.CertPool
	// InsecureSkipVerify skips TLS verification.
	InsecureSkipVerify bool
	// UpdateConfigPath contains the path to the agent auto-updates configuration.
	UpdateConfigPath string
	// TeleportConfig contains the path to Teleport's configuration.
	TeleportConfigPath string
	// Installer manages installations of the Teleport agent.
	Installer Installer
	// Process manages a running instance of Teleport.
	Process Process
	// ReexecSetup re-execs teleport-update with the setup command.
	// This configures the updater service, verifies the installation, and optionally reloads Teleport.
	ReexecSetup func(ctx context.Context, reload bool) error
	// SetupNamespace configures the Teleport updater service for the current Namespace.
	SetupNamespace func(ctx context.Context) error
	// TeardownNamespace removes all traces of the updater service in the current Namespace, including Teleport.
	TeardownNamespace func(ctx context.Context) error
}

// Installer provides an API for installing Teleport agents.
type Installer interface {
	// Install the Teleport agent at revision from the download Template.
	// Install must be idempotent.
	Install(ctx context.Context, rev Revision, baseURL string) error
	// Link the Teleport agent at the specified revision of Teleport into the linking locations.
	// The revert function must restore the previous linking, returning false on any failure.
	// Link must be idempotent. Link's revert function must be idempotent.
	Link(ctx context.Context, rev Revision, force bool) (revert func(context.Context) bool, err error)
	// LinkSystem links the system installation of Teleport into the linking locations.
	// The revert function must restore the previous linking, returning false on any failure.
	// LinkSystem must be idempotent. LinkSystem's revert function must be idempotent.
	LinkSystem(ctx context.Context) (revert func(context.Context) bool, err error)
	// TryLink links the specified revision of Teleport into the linking locations.
	// Unlike Link, TryLink will fail if existing links to other locations are present.
	// TryLink must be idempotent.
	TryLink(ctx context.Context, rev Revision) error
	// TryLinkSystem links the system (package) installation of Teleport into the linking locations.
	// Unlike LinkSystem, TryLinkSystem will fail if existing links to other locations are present.
	// TryLinkSystem must be idempotent.
	TryLinkSystem(ctx context.Context) error
	// Unlink unlinks the specified revision of Teleport from the linking locations.
	// Unlink must be idempotent.
	Unlink(ctx context.Context, rev Revision) error
	// UnlinkSystem unlinks the system (package) installation of Teleport from the linking locations.
	// UnlinkSystem must be idempotent.
	UnlinkSystem(ctx context.Context) error
	// List the installed revisions of Teleport.
	List(ctx context.Context) (revisions []Revision, err error)
	// Remove the Teleport agent at revision.
	// Must return ErrLinked if unable to remove due to being linked.
	// Remove must be idempotent.
	Remove(ctx context.Context, rev Revision) error
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
	// ErrFilePresent is returned when a file is present.
	ErrFilePresent = errors.New("file present")
	// ErrInvalid is returned when an operation is invalid.
	ErrInvalid = errors.New("invalid")
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
	// IsEnabled must return true if the Process is configured to run on system boot.
	// If the type implementing Process does not support the system process manager,
	// IsEnabled must return ErrNotSupported.
	IsEnabled(ctx context.Context) (bool, error)
	// IsActive must return true if the Process is currently running.
	// If the type implementing Process does not support the system process manager,
	// IsActive must return ErrNotSupported.
	IsActive(ctx context.Context) (bool, error)
	// IsPresent must return true if the Process is installed on the system.
	// If the type implementing Process does not support the system process manager,
	// IsPresent must return ErrNotSupported.
	IsPresent(ctx context.Context) (bool, error)
}

// OverrideConfig contains overrides for individual update operations.
// If validated, these overrides may be persisted to disk.
type OverrideConfig struct {
	UpdateSpec
	// ForceVersion to the specified version.
	ForceVersion string
	// ForceFlags in installed Teleport.
	ForceFlags autoupdate.InstallFlags
	// AllowOverwrite of installed binaries.
	AllowOverwrite bool
}

func deref[T any](ptr *T) T {
	if ptr != nil {
		return *ptr
	}
	var t T
	return t
}

func toPtr[T any](t T) *T {
	return &t
}

// Install attempts an initial installation of Teleport.
// If the initial installation succeeds, the override configuration is persisted.
// Otherwise, the configuration is not changed.
// This function is idempotent.
func (u *Updater) Install(ctx context.Context, override OverrideConfig) error {
	// Read configuration from update.yaml and override any new values passed as flags.
	cfg, err := readConfig(u.UpdateConfigPath)
	if err != nil {
		return trace.Wrap(err, "failed to read %s", updateConfigName)
	}
	if err := validateConfigSpec(&cfg.Spec, override); err != nil {
		return trace.Wrap(err)
	}

	agentProxy := u.findAgentProxy(ctx)
	if cfg.Spec.Proxy == "" {
		cfg.Spec.Proxy = agentProxy
	} else if agentProxy != "" &&
		!sameProxies(cfg.Spec.Proxy, agentProxy) {
		u.Log.WarnContext(ctx, "Proxy specified in update.yaml does not match teleport.yaml. Unexpected updates may occur.", "update_proxy", cfg.Spec.Proxy, "teleport_proxy", agentProxy)
	}

	active := cfg.Status.Active
	skip := deref(cfg.Status.Skip)

	// Lookup target version from the proxy.

	resp, err := u.find(ctx, cfg)
	if err != nil {
		return trace.Wrap(err)
	}
	targetVersion := resp.Target.Version
	targetFlags := resp.Target.Flags
	targetFlags |= override.ForceFlags
	if override.ForceVersion != "" {
		targetVersion = override.ForceVersion
	}
	target := NewRevision(targetVersion, targetFlags)

	switch target.Version {
	case "":
		return trace.Errorf("agent version not available from Teleport cluster")
	case skip.Version:
		u.Log.WarnContext(ctx, "Target version was previously marked as broken. Retrying update.", targetKey, target, activeKey, active)
	default:
		u.Log.InfoContext(ctx, "Initiating installation.", targetKey, target, activeKey, active)
	}

	if err := u.update(ctx, cfg, target, override.AllowOverwrite, resp.AGPL); err != nil {
		if errors.Is(err, ErrFilePresent) && !override.AllowOverwrite {
			u.Log.WarnContext(ctx, "Use --overwrite to force removal of existing binaries installed via script.")
			u.Log.WarnContext(ctx, "If a teleport rpm or deb package is installed, upgrade it to the latest version and retry. DO NOT USE --overwrite.")
		}
		return trace.Wrap(err)
	}
	if target.Version == skip.Version {
		cfg.Status.Skip = nil
	}

	// Only write the configuration file if the initial update succeeds.
	// Note: skip_version is never set on failed enable, only failed update.

	if err := writeConfig(u.UpdateConfigPath, cfg); err != nil {
		return trace.Wrap(err, "failed to write %s", updateConfigName)
	}
	u.Log.InfoContext(ctx, "Configuration updated.")
	return trace.Wrap(u.notices(ctx))
}

// proxyAddrConfig contains potential proxy server addresses from teleport.yaml.
type proxyAddrConfig struct {
	Teleport proxyAddrTeleport `yaml:"teleport"`
}

type proxyAddrTeleport struct {
	AuthServers []string `yaml:"auth_servers"`
	AuthServer  string   `yaml:"auth_server"`
	ProxyServer string   `yaml:"proxy_server"`
}

// findAgentProxy finds a proxy in teleport.yaml if not specified or set in update configuration.
// Note that any implicitly defaulted port in teleport.yaml is explicitly defaulted (to 3080) by this method.
func (u *Updater) findAgentProxy(ctx context.Context) string {
	f, err := libutils.OpenFileAllowingUnsafeLinks(u.TeleportConfigPath)
	if err != nil {
		u.Log.DebugContext(ctx, "Unable to open Teleport config to read proxy", "config", u.TeleportConfigPath, errorKey, err)
		return ""
	}
	defer f.Close()
	var cfg proxyAddrConfig
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		u.Log.DebugContext(ctx, "Unable to parse Teleport config to read proxy", "config", u.TeleportConfigPath, errorKey, err)
		return ""
	}

	var addr string
	var port int
	switch t := cfg.Teleport; {
	case t.ProxyServer != "":
		addr = t.ProxyServer
		port = libdefaults.HTTPListenPort
	case t.AuthServer != "":
		addr = t.AuthServer
		port = libdefaults.AuthListenPort
	case len(t.AuthServers) > 0:
		addr = t.AuthServers[0]
		port = libdefaults.AuthListenPort
	}
	netaddr, err := libutils.ParseHostPortAddr(addr, port)
	if err != nil {
		u.Log.DebugContext(ctx, "Unable to parse proxy in Teleport config", "config", u.TeleportConfigPath, "proxy_addr", addr, "proxy_port", port, errorKey, err)
		return ""
	}
	return netaddr.String()
}

// sameProxies returns true if both proxies addresses are the same.
// Note that the port is defaulted to 443, which is different from teleport.yaml's default.
func sameProxies(a, b string) bool {
	const defaultPort = 443
	if a == b {
		return true
	}
	addrA, err := libutils.ParseAddr(a)
	if err != nil {
		return false
	}
	addrB, err := libutils.ParseAddr(b)
	if err != nil {
		return false
	}
	return addrA.Host() == addrB.Host() &&
		addrA.Port(defaultPort) == addrB.Port(defaultPort)
}

// Remove removes everything created by the updater for the given namespace.
// Before attempting this, Remove attempts to gracefully recover the system-packaged version of Teleport (if present).
// This function is idempotent.
func (u *Updater) Remove(ctx context.Context) error {
	cfg, err := readConfig(u.UpdateConfigPath)
	if err != nil {
		return trace.Wrap(err, "failed to read %s", updateConfigName)
	}
	if err := validateConfigSpec(&cfg.Spec, OverrideConfig{}); err != nil {
		return trace.Wrap(err)
	}
	active := cfg.Status.Active
	if active.Version == "" {
		u.Log.InfoContext(ctx, "No installation of Teleport managed by the updater. Removing updater configuration.")
		if err := u.TeardownNamespace(ctx); err != nil {
			return trace.Wrap(err)
		}
		u.Log.InfoContext(ctx, "Automatic update configuration for Teleport successfully uninstalled.")
		return nil
	}

	revert, err := u.Installer.LinkSystem(ctx)
	if errors.Is(err, ErrNoBinaries) ||
		errors.Is(err, ErrInvalid) {
		u.Log.InfoContext(ctx, "Updater-managed installation of Teleport detected. Attempting to unlink and remove.")
		ok, err := isActiveOrEnabled(ctx, u.Process)
		if err != nil && !errors.Is(err, ErrNotSupported) {
			return trace.Wrap(err)
		}
		if ok {
			return trace.Errorf("refusing to remove active installation of Teleport, please stop and disable Teleport first")
		}
		if err := u.Installer.Unlink(ctx, active); err != nil {
			return trace.Wrap(err)
		}
		u.Log.InfoContext(ctx, "Teleport uninstalled.", "version", active)
		if err := u.TeardownNamespace(ctx); err != nil {
			return trace.Wrap(err)
		}
		u.Log.InfoContext(ctx, "Automatic update configuration for Teleport successfully uninstalled.")
		return nil
	}
	if err != nil {
		return trace.Wrap(err, "failed to link")
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
	if errors.Is(err, context.Canceled) {
		return trace.Errorf("sync canceled")
	}
	if errors.Is(err, ErrNotSupported) {
		u.Log.WarnContext(ctx, "Not syncing systemd configuration because systemd is not running.")
	} else if err != nil {
		// If sync fails, we may have left the host in a bad state, so we revert linking and re-Sync.
		u.Log.ErrorContext(ctx, "Reverting symlinks due to invalid configuration.")
		if ok := revertConfig(ctx); ok {
			u.Log.WarnContext(ctx, "Teleport updater encountered a configuration error and successfully reverted the installation.")
		}
		return trace.Wrap(err, "failed to validate configuration for system package version of Teleport")
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
		return trace.Wrap(err, "failed to start system package version of Teleport")
	}
	u.Log.InfoContext(ctx, "Auto-updating Teleport removed and replaced by Teleport package.", "version", active)
	if err := u.TeardownNamespace(ctx); err != nil {
		return trace.Wrap(err)
	}
	u.Log.InfoContext(ctx, "Auto-update configuration for Teleport successfully uninstalled.")
	return nil
}

// isActiveOrEnabled returns true if the service is active or enabled.
func isActiveOrEnabled(ctx context.Context, s Process) (bool, error) {
	enabled, err := s.IsEnabled(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	if enabled {
		return true, nil
	}
	active, err := s.IsActive(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	if active {
		return true, nil
	}
	return false, nil
}

// Status returns all available local and remote fields related to agent auto-updates.
// Status is safe to run concurrently with other Updater commands.
func (u *Updater) Status(ctx context.Context) (Status, error) {
	var out Status
	// Read configuration from update.yaml.
	cfg, err := readConfig(u.UpdateConfigPath)
	if err != nil {
		return out, trace.Wrap(err, "failed to read %s", updateConfigName)
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
	cfg, err := readConfig(u.UpdateConfigPath)
	if err != nil {
		return trace.Wrap(err, "failed to read %s", updateConfigName)
	}
	if !cfg.Spec.Enabled {
		u.Log.InfoContext(ctx, "Automatic updates already disabled.")
		return nil
	}
	cfg.Spec.Enabled = false
	if err := writeConfig(u.UpdateConfigPath, cfg); err != nil {
		return trace.Wrap(err, "failed to write %s", updateConfigName)
	}
	return nil
}

// Unpin allows the current version to be changed by Update.
// This function is idempotent.
func (u *Updater) Unpin(ctx context.Context) error {
	cfg, err := readConfig(u.UpdateConfigPath)
	if err != nil {
		return trace.Wrap(err, "failed to read %s", updateConfigName)
	}
	if !cfg.Spec.Pinned {
		u.Log.InfoContext(ctx, "Current version not pinned.", activeKey, cfg.Status.Active)
		return nil
	}
	cfg.Spec.Pinned = false
	if err := writeConfig(u.UpdateConfigPath, cfg); err != nil {
		return trace.Wrap(err, "failed to write %s", updateConfigName)
	}
	return nil
}

// Update initiates an agent update.
// If the update succeeds, the new installed version is marked as active.
// Otherwise, the auto-updates configuration is not changed.
// Unlike Enable, Update will not validate or repair the current version.
// This function is idempotent.
func (u *Updater) Update(ctx context.Context, now bool) error {
	// Read configuration from update.yaml and override any new values passed as flags.
	cfg, err := readConfig(u.UpdateConfigPath)
	if err != nil {
		return trace.Wrap(err, "failed to read %s", updateConfigName)
	}
	if err := validateConfigSpec(&cfg.Spec, OverrideConfig{}); err != nil {
		return trace.Wrap(err)
	}
	if p := u.findAgentProxy(ctx); p != "" &&
		!sameProxies(cfg.Spec.Proxy, p) {
		u.Log.WarnContext(ctx, "Proxy specified in update.yaml does not match teleport.yaml. Unexpected updates may occur.", "update_proxy", cfg.Spec.Proxy, "teleport_proxy", p)
	}

	active := cfg.Status.Active
	skip := deref(cfg.Status.Skip)
	if !cfg.Spec.Enabled {
		u.Log.InfoContext(ctx, "Automatic updates disabled.", activeKey, active)
		return nil
	}

	resp, err := u.find(ctx, cfg)
	if err != nil {
		return trace.Wrap(err)
	}
	target := resp.Target

	if cfg.Spec.Pinned {
		switch target {
		case active:
			u.Log.InfoContext(ctx, "Teleport is up-to-date. Installation is pinned to prevent future updates.", activeKey, active)
		default:
			u.Log.InfoContext(ctx, "Teleport version is pinned. Skipping update.", targetKey, target, activeKey, active)
		}
		return nil
	}

	// If a version fails and is marked skip, we ignore any edition changes as well.
	// If a cluster is broadcasting a version that failed to start, changing ent/fips is unlikely to fix the issue.

	if !resp.InWindow && !now {
		switch {
		case target.Version == "":
			u.Log.WarnContext(ctx, "Cannot determine target agent version. Waiting for both version and update window.")
		case target == active:
			u.Log.InfoContext(ctx, "Teleport is up-to-date. Update window is not active.", activeKey, active)
		case target.Version == skip.Version:
			u.Log.InfoContext(ctx, "Update available, but the new version is marked as broken. Update window is not active.", targetKey, target, activeKey, active)
		default:
			u.Log.InfoContext(ctx, "Update available, but update window is not active.", targetKey, target, activeKey, active)
		}
		return nil
	}

	switch {
	case target.Version == "":
		if resp.InWindow {
			u.Log.ErrorContext(ctx, "Update window is active, but target version is not available.", activeKey, active)
		}
		return trace.Errorf("target version missing")
	case target == active:
		if resp.InWindow {
			u.Log.InfoContext(ctx, "Teleport is up-to-date. Update window is active, but no action is needed.", activeKey, active)
		} else {
			u.Log.InfoContext(ctx, "Teleport is up-to-date. No action is needed.", activeKey, active)
		}
		return nil
	case target.Version == skip.Version:
		u.Log.InfoContext(ctx, "Update available, but the new version is marked as broken. Skipping update.", targetKey, target, activeKey, active)
		return nil
	default:
		u.Log.InfoContext(ctx, "Update available. Initiating update.", targetKey, target, activeKey, active)
	}
	if !now {
		time.Sleep(resp.Jitter)
	}

	updateErr := u.update(ctx, cfg, target, false, resp.AGPL)
	writeErr := writeConfig(u.UpdateConfigPath, cfg)
	if writeErr != nil {
		writeErr = trace.Wrap(writeErr, "failed to write %s", updateConfigName)
	} else {
		u.Log.InfoContext(ctx, "Configuration updated.")
	}
	// Show notices last
	if updateErr == nil && now {
		updateErr = u.notices(ctx)
	}
	return trace.NewAggregate(updateErr, writeErr)
}

func (u *Updater) find(ctx context.Context, cfg *UpdateConfig) (FindResp, error) {
	if cfg.Spec.Proxy == "" {
		return FindResp{}, trace.Errorf("Teleport proxy URL must be specified with --proxy or present in %s", updateConfigName)
	}
	addr, err := libutils.ParseAddr(cfg.Spec.Proxy)
	if err != nil {
		return FindResp{}, trace.Wrap(err, "failed to parse proxy server address")
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
		return FindResp{}, trace.Wrap(err, "failed to request version from proxy")
	}
	var flags autoupdate.InstallFlags
	var agpl bool
	switch resp.Edition {
	case modules.BuildEnterprise:
		flags |= autoupdate.FlagEnterprise
	case modules.BuildCommunity:
	case modules.BuildOSS:
		agpl = true
	default:
		agpl = true
		u.Log.WarnContext(ctx, "Unknown edition detected, defaulting to OSS.", "edition", resp.Edition)
	}
	if resp.FIPS {
		flags |= autoupdate.FlagFIPS
	}
	jitterSec := resp.AutoUpdate.AgentUpdateJitterSeconds
	return FindResp{
		Target:   NewRevision(resp.AutoUpdate.AgentVersion, flags),
		InWindow: resp.AutoUpdate.AgentAutoUpdate,
		Jitter:   time.Duration(jitterSec) * time.Second,
		AGPL:     agpl,
	}, nil
}

func (u *Updater) update(ctx context.Context, cfg *UpdateConfig, target Revision, force, agpl bool) error {
	baseURL := cfg.Spec.BaseURL
	if baseURL == "" {
		if agpl {
			return trace.Errorf("--base-url flag must be specified for AGPL edition of Teleport")
		}
		baseURL = autoupdate.DefaultBaseURL
	}

	active := cfg.Status.Active
	backup := deref(cfg.Status.Backup)
	switch backup {
	case Revision{}, target, active:
	default:
		if target == active {
			// Keep backup version if we are only verifying active version
			break
		}
		err := u.Installer.Remove(ctx, backup)
		if err != nil {
			// this could happen if it was already removed due to a failed installation
			u.Log.WarnContext(ctx, "Failed to remove backup version of Teleport before new install.", errorKey, err, backupKey, backup)
		}
	}

	// Install and link the desired version (or validate existing installation)

	err := u.Installer.Install(ctx, target, baseURL)
	if err != nil {
		return trace.Wrap(err, "failed to install")
	}

	// If the target version has fewer binaries, this will leave old binaries linked.
	// This may prevent the installation from being removed.
	// Cleanup logic at the end of this function will ensure that they are removed
	// eventually.

	revert, err := u.Installer.Link(ctx, target, force)
	if err != nil {
		return trace.Wrap(err, "failed to link")
	}

	// If we fail to revert after this point, the next update/enable will
	// fix the link to restore the active version.

	revertConfig := func(ctx context.Context) bool {
		if target.Version != "" {
			cfg.Status.Skip = toPtr(target)
		}
		if force {
			u.Log.ErrorContext(ctx, "Unable to revert Teleport symlinks in overwrite mode. Installation likely broken.")
			return false
		}
		if ok := revert(ctx); !ok {
			u.Log.ErrorContext(ctx, "Failed to revert Teleport symlinks. Installation likely broken.")
			return false
		}
		if err := u.SetupNamespace(ctx); err != nil {
			u.Log.ErrorContext(ctx, "Failed to revert configuration after failed restart.", errorKey, err)
			return false
		}
		return true
	}

	if cfg.Status.Active != target {
		err := u.ReexecSetup(ctx, true)
		if errors.Is(err, context.Canceled) {
			return trace.Errorf("check canceled")
		}
		if err != nil {
			// If reloading Teleport at the new version fails, revert and reload.
			u.Log.ErrorContext(ctx, "Reverting symlinks due to failed restart.")
			if ok := revertConfig(ctx); ok {
				if err := u.Process.Reload(ctx); err != nil && !errors.Is(err, ErrNotNeeded) {
					u.Log.ErrorContext(ctx, "Failed to reload Teleport after reverting. Installation likely broken.", errorKey, err)
				} else {
					u.Log.WarnContext(ctx, "Teleport updater detected an error with the new installation and successfully reverted it.")
				}
			}
			return trace.Wrap(err, "failed to start new version %s of Teleport", target)
		}
		u.Log.InfoContext(ctx, "Target version successfully installed.", targetKey, target)

		if r := cfg.Status.Active; r.Version != "" {
			cfg.Status.Backup = toPtr(r)
		}
		cfg.Status.Active = target
	} else {
		err := u.ReexecSetup(ctx, false)
		if errors.Is(err, context.Canceled) {
			return trace.Errorf("check canceled")
		}
		if err != nil {
			// If sync fails, we may have left the host in a bad state, so we revert linking and re-Sync.
			u.Log.ErrorContext(ctx, "Reverting symlinks due to invalid configuration.")
			if ok := revertConfig(ctx); ok {
				u.Log.WarnContext(ctx, "Teleport updater encountered a configuration error and successfully reverted the installation.")
			}
			return trace.Wrap(err, "failed to validate new version %s of Teleport", target)
		}
		u.Log.InfoContext(ctx, "Target version successfully validated.", targetKey, target)
	}
	if r := deref(cfg.Status.Backup); r.Version != "" {
		u.Log.InfoContext(ctx, "Backup version set.", backupKey, r)
	}

	return trace.Wrap(u.cleanup(ctx, []Revision{
		target, active, backup,
	}))
}

// Setup writes updater configuration and verifies the Teleport installation.
// If restart is true, Setup also restarts Teleport.
// Setup is safe to run concurrently with other Updater commands.
func (u *Updater) Setup(ctx context.Context, restart bool) error {
	// Setup teleport-updater configuration and sync systemd.

	err := u.SetupNamespace(ctx)
	if errors.Is(err, context.Canceled) {
		return trace.Errorf("sync canceled")
	}
	if errors.Is(err, ErrNotSupported) {
		u.Log.WarnContext(ctx, "Skipping all systemd setup because systemd is not running.")
		return nil
	}
	if err != nil {
		return trace.Wrap(err, "failed to setup updater")
	}

	present, err := u.Process.IsPresent(ctx)
	if errors.Is(err, context.Canceled) {
		return trace.Errorf("config check canceled")
	}
	if err != nil {
		return trace.Wrap(err, "failed to determine if new version of Teleport has an installed systemd service")
	}
	if !present {
		return trace.Errorf("cannot find systemd service for new version of Teleport, check SELinux settings")
	}

	// Restart Teleport if necessary.

	if restart {
		err = u.Process.Reload(ctx)
		if errors.Is(err, context.Canceled) {
			return trace.Errorf("reload canceled")
		}
		if err != nil &&
			!errors.Is(err, ErrNotNeeded) { // skip if not needed
			return trace.Wrap(err, "failed to reload Teleport")
		}
	}
	return nil
}

// notices displays final notices after install or update.
func (u *Updater) notices(ctx context.Context) error {
	enabled, err := u.Process.IsEnabled(ctx)
	if err != nil {
		return trace.Wrap(err, "failed to query Teleport systemd enabled status")
	}
	active, err := u.Process.IsActive(ctx)
	if err != nil {
		return trace.Wrap(err, "failed to query Teleport systemd active status")
	}
	if !enabled && active {
		u.Log.WarnContext(ctx, "Teleport is installed and started, but not configured to start on boot.")
		u.Log.WarnContext(ctx, "After configuring teleport.yaml, you can enable it with: systemctl enable teleport")
	}
	if !active && enabled {
		u.Log.WarnContext(ctx, "Teleport is installed and enabled at boot, but not running.")
		u.Log.WarnContext(ctx, "After configuring teleport.yaml, you can start it with: systemctl start teleport")
	}
	if !active && !enabled {
		u.Log.WarnContext(ctx, "Teleport is installed, but not running or enabled at boot.")
		u.Log.WarnContext(ctx, "After configuring teleport.yaml, you can enable and start it with: systemctl enable teleport --now")
	}
	return nil
}

// cleanup orphan installations
func (u *Updater) cleanup(ctx context.Context, keep []Revision) error {
	revs, err := u.Installer.List(ctx)
	if err != nil {
		u.Log.ErrorContext(ctx, "Failed to read installed versions.", errorKey, err)
		return nil
	}
	if len(revs) < 3 {
		return nil
	}
	u.Log.WarnContext(ctx, "More than two versions of Teleport are installed. Removing unused versions.", "count", len(revs))
	for _, v := range revs {
		if v.Version == "" || slices.Contains(keep, v) {
			continue
		}
		err := u.Installer.Remove(ctx, v)
		if errors.Is(err, ErrLinked) {
			u.Log.WarnContext(ctx, "Refusing to remove version with orphan links.", "version", v)
			continue
		}
		if err != nil {
			u.Log.WarnContext(ctx, "Failed to remove unused version of Teleport.", errorKey, err, "version", v)
			continue
		}
		u.Log.WarnContext(ctx, "Deleted unused version of Teleport.", "version", v)
	}
	return nil
}

// LinkPackage creates links from the system (package) installation of Teleport, if they are needed.
// LinkPackage returns nil and warns if an auto-updates version is already linked, but auto-updates is disabled.
// LinkPackage returns an error only if an unknown version of Teleport is present (e.g., manually copied files).
// This function is idempotent.
func (u *Updater) LinkPackage(ctx context.Context) error {
	cfg, err := readConfig(u.UpdateConfigPath)
	if err != nil {
		return trace.Wrap(err, "failed to read %s", updateConfigName)
	}
	if err := validateConfigSpec(&cfg.Spec, OverrideConfig{}); err != nil {
		return trace.Wrap(err)
	}
	active := cfg.Status.Active
	if cfg.Spec.Enabled {
		u.Log.InfoContext(ctx, "Automatic updates is enabled. Skipping system package link.", activeKey, active)
		return nil
	}
	if cfg.Spec.Pinned {
		u.Log.InfoContext(ctx, "Automatic update version is pinned. Skipping system package link.", activeKey, active)
		return nil
	}
	// If an active version is set, but auto-updates is disabled, try to link the system installation in case the config is stale.
	// If any links are present, this will return ErrLinked and not create any system links.
	// This state is important to log as a warning,
	if err := u.Installer.TryLinkSystem(ctx); errors.Is(err, ErrLinked) {
		u.Log.WarnContext(ctx, "Automatic updates is disabled, but a non-package version of Teleport is linked.", activeKey, active)
		return nil
	} else if errors.Is(err, ErrInvalid) {
		return trace.Wrap(err)
	} else if err != nil {
		return trace.Wrap(err, "failed to link system package installation")
	}

	// If syncing succeeds, ensure the installed systemd service can be found via systemctl.
	// SELinux contexts can interfere with systemctl's ability to read service files.
	if err := u.Process.Sync(ctx); errors.Is(err, ErrNotSupported) {
		u.Log.WarnContext(ctx, "Systemd is not installed. Skipping sync.")
	} else if err != nil {
		return trace.Wrap(err, "failed to sync systemd configuration")
	} else {
		present, err := u.Process.IsPresent(ctx)
		if err != nil {
			return trace.Wrap(err, "failed to determine if Teleport has an installed systemd service")
		}
		if !present {
			return trace.Errorf("cannot find systemd service for Teleport, check SELinux settings")
		}
	}
	u.Log.InfoContext(ctx, "Successfully linked system package installation.")
	return nil
}

// UnlinkPackage removes links from the system (package) installation of Teleport, if they are present.
// This function is idempotent.
func (u *Updater) UnlinkPackage(ctx context.Context) error {
	if err := u.Installer.UnlinkSystem(ctx); err != nil {
		return trace.Wrap(err, "failed to unlink system package installation")
	}
	if err := u.Process.Sync(ctx); errors.Is(err, ErrNotSupported) {
		u.Log.WarnContext(ctx, "Systemd is not installed. Skipping sync.")
	} else if err != nil {
		return trace.Wrap(err, "failed to sync systemd configuration")
	}
	u.Log.InfoContext(ctx, "Successfully unlinked system package installation.")
	return nil
}
