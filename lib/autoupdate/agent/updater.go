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
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"io/fs"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/autoupdate"
	"github.com/gravitational/teleport/lib/client/debug"
	libdefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	libutils "github.com/gravitational/teleport/lib/utils"
)

const (
	// BinaryName specifies the name of the updater binary.
	BinaryName = "teleport-update"
)

const (
	// packageSystemDir is the location where packaged Teleport binaries and services are installed.
	packageSystemDir = "/opt/teleport/system"
	// reservedFreeDisk is the minimum required free space left on disk during downloads.
	// TODO(sclevine): This value is arbitrary and could be replaced by, e.g., min(1%, 200mb) in the future
	//   to account for a range of disk sizes.
	reservedFreeDisk = 10_000_000
	// debugSocketFileName is the name of Teleport's debug socket in the data dir.
	debugSocketFileName = "debug.sock" // 10 MB
	// requiredUmask must be set before this package can be used.
	// Use syscall.Umask to set when no other goroutines are running.
	requiredUmask = 0o022
	// teleportHostIDFileName is the name of Teleport's host ID file.
	teleportHostIDFileName = "host_uuid"
	// systemdMachineIDFile is a path containing a machine-unique identifier created by systemd.
	systemdMachineIDFile = "/etc/machine-id"
)

// Log keys
const (
	targetKey = "target_version"
	activeKey = "active_version"
	backupKey = "backup_version"
	errorKey  = "error"
)

var (
	initTime = time.Now().UTC()
)

// SetRequiredUmask sets the umask to match the systemd umask that the teleport-update service will execute with.
// This ensures consistent file permissions.
// NOTE: This must be run in main.go before any goroutines that create files are started.
func SetRequiredUmask(ctx context.Context, log *slog.Logger) {
	warnUmask(ctx, log, syscall.Umask(requiredUmask))
}

func warnUmask(ctx context.Context, log *slog.Logger, old int) {
	if old&^requiredUmask != 0 {
		log.WarnContext(ctx, "Restrictive umask detected. Umask has been changed to 0022 for teleport-update and all child processes.")
		log.WarnContext(ctx, "All files created by teleport-update will have permissions set according to this umask.")
	}
}

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
		cfg.SystemDir = packageSystemDir
	}
	validator := Validator{Log: cfg.Log}
	debugClient := debug.NewClient(filepath.Join(ns.dataDir, debugSocketFileName))
	return &Updater{
		Log:                 cfg.Log,
		Pool:                certPool,
		InsecureSkipVerify:  cfg.InsecureSkipVerify,
		UpdateConfigFile:    filepath.Join(ns.Dir(), updateConfigName),
		UpdateIDFile:        ns.updaterIDFile,
		MachineIDFile:       systemdMachineIDFile,
		TeleportIDFile:      filepath.Join(ns.dataDir, teleportHostIDFileName),
		TeleportConfigFile:  ns.configFile,
		TeleportServiceName: filepath.Base(ns.serviceFile),
		DefaultProxyAddr:    ns.defaultProxyAddr,
		DefaultPathDir:      ns.defaultPathDir,
		Installer: &LocalInstaller{
			InstallDir:              filepath.Join(ns.Dir(), versionsDirName),
			TargetServiceFile:       ns.serviceFile,
			SystemBinDir:            filepath.Join(cfg.SystemDir, "bin"),
			SystemServiceFile:       filepath.Join(cfg.SystemDir, serviceDir, serviceName),
			HTTP:                    client,
			Log:                     cfg.Log,
			ReservedFreeTmpDisk:     reservedFreeDisk,
			ReservedFreeInstallDisk: reservedFreeDisk,
			TransformService:        ns.ReplaceTeleportService,
			ValidateBinary:          validator.IsBinary,
			Template:                autoupdate.DefaultCDNURITemplate,
		},
		Process: &SystemdService{
			ServiceName: filepath.Base(ns.serviceFile),
			PIDFile:     ns.pidFile,
			Ready:       debugClient,
			Log:         cfg.Log,
		},
		ReexecSetup: func(ctx context.Context, pathDir string, reload bool) error {
			name := filepath.Join(pathDir, BinaryName)
			if cfg.SelfSetup && runtime.GOOS == constants.LinuxOS {
				name = "/proc/self/exe"
			}
			args := []string{
				"--install-dir", ns.installDir,
				"--install-suffix", ns.name,
				"--log-format", cfg.LogFormat,
			}
			if cfg.Debug {
				args = append(args, "--debug")
			}
			args = append(args, "setup", "--path", pathDir)
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
		LogConfigWarnings: ns.LogWarnings,
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
// SetRequiredUmask must be called before any methods are executed, except for Status.
type Updater struct {
	// Log contains a logger.
	Log *slog.Logger
	// Pool used for requests to the Teleport web API.
	Pool *x509.CertPool
	// InsecureSkipVerify skips TLS verification.
	InsecureSkipVerify bool
	// UpdateConfigFile contains the path to the agent auto-updates configuration.
	UpdateConfigFile string
	// UpdateIDFile contains the path to the ID used to track the Teleport agent during updates.
	// This ID is written and read by the Teleport agent and used to schedule progressive updates.
	UpdateIDFile string
	// MachineIDFile contains the path to a system-unique ID file.
	MachineIDFile string
	// TeleportIDFile contains the path to Teleport's host ID.
	TeleportIDFile string
	// TeleportConfigFile contains the path to Teleport's configuration.
	TeleportConfigFile string
	// TeleportServiceName contains the full name of the systemd service for Teleport
	TeleportServiceName string
	// DefaultProxyAddr contains Teleport's proxy address. This may differ from the updater's.
	DefaultProxyAddr string
	// DefaultPathDir contains the default path that Teleport binaries should be installed into.
	DefaultPathDir string
	// Installer manages installations of the Teleport agent.
	Installer Installer
	// Process manages a running instance of Teleport.
	Process Process
	// ReexecSetup re-execs teleport-update with the setup command.
	// This configures the updater service, verifies the installation, and optionally reloads Teleport.
	ReexecSetup func(ctx context.Context, path string, reload bool) error
	// SetupNamespace configures the Teleport updater service for the current Namespace.
	SetupNamespace func(ctx context.Context, path string) error
	// TeardownNamespace removes all traces of the updater service in the current Namespace, including Teleport.
	TeardownNamespace func(ctx context.Context) error
	// LogConfigWarnings logs warnings related to the configuration Namespace.
	LogConfigWarnings func(ctx context.Context, pathDir string)
}

// Installer provides an API for installing Teleport agents.
type Installer interface {
	// Install the Teleport agent at revision from the download Template.
	// If force is true, Install will remove broken revisions.
	// Install must be idempotent.
	Install(ctx context.Context, rev Revision, baseURL string, force bool) error
	// Link the Teleport agent at the specified revision of Teleport into path.
	// The revert function must restore the previous linking, returning false on any failure.
	// If force is true, Link will overwrite non-symlinks.
	// Link must be idempotent. Link's revert function must be idempotent.
	Link(ctx context.Context, rev Revision, pathDir string, force bool) (revert func(context.Context) bool, err error)
	// LinkSystem links the system installation of Teleport into the system linking location.
	// The revert function must restore the previous linking, returning false on any failure.
	// LinkSystem must be idempotent. LinkSystem's revert function must be idempotent.
	LinkSystem(ctx context.Context) (revert func(context.Context) bool, err error)
	// TryLink links the specified revision of Teleport into path.
	// Unlike Link, TryLink will fail if existing links to other locations are present.
	// TryLink must be idempotent.
	TryLink(ctx context.Context, rev Revision, pathDir string) error
	// TryLinkSystem links the system (package) installation of Teleport into the system linking location.
	// Unlike LinkSystem, TryLinkSystem will fail if existing links to other locations are present.
	// TryLinkSystem must be idempotent.
	TryLinkSystem(ctx context.Context) error
	// Unlink unlinks the specified revision of Teleport from path.
	// Unlink must be idempotent.
	Unlink(ctx context.Context, rev Revision, pathDir string) error
	// UnlinkSystem unlinks the system (package) installation of Teleport from the system linking location.
	// UnlinkSystem must be idempotent.
	UnlinkSystem(ctx context.Context) error
	// List the installed revisions of Teleport.
	List(ctx context.Context) (revisions []Revision, err error)
	// Remove the Teleport agent at revision.
	// Remove must be idempotent.
	Remove(ctx context.Context, rev Revision) error
	// IsLinked returns true if the revision is linked to path.
	IsLinked(ctx context.Context, rev Revision, pathDir string) (bool, error)
}

var (
	// ErrLinked is returned when a linked version cannot be operated on.
	ErrLinked = errors.New("version is linked")
	// ErrNotNeeded is returned when the operation is not needed.
	ErrNotNeeded = errors.New("not needed")
	// ErrNotSupported is returned when the operation is not supported on the platform.
	ErrNotSupported = errors.New("not supported on this platform")
	// ErrNotAvailable is returned when the operation is not available at the current version of the platform.
	ErrNotAvailable = errors.New("not available at this version")
	// ErrNoBinaries is returned when no binaries are available to be linked.
	ErrNoBinaries = errors.New("no binaries available to link")
	// ErrFilePresent is returned when a file is present.
	ErrFilePresent = errors.New("file present")
	// ErrNotInstalled is returned when Teleport is not installed.
	ErrNotInstalled = errors.New("not installed")
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

	// The fields below override the behavior of
	// Updater.Install for a single run.

	// ForceVersion to the specified version.
	ForceVersion string
	// ForceFlags in installed Teleport.
	ForceFlags autoupdate.InstallFlags
	// AllowOverwrite of installed binaries.
	AllowOverwrite bool
	// AllowProxyConflict when proxies in teleport.yaml and update.yaml are mismatched.
	AllowProxyConflict bool
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
	cfg, err := readConfig(u.UpdateConfigFile)
	if err != nil {
		return trace.Wrap(err, "failed to read %s", updateConfigName)
	}
	if err := validateConfigSpec(&cfg.Spec, override); err != nil {
		return trace.Wrap(err)
	}

	if cfg.Spec.Proxy == "" {
		cfg.Spec.Proxy = u.DefaultProxyAddr
	} else if u.DefaultProxyAddr != "" &&
		!sameProxies(cfg.Spec.Proxy, u.DefaultProxyAddr) &&
		!override.AllowProxyConflict {
		u.Log.ErrorContext(ctx, "Proxy specified in update.yaml does not match teleport.yaml.", "update_proxy", cfg.Spec.Proxy, "teleport_proxy", u.DefaultProxyAddr)
		return trace.Errorf("refusing to install with conflicting proxy addresses, pass --allow-proxy-conflict to override")
	}
	if cfg.Spec.Path == "" {
		cfg.Spec.Path = u.DefaultPathDir
	}
	cfg.Status.IDFile = u.UpdateIDFile

	active := cfg.Status.Active
	skip := deref(cfg.Status.Skip)

	// Lookup target version from the proxy.

	resp, err := u.find(ctx, cfg, u.readID(ctx))
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
			u.Log.ErrorContext(ctx, "A non-packaged or outdated installation of Teleport was detected on this system.")
			u.Log.ErrorContext(ctx, "Use --overwrite to force immediate removal of any existing binaries installed manually or via script.")
			u.Log.ErrorContext(ctx, "Alternatively, if a Teleport RPM or DEB package is installed, upgrade it to the latest version and retry without --overwrite.")
		}
		return trace.Wrap(err)
	}
	if target.Version == skip.Version {
		cfg.Status.Skip = nil
	}

	// Only write the configuration file if the initial update succeeds.
	// Note: skip_version is never set on failed enable, only failed update.

	if err := writeConfig(u.UpdateConfigFile, cfg); err != nil {
		return trace.Wrap(err, "failed to write %s", updateConfigName)
	}
	u.Log.InfoContext(ctx, "Configuration updated.")
	u.LogConfigWarnings(ctx, cfg.Spec.Path)
	u.notices(ctx)
	return nil
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
func (u *Updater) Remove(ctx context.Context, force bool) error {
	cfg, err := readConfig(u.UpdateConfigFile)
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

	// Do not link system package installation if the installation we are removing
	// is not installed into /usr/local/bin. In this case, we also need to make sure
	// it is clear we are not going to recover the package's systemd service if it
	// was overwritten.
	if filepath.Clean(cfg.Spec.Path) != filepath.Clean(defaultPathDir) {
		if u.TeleportServiceName == serviceName {
			if !force {
				u.Log.ErrorContext(ctx, "Default Teleport systemd service would be removed, and --force was not passed.")
				u.Log.ErrorContext(ctx, "Refusing to remove Teleport from this system.")
				return trace.Errorf("unable to remove Teleport completely without --force")
			} else {
				u.Log.WarnContext(ctx, "Default Teleport systemd service will be removed since --force was passed.")
				u.Log.WarnContext(ctx, "Teleport will be removed from this system.")
			}
		}
		return u.removeWithoutSystem(ctx, cfg)
	}
	revert, err := u.Installer.LinkSystem(ctx)
	if errors.Is(err, ErrNoBinaries) {
		if !force {
			u.Log.ErrorContext(ctx, "No packaged installation of Teleport was found, and --force was not passed.")
			u.Log.ErrorContext(ctx, "Refusing to remove Teleport from this system entirely without --force.")
			return trace.Errorf("unable to remove Teleport completely without --force")
		} else {
			u.Log.WarnContext(ctx, "No packaged installation of Teleport was found, but --force was passed.")
			u.Log.WarnContext(ctx, "Teleport will be removed from this system entirely.")
		}
		return u.removeWithoutSystem(ctx, cfg)
	}
	if err != nil {
		return trace.Wrap(err, "failed to link")
	}

	u.Log.InfoContext(ctx, "Updater-managed installation of Teleport detected.")
	u.Log.InfoContext(ctx, "Restoring packaged version of Teleport before removing.")

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
				u.Log.ErrorContext(ctx, "Failed to reload Teleport after reverting.", errorKey, err)
				u.Log.ErrorContext(ctx, "Installation likely broken.")
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

func (u *Updater) removeWithoutSystem(ctx context.Context, cfg *UpdateConfig) error {
	u.Log.InfoContext(ctx, "Updater-managed installation of Teleport detected.")
	u.Log.InfoContext(ctx, "Attempting to unlink and remove.")
	ok, err := u.Process.IsActive(ctx)
	if err != nil && !errors.Is(err, ErrNotSupported) {
		return trace.Wrap(err)
	}
	if ok {
		return trace.Errorf("refusing to remove active installation of Teleport, please stop and disable Teleport first")
	}
	if err := u.Installer.Unlink(ctx, cfg.Status.Active, cfg.Spec.Path); err != nil {
		return trace.Wrap(err)
	}
	u.Log.InfoContext(ctx, "Teleport uninstalled.", "version", cfg.Status.Active)
	if err := u.TeardownNamespace(ctx); err != nil {
		return trace.Wrap(err)
	}
	u.Log.InfoContext(ctx, "Automatic update configuration for Teleport successfully uninstalled.")
	return nil
}

// readID generates a DBPID based on both the systemd machine ID and Teleport host ID.
// This reduces the chance that multiple hosts will have the same updater ID, while
// allowing both the teleport and teleport-update binaries to deterministically derive
// the same value. This also avoids issues caused by non-UUID values in host_uuid,
// and ensures that updaters without running agents have a unique value.
// The ID must be persisted to ensure it does not change between Teleport updates.
// Only the Teleport Agent may persist the ID, as it may start first at system boot and must
// know the value immediately when it starts.
// Errors will be logged.
func (u *Updater) readID(ctx context.Context) string {
	tid, err := os.ReadFile(u.TeleportIDFile)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		u.Log.WarnContext(ctx, "Failed to read Teleport host ID.", "path", u.TeleportIDFile, errorKey, err)
	}
	mid, err := os.ReadFile(u.MachineIDFile)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		u.Log.WarnContext(ctx, "Failed to read systemd machine ID.", "path", u.MachineIDFile, errorKey, err)
	}
	out, err := FindDBPID(u.UpdateIDFile, bytes.TrimSpace(mid), bytes.TrimSpace(tid), false)
	if err != nil {
		u.Log.ErrorContext(ctx, "Unable to generate unique ID for this host.", errorKey, err)
		u.Log.ErrorContext(ctx, "The Teleport agent may not be tracked, and may fail if used as a canary.")
		return ""
	}
	return out
}

func readIfPresent(path string) string {
	idBytes, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(bytes.TrimSpace(idBytes))
}

// FindDBPID returns a deterministic boot-persistent identifier (DBPID).
// This is a sha256-based UUIDv5 that is regenerated at each boot using a
// machine-derived identifier and a namespace identifier.
// DBPIDs are stable across reboots as long as their identifiers remain the
// same and the logic implementing the ID generation remains the same.
// This reduces the risk that the ID changes unexpectedly when a new version
// of the process in launched, because both the system must be rebooted
// and the IDs (or logic) must change for the DBPID to change.
// ASN.1 is used to support binary identifiers and to ensure stability across
// versions of the Go standard library.
//
// It is safe for other code to read path directly to determine the cached DBPID.
// FindDBPID may be called concurrently on the same path with persist set to false.
// Calls with persist set to true are not reentrant.
func FindDBPID(path string, systemID, namespaceID []byte, persist bool) (string, error) {
	idBytes, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", trace.Wrap(err)
	}
	if s := bytes.TrimSpace(idBytes); err == nil && len(s) > 0 {
		return string(s), nil
	}
	id, err := generateDBPID(systemID, namespaceID)
	if err != nil {
		return id, trace.Wrap(err)
	}
	if persist {
		err = writeFileAtomicWithinDir(path, []byte(id), configFileMode)
	}
	return id, trace.Wrap(err)
}

func generateDBPID(systemID, namespaceID []byte) (string, error) {
	/*
		--- ASN.1 Schema

		DBPID DEFINITIONS  ::=  BEGIN
		    DBPID  ::=  SEQUENCE  {
		        systemID     OCTET STRING,
		        namespaceID  OCTET STRING
		    }
		END
	*/

	// changing this struct will change all hashes
	obj := struct {
		SID  []byte
		NSID []byte
	}{
		SID:  systemID,
		NSID: namespaceID,
	}
	if len(obj.SID) == 0 && len(obj.NSID) == 0 {
		return "", trace.BadParameter("all provided IDs are empty")
	}
	der, err := asn1.Marshal(obj)
	if err != nil {
		return "", trace.Wrap(err)
	}
	// this only uses the first 16 bytes of the sha256 hash, which is acceptable
	// for uniquely identify agents connected to a cluster
	return uuid.NewHash(sha256.New(), uuid.Nil, der, 5).String(), nil
}

// Status returns all available local and remote fields related to agent auto-updates.
// Status is safe to run concurrently with other Updater commands.
// Status does not write files, and therefore does not require SetRequiredUmask.
func (u *Updater) Status(ctx context.Context) (Status, error) {
	var out Status
	// Read configuration from update.yaml.
	cfg, err := readConfig(u.UpdateConfigFile)
	if err != nil {
		return out, trace.Wrap(err, "failed to read %s", updateConfigName)
	}
	if err := validateConfigSpec(&cfg.Spec, OverrideConfig{}); err != nil {
		return out, trace.Wrap(err)
	}
	if cfg.Spec.Proxy == "" {
		return out, trace.Wrap(ErrNotInstalled)
	}
	out.UpdateSpec = cfg.Spec
	out.UpdateStatus = cfg.Status

	// Lookup target version from the proxy.
	out.ID = readIfPresent(cfg.Status.IDFile)
	resp, err := u.find(ctx, cfg, out.ID)
	if err != nil {
		return out, trace.Wrap(err)
	}
	out.FindResp = resp
	out.IDFile = ""
	return out, nil
}

// Disable disables agent auto-updates.
// This function is idempotent.
func (u *Updater) Disable(ctx context.Context) error {
	cfg, err := readConfig(u.UpdateConfigFile)
	if err != nil {
		return trace.Wrap(err, "failed to read %s", updateConfigName)
	}
	if !cfg.Spec.Enabled {
		u.Log.InfoContext(ctx, "Automatic updates already disabled.")
		return nil
	}
	cfg.Spec.Enabled = false
	if err := writeConfig(u.UpdateConfigFile, cfg); err != nil {
		return trace.Wrap(err, "failed to write %s", updateConfigName)
	}
	return nil
}

// Unpin allows the current version to be changed by Update.
// This function is idempotent.
func (u *Updater) Unpin(ctx context.Context) error {
	cfg, err := readConfig(u.UpdateConfigFile)
	if err != nil {
		return trace.Wrap(err, "failed to read %s", updateConfigName)
	}
	if !cfg.Spec.Pinned {
		u.Log.InfoContext(ctx, "Current version not pinned.", activeKey, cfg.Status.Active)
		return nil
	}
	cfg.Spec.Pinned = false
	if err := writeConfig(u.UpdateConfigFile, cfg); err != nil {
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
	cfg, err := readConfig(u.UpdateConfigFile)
	if err != nil {
		return trace.Wrap(err, "failed to read %s", updateConfigName)
	}
	if err := validateConfigSpec(&cfg.Spec, OverrideConfig{}); err != nil {
		return trace.Wrap(err)
	}

	active := cfg.Status.Active
	skip := deref(cfg.Status.Skip)
	if !cfg.Spec.Enabled {
		u.Log.InfoContext(ctx, "Automatic updates disabled.", activeKey, active)
		return nil
	}

	if u.DefaultProxyAddr != "" &&
		!sameProxies(cfg.Spec.Proxy, u.DefaultProxyAddr) {
		u.Log.WarnContext(ctx, "Proxy specified in update.yaml does not match teleport.yaml.", "update_proxy", cfg.Spec.Proxy, "teleport_proxy", u.DefaultProxyAddr)
		u.Log.WarnContext(ctx, "Unexpected updates may occur.")
	}

	if cfg.Spec.Path == "" {
		return trace.Errorf("failed to read destination path for binary links from %s", updateConfigName)
	}
	cfg.Status.IDFile = u.UpdateIDFile

	resp, err := u.find(ctx, cfg, u.readID(ctx))
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
	if !now && resp.Jitter > 0 {
		select {
		case <-time.After(time.Duration(rand.Int64N(int64(resp.Jitter)))):
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		}
	}
	cfg.Status.LastUpdate = &LastUpdate{
		Time:   initTime.Truncate(time.Millisecond),
		Target: target,
	}
	updateErr := u.update(ctx, cfg, target, false, resp.AGPL)
	if updateErr == nil {
		cfg.Status.LastUpdate.Success = true
	}
	writeErr := writeConfig(u.UpdateConfigFile, cfg)
	if writeErr != nil {
		writeErr = trace.Wrap(writeErr, "failed to write %s", updateConfigName)
	} else {
		u.Log.InfoContext(ctx, "Configuration updated.")
	}
	// Show notices last
	if updateErr == nil && now {
		u.notices(ctx)
	}
	return trace.NewAggregate(updateErr, writeErr)
}

func (u *Updater) find(ctx context.Context, cfg *UpdateConfig, id string) (FindResp, error) {
	if cfg.Spec.Proxy == "" {
		return FindResp{}, trace.Errorf("Teleport proxy URL must be specified with --proxy or present in %s", updateConfigName)
	}
	addr, err := libutils.ParseAddr(cfg.Spec.Proxy)
	if err != nil {
		return FindResp{}, trace.Wrap(err, "failed to parse proxy server address")
	}
	group := cfg.Spec.Group
	if group == "" {
		group = "default"
	}
	resp, err := webclient.Find(&webclient.Config{
		Context:     ctx,
		ProxyAddr:   addr.Addr,
		Insecure:    u.InsecureSkipVerify,
		Timeout:     30 * time.Second,
		UpdateGroup: group,
		UpdateID:    id,
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

func (u *Updater) removeRevision(ctx context.Context, cfg *UpdateConfig, rev Revision) error {
	linked, err := u.Installer.IsLinked(ctx, rev, cfg.Spec.Path)
	if err != nil {
		return trace.Wrap(err, "failed to determine if linked")
	}
	if linked {
		return trace.Wrap(ErrLinked, "refusing to remove")
	}
	return trace.Wrap(u.Installer.Remove(ctx, rev))
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
		err := u.removeRevision(ctx, cfg, backup)
		if err != nil {
			// this could happen if it was already removed due to a failed installation
			u.Log.WarnContext(ctx, "Failed to remove backup version of Teleport before new install.", errorKey, err, backupKey, backup)
		}
	}

	// Install and link the desired version (or validate existing installation)

	linked, err := u.Installer.IsLinked(ctx, target, cfg.Spec.Path)
	if err != nil {
		return trace.Wrap(err, "failed to determine if linked")
	}
	err = u.Installer.Install(ctx, target, baseURL, !linked)
	if err != nil {
		return trace.Wrap(err, "failed to install")
	}

	// If the target version has fewer binaries, this will leave old binaries linked.
	// This may prevent the installation from being removed.
	// Cleanup logic at the end of this function will ensure that they are removed
	// eventually.

	revert, err := u.Installer.Link(ctx, target, cfg.Spec.Path, force)
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
		if err := u.SetupNamespace(ctx, cfg.Spec.Path); err != nil {
			u.Log.ErrorContext(ctx, "Failed to revert configuration after failed restart.", errorKey, err)
			return false
		}
		return true
	}

	if cfg.Status.Active != target {
		err := u.ReexecSetup(ctx, cfg.Spec.Path, true)
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
		err := u.ReexecSetup(ctx, cfg.Spec.Path, false)
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
	u.cleanup(ctx, cfg, []Revision{
		target, active, backup,
	})
	return nil
}

// Setup writes updater configuration and verifies the Teleport installation.
// If restart is true, Setup also restarts Teleport.
// Setup is safe to run concurrently with other Updater commands.
func (u *Updater) Setup(ctx context.Context, path string, restart bool) error {
	// Setup teleport-updater configuration and sync systemd.

	err := u.SetupNamespace(ctx, path)
	if errors.Is(err, context.Canceled) {
		return trace.Errorf("sync canceled")
	}
	if err != nil {
		return trace.Wrap(err, "failed to setup updater")
	}

	present, err := u.Process.IsPresent(ctx)
	if errors.Is(err, context.Canceled) {
		return trace.Errorf("config check canceled")
	}
	if errors.Is(err, ErrNotSupported) {
		u.Log.WarnContext(ctx, "Skipping all systemd setup because systemd is not running.")
		return nil
	}
	if errors.Is(err, ErrNotAvailable) {
		u.Log.DebugContext(ctx, "Systemd version is outdated. Skipping SELinux verification.")
	} else if err != nil {
		return trace.Wrap(err, "failed to determine if new version of Teleport has an installed systemd service")
	} else if !present {
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
func (u *Updater) notices(ctx context.Context) {
	enabled, err := u.Process.IsEnabled(ctx)
	if errors.Is(err, ErrNotSupported) {
		u.Log.WarnContext(ctx, "Teleport is installed, but systemd is not present to start it.")
		u.Log.WarnContext(ctx, "After configuring teleport.yaml, your system must also be configured to start Teleport.")
		return
	}
	if errors.Is(err, ErrNotAvailable) {
		u.Log.WarnContext(ctx, "Remember to use systemctl to enable and start Teleport.")
		return
	}
	if err != nil {
		u.Log.ErrorContext(ctx, "Failed to determine if Teleport is enabled.", errorKey, err)
		return
	}
	active, err := u.Process.IsActive(ctx)
	if err != nil {
		u.Log.ErrorContext(ctx, "Failed to determine if Teleport is active.", errorKey, err)
		return
	}
	if !enabled && active {
		u.Log.WarnContext(ctx, "Teleport is installed and started, but not configured to start on boot.")
		u.Log.WarnContext(ctx, "After configuring teleport.yaml, you must enable it.",
			"command", "systemctl enable "+u.TeleportServiceName)
	}
	if !active && enabled {
		u.Log.WarnContext(ctx, "Teleport is installed and enabled at boot, but not running.")
		u.Log.WarnContext(ctx, "After configuring teleport.yaml, you must start it.",
			"command", "systemctl start "+u.TeleportServiceName)
	}
	if !active && !enabled {
		u.Log.WarnContext(ctx, "Teleport is installed, but not running or enabled at boot.")
		u.Log.WarnContext(ctx, "After configuring teleport.yaml, you must enable and start.",
			"command", "systemctl enable --now "+u.TeleportServiceName)
	}
}

// cleanup orphan installations
func (u *Updater) cleanup(ctx context.Context, cfg *UpdateConfig, keep []Revision) {
	revs, err := u.Installer.List(ctx)
	if err != nil {
		u.Log.ErrorContext(ctx, "Failed to read installed versions.", errorKey, err)
		return
	}
	if len(revs) < 3 {
		return
	}
	u.Log.WarnContext(ctx, "More than two versions of Teleport are installed. Removing unused versions.", "count", len(revs))
	for _, v := range revs {
		if v.Version == "" || slices.Contains(keep, v) {
			continue
		}
		err := u.removeRevision(ctx, cfg, v)
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
}

// LinkPackage creates links from the system (package) installation of Teleport, if they are needed.
// LinkPackage returns nil and warns if an auto-updates version is already linked, but auto-updates is disabled.
// LinkPackage returns an error only if an unknown version of Teleport is present (e.g., manually copied files).
// This function is idempotent.
func (u *Updater) LinkPackage(ctx context.Context) error {
	cfg, err := readConfig(u.UpdateConfigFile)
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
		if errors.Is(err, ErrNotAvailable) {
			u.Log.DebugContext(ctx, "Systemd version is outdated. Skipping SELinux verification.")
		} else if err != nil {
			return trace.Wrap(err, "failed to determine if Teleport has an installed systemd service")
		} else if !present {
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
