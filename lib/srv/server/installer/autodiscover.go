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

package installer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/safetext/shsprintf"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/automaticupgrades/constants"
	"github.com/gravitational/teleport/lib/automaticupgrades/version"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/client/debug"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/cloud/imds"
	awsimds "github.com/gravitational/teleport/lib/cloud/imds/aws"
	azureimds "github.com/gravitational/teleport/lib/cloud/imds/azure"
	gcpimds "github.com/gravitational/teleport/lib/cloud/imds/gcp"
	oracleimds "github.com/gravitational/teleport/lib/cloud/imds/oracle"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/linux"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/packagemanager"
)

const (
	// defaultJoinCheckDelay is the time to wait after starting the teleport
	// service before checking if the agent successfully joined the cluster.
	defaultJoinCheckDelay = 30 * time.Second

	// defaultInstallLockGracePeriod is additional time beyond joinCheckDelay to
	// wait for the install lock before returning a lock contention error.
	defaultInstallLockGracePeriod = 10 * time.Second

	// defaultReadyzCheckTimeout is the timeout for a single readyz query.
	defaultReadyzCheckTimeout = 10 * time.Second

	// readyzStatusStartingKeyword marks a transient startup status in readyz.
	readyzStatusStartingKeyword = "starting"

	// maxJournalLines is the number of recent journalctl lines to capture.
	maxJournalLines = 50

	// joinHealthCheckAttempts is the number of health checks performed per invocation
	// (initial check + one delayed retry).
	joinHealthCheckAttempts = 2
)

// ErrJoinFailure is returned when the Teleport agent is installed but fails to join the cluster.
var ErrJoinFailure = errors.New("join failure")

var teleportNodeConfigureArgRedactors = map[string]utils.ArgValueRedactor{
	"--token": backend.MaskKeyName,
}

const (
	discoverNotice = "" +
		"Teleport Discover has successfully configured /etc/teleport.yaml for this server to join the cluster.\n" +
		"Discover might replace the configuration if the server fails to join the cluster.\n" +
		"Please remove this file if you are managing this instance using another tool or doing so manually.\n"
)

// AutoDiscoverNodeInstallerConfig installs and configures a Teleport Server into the current system.
type AutoDiscoverNodeInstallerConfig struct {
	Logger *slog.Logger

	// ProxyPublicAddr is the proxy public address that the instance will connect to.
	// Eg, example.platform.sh
	ProxyPublicAddr string

	// InstallationManagedByTeleportUpdateWithSuffix indicates that a suffix was used to install teleport by teleport-update.
	// When set, this command will use non-default file system paths:
	// - teleport binary is located at: /opt/teleport/<suffix>/bin/teleport
	// - systemd unit is located at: /etc/systemd/system/teleport_<suffix>.service
	// - configuration must be written to: /etc/teleport_<suffix>.yaml
	// - data directory must be set to: /var/lib/teleport_<suffix>
	//
	// This is in accordance with the teleport-update behavior.
	InstallationManagedByTeleportUpdateWithSuffix string

	// TeleportPackage contains the teleport package name.
	// Allowed values: teleport, teleport-ent, teleport-ent-fips
	TeleportPackage string

	// RepositoryChannel is the repository channel to use.
	// Eg stable/cloud or stable/rolling
	RepositoryChannel string

	// AutoUpgrades indicates whether the installed binaries should auto upgrade.
	// System must support systemd to enable AutoUpgrades.
	AutoUpgrades           bool
	autoUpgradesChannelURL string

	// AzureClientID is the client ID of the managed identity to use when joining
	// the cluster. Only applicable for the azure join method.
	AzureClientID string

	// TokenName is the token name to be used by the instance to join the cluster.
	TokenName string

	// aptPublicKeyEndpoint contains the URL for the APT public key.
	// Defaults to: https://apt.releases.teleport.dev/gpg
	aptPublicKeyEndpoint string

	// fsRootPrefix is the prefix to use when reading operating system information and when installing teleport.
	// Used for testing.
	fsRootPrefix string

	// binariesLocation contain the location of each required binary.
	// Used for testing.
	binariesLocation packagemanager.BinariesLocation

	// imdsProviders contains the Cloud Instance Metadata providers.
	// Used for testing.
	imdsProviders []func(ctx context.Context) (imds.Client, error)

	// joinCheckDelay is how long to wait after starting the Teleport service before
	// querying the readyz endpoint. Defaults to defaultJoinCheckDelay (30s).
	joinCheckDelay time.Duration

	// installLockWaitTimeout is how long to wait for the install lock before returning a lock
	// contention error. Defaults to joinCheckDelay + defaultInstallLockGracePeriod.
	// Used for testing.
	installLockWaitTimeout time.Duration

	// readyzCheck, when set, replaces the default debug-socket readyz call.
	// Used for testing.
	readyzCheck func(ctx context.Context) (debug.Readiness, error)

	// readyzCheckTimeout overrides the timeout for a single readyz query.
	// Used for testing.
	readyzCheckTimeout time.Duration
}

func (c *AutoDiscoverNodeInstallerConfig) checkAndSetDefaults() error {
	if c == nil {
		return trace.BadParameter("install teleport config is required")
	}

	if c.fsRootPrefix == "" {
		c.fsRootPrefix = "/"
	}

	if c.ProxyPublicAddr == "" {
		return trace.BadParameter("proxy public addr is required")
	}

	if c.Logger == nil {
		c.Logger = slog.Default()
	}

	if c.RepositoryChannel == "" {
		return trace.BadParameter("repository channel is required")
	}

	if !slices.Contains(types.PackageNameKinds, c.TeleportPackage) {
		return trace.BadParameter("teleport-package must be one of %+v", types.PackageNameKinds)
	}

	if c.AutoUpgrades && c.TeleportPackage == types.PackageNameOSS {
		return trace.BadParameter("only enterprise package supports auto upgrades")
	}

	if c.AutoUpgrades && c.TeleportPackage == types.PackageNameEntFIPS {
		return trace.BadParameter("auto upgrades are not supported in FIPS environments")
	}

	if c.autoUpgradesChannelURL == "" {
		c.autoUpgradesChannelURL = "https://" + c.ProxyPublicAddr + "/v1/webapi/automaticupgrades/channel/default"
	}

	if c.InstallationManagedByTeleportUpdateWithSuffix != "" {
		// Only update it if not already set by the caller (only tests set it).
		if c.binariesLocation.Teleport == "" {
			c.binariesLocation.Teleport = filepath.Join(c.fsRootPrefix, "opt", "teleport", c.InstallationManagedByTeleportUpdateWithSuffix, "bin", "teleport")
		}
	}

	c.binariesLocation.CheckAndSetDefaults()

	if c.joinCheckDelay == 0 {
		c.joinCheckDelay = defaultJoinCheckDelay
	}

	if c.installLockWaitTimeout == 0 {
		c.installLockWaitTimeout = c.joinCheckDelay + defaultInstallLockGracePeriod
	}

	if len(c.imdsProviders) == 0 {
		c.imdsProviders = []func(ctx context.Context) (imds.Client, error){
			func(ctx context.Context) (imds.Client, error) {
				clt, err := awsimds.NewInstanceMetadataClient(ctx)
				return clt, trace.Wrap(err)
			},
			func(ctx context.Context) (imds.Client, error) {
				return azureimds.NewInstanceMetadataClient(), nil
			},
			func(ctx context.Context) (imds.Client, error) {
				instancesClient, err := gcp.NewInstancesClient(ctx)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				clt, err := gcpimds.NewInstanceMetadataClient(instancesClient)
				return clt, trace.Wrap(err)
			},
			func(ctx context.Context) (imds.Client, error) {
				return oracleimds.NewInstanceMetadataClient(), nil
			},
		}
	}

	return nil
}

// AutoDiscoverNodeInstaller will install teleport in the current system.
// It's meant to be used by the Server Auto Discover script.
type AutoDiscoverNodeInstaller struct {
	*AutoDiscoverNodeInstallerConfig
}

// NewAutoDiscoverNodeInstaller returns a new AutoDiscoverNodeInstaller.
func NewAutoDiscoverNodeInstaller(cfg *AutoDiscoverNodeInstallerConfig) (*AutoDiscoverNodeInstaller, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	ti := &AutoDiscoverNodeInstaller{
		AutoDiscoverNodeInstallerConfig: cfg,
	}

	return ti, nil
}

const (
	// exclusiveInstallFileLock is the name of the lockfile to be used when installing teleport.
	// Used for the default installers (see api/types/installers/{agentless,}installer.sh.tmpl/).
	exclusiveInstallFileLock = "/var/lock/teleport_install.lock"

	// etcOSReleaseFile is the location of the OS Release information.
	// This is valid for most linux distros, that rely on systemd.
	etcOSReleaseFile = "/etc/os-release"

	// teleportYamlConfigNewExtension is the extension used to indicate that this is a new target teleport.yaml version
	teleportYamlConfigNewExtension = ".new"
)

var imdsClientTypeToJoinMethod = map[types.InstanceMetadataType]types.JoinMethod{
	types.InstanceMetadataTypeAzure: types.JoinMethodAzure,
	types.InstanceMetadataTypeEC2:   types.JoinMethodIAM,
	types.InstanceMetadataTypeGCP:   types.JoinMethodGCP,
}

// Install teleport in the current system.
func (ani *AutoDiscoverNodeInstaller) Install(ctx context.Context) error {
	if ani.InstallationManagedByTeleportUpdateWithSuffix != "" {
		slog.InfoContext(ctx, "Using non-default path for teleport installation",
			"suffix", ani.InstallationManagedByTeleportUpdateWithSuffix,
			"teleport_binary", ani.binariesLocation.Teleport,
			"systemd_unit", ani.buildTeleportSystemdUnitName(),
			"configuration_file", ani.buildTeleportConfigurationPath(),
			"data_directory", ani.buildTeleportDataDirPath(),
		)
	}

	// Install, configure, and health-check all run under the install lock.
	return trace.Wrap(ani.installAndConfigure(ctx))
}

// installAndConfigure acquires the install lock and performs the install, configuration,
// and health-check steps. The health check runs under the lock to prevent a concurrent
// installer from restarting Teleport while we're waiting for the readyz result.
func (ani *AutoDiscoverNodeInstaller) installAndConfigure(ctx context.Context) error {
	// Ensure only one installer is running by locking the same file as the script installers.
	lockFile := ani.buildAbsoluteFilePath(exclusiveInstallFileLock)
	unlockFn, err := utils.FSTryWriteLockTimeout(ctx, lockFile, ani.installLockWaitTimeout)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			return trace.BadParameter("Could not get lock %s. Either remove it or wait for the other installer to finish.", lockFile)
		}
		return trace.Wrap(err, "acquiring install lock %s", lockFile)
	}
	defer func() {
		if err := unlockFn(); err != nil {
			ani.Logger.WarnContext(ctx, "Failed to remove lock. Please remove it manually.", "file", lockFile)
		}
	}()

	imdsClient, err := ani.getIMDSClient(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	ani.Logger.InfoContext(ctx, "Detected cloud provider", "cloud", imdsClient.GetType())

	// Check if teleport is already installed and install it, if it's absent.
	// In the new autoupdate install flow, teleport-update should have already
	// taken care of installing teleport.
	if _, err := os.Stat(ani.binariesLocation.Teleport); err != nil {
		// If teleport is not present and the installation is managed by teleport-update,
		// then this is an error because teleport-update should have installed it.
		// This prevents the installer from installing teleport in a different version and/or location than the one managed by teleport-update.
		if ani.InstallationManagedByTeleportUpdateWithSuffix != "" {
			return trace.BadParameter("teleport binary not found, ensure teleport-update installed it correctly: %v", err)
		}

		ani.Logger.InfoContext(ctx, "Installing teleport")
		if err := ani.installTeleportFromRepo(ctx); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := ani.configureTeleportNode(ctx, imdsClient); err != nil {
		if trace.IsAlreadyExists(err) {
			ani.Logger.InfoContext(ctx, "Teleport configuration already exists and has the same values, skipping systemd service restart",
				"configuration_file", ani.buildTeleportConfigurationPath(),
				"systemd_service", ani.buildTeleportSystemdUnitName(),
			)
			// Config unchanged, so skip restart but still run a health check. This preserves visibility into
			// lingering join/service failures on subsequent polls.
			return trace.Wrap(ani.checkJoinHealth(ctx))
		}

		return trace.Wrap(err)
	}
	ani.Logger.InfoContext(ctx, "Configuration written",
		"configuration_file", ani.buildTeleportConfigurationPath(),
	)

	ani.Logger.InfoContext(ctx, "Enabling and starting teleport service",
		"systemd_service", ani.buildTeleportSystemdUnitName(),
	)
	if err := ani.enableAndRestartTeleportService(ctx); err != nil {
		return trace.Wrap(err)
	}

	// Health check runs while the install lock is still held, so another installer flow that
	// uses this lock can't restart Teleport while we're waiting for the readyz result.
	return trace.Wrap(ani.checkJoinHealth(ctx))
}

// checkJoinHealth checks whether the Teleport service is running and has joined the cluster.
// It checks immediately first (in case the service is already up from a previous run), and
// only waits and retries if a delay can make the result more conclusive.
func (a *AutoDiscoverNodeInstaller) checkJoinHealth(ctx context.Context) error {
	serviceName := a.buildTeleportSystemdUnitName()

	// First attempt: check immediately. If the service is already running and the
	// debug socket is available, we get a fast answer without waiting.
	result := a.doJoinHealthCheck(ctx, serviceName)
	if result.definitive {
		return result.err
	}

	if a.joinCheckDelay > 0 {
		a.Logger.InfoContext(ctx, "Agent not ready yet, waiting before retry", "delay", a.joinCheckDelay)
		timer := time.NewTimer(a.joinCheckDelay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		}
	}

	result = a.doJoinHealthCheck(ctx, serviceName)
	if result.err != nil {
		return result.err
	}

	if !result.definitive && result.starting {
		// Still "starting" after the wait (the agent never joined). This is a real join failure.
		status := result.startingStatus
		if status == "" {
			status = "unknown"
		}
		msg := fmt.Sprintf("readyz remained in starting state after %s wait (attempts=%d, status=%q)", a.joinCheckDelay, joinHealthCheckAttempts, status)
		err := trace.Wrap(ErrJoinFailure, msg)
		return a.appendJournalWithJoinFailureHint(ctx, serviceName, err)
	}

	if !result.definitive && result.serviceTransition {
		msg := fmt.Sprintf("service %s remained in transition after %s wait (attempts=%d, state=%q)", serviceName, a.joinCheckDelay, joinHealthCheckAttempts, result.serviceState)
		err := trace.Wrap(ErrJoinFailure, msg)
		return a.appendJournalWithJoinFailureHint(ctx, serviceName, err)
	}

	if !result.definitive {
		// Service is active but readyz remained unreachable across both attempts.
		// Treat this as a join failure instead of silently succeeding forever.
		msg := fmt.Sprintf("readyz socket remained unavailable after %s wait (attempts=%d)", a.joinCheckDelay, joinHealthCheckAttempts)
		err := trace.Wrap(ErrJoinFailure, msg)
		return a.appendJournalWithJoinFailureHint(ctx, serviceName, err)
	}

	return nil
}

// healthCheckResult holds the outcome of a join health check.
type healthCheckResult struct {
	err error
	// definitive is true when the check reached a conclusive result (success or failure).
	// It is false when the debug socket was not available or the agent was still starting,
	// meaning a retry after a delay may produce a different result.
	definitive bool
	// starting is true when the non-definitive result came from a "starting" readyz status.
	// Distinguished from socket-absent so the caller can emit a more specific message when
	// "starting" persists after retry.
	starting bool
	// startingStatus stores the readyz status string when starting is true.
	startingStatus string
	// serviceTransition is true when systemd reported the service in a transient state
	// (for example "activating" or "deactivating") and a delayed retry may resolve it.
	serviceTransition bool
	// serviceState stores the systemd state when serviceTransition is true.
	serviceState string
}

// doJoinHealthCheck verifies the systemd unit is active and queries the readyz endpoint.
func (a *AutoDiscoverNodeInstaller) doJoinHealthCheck(ctx context.Context, serviceName string) healthCheckResult {
	// Check service status before readyz: a dead process can't serve the debug socket.
	serviceStatus := a.checkServiceStatus(ctx, serviceName)
	if serviceStatus.err != nil {
		return healthCheckResult{
			err:        a.appendJournalWithJoinFailureHint(ctx, serviceName, serviceStatus.err),
			definitive: true,
		}
	}
	if serviceStatus.transition {
		return healthCheckResult{
			definitive:        false,
			serviceTransition: true,
			serviceState:      serviceStatus.state,
		}
	}

	reachable, starting, readyzStatus, err := a.checkReadyz(ctx)
	if err != nil {
		return healthCheckResult{
			err:        a.appendJournalWithJoinFailureHint(ctx, serviceName, err),
			definitive: true,
		}
	}
	if starting {
		// Process is up but still starting (not definitive, retry after delay).
		return healthCheckResult{
			definitive:     false,
			starting:       true,
			startingStatus: readyzStatus,
		}
	}
	if !reachable {
		// Socket not available (not a definitive result).
		return healthCheckResult{definitive: false}
	}

	return healthCheckResult{definitive: true}
}

type serviceStatusResult struct {
	state      string
	transition bool
	err        error
}

// checkServiceStatus performs a systemd status check for the named unit.
//
// It returns:
// - transition=true for transient states like activating/deactivating/reloading;
// - err!=nil for recognized terminal non-active states and command execution failures;
// - zero-value for active/inconclusive cases.
func (a *AutoDiscoverNodeInstaller) checkServiceStatus(ctx context.Context, serviceName string) serviceStatusResult {
	cmd := exec.CommandContext(ctx, a.binariesLocation.Systemctl, "is-active", serviceName)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return serviceStatusResult{err: trace.Wrap(ctxErr)}
		}

		// A non-ExitError means the command itself failed to run (binary not
		// found, permission denied, etc.) rather than reporting a non-active state.
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			stderrStr := strings.TrimSpace(stderr.String())
			return serviceStatusResult{
				err: trace.Wrap(
					ErrJoinFailure,
					"unable to check service %s status via systemctl: %v (stderr: %s)",
					serviceName,
					err,
					stderrStr,
				),
			}
		}
	}

	// ExitError: systemctl ran but the service is not active.
	// stdout is captured in the buffer regardless of exit code.
	state := strings.TrimSpace(stdout.String())
	stderrStr := strings.TrimSpace(stderr.String())

	// systemctl is-active normally emits a state string (active, inactive, failed, etc.).
	// Empty output is unexpected; log and let the caller fall through to readyz.
	if state == "" {
		a.Logger.WarnContext(ctx, "systemctl is-active produced no output",
			"service", serviceName,
			"stderr", stderrStr,
		)
		return serviceStatusResult{}
	}

	if isServiceTransitionState(state) {
		a.Logger.InfoContext(ctx, "Service is transitioning",
			"service", serviceName,
			"state", state,
		)
		return serviceStatusResult{state: state, transition: true}
	}

	if state != "active" {
		return serviceStatusResult{
			err: trace.Wrap(
				ErrJoinFailure,
				"systemd reported service %s is not active (state: %q, stderr: %s)",
				serviceName,
				state,
				stderrStr,
			),
		}
	}

	a.Logger.DebugContext(ctx, "Service is active", "service", serviceName)
	return serviceStatusResult{}
}

func isServiceTransitionState(state string) bool {
	switch state {
	case "activating", "deactivating", "reloading":
		return true
	default:
		return false
	}
}

// checkReadyz queries the Teleport debug socket's readyz endpoint to determine whether the agent
// has joined the cluster. It applies a timeout to avoid hanging on a wedged process.
//
// Return values:
//   - (true, false, status, nil)   — agent is ready
//   - (true, true,  status, nil)   — socket reachable but agent still starting (transient)
//   - (true, false, "",    error)  — definitive readyz failure
//   - (false, false, "",    nil)   — debug socket unavailable
func (a *AutoDiscoverNodeInstaller) checkReadyz(ctx context.Context) (reachable, starting bool, status string, err error) {
	ctx, cancel := context.WithTimeout(ctx, a.getReadyzCheckTimeout())
	defer cancel()

	var readiness debug.Readiness
	if a.readyzCheck != nil {
		readiness, err = a.readyzCheck(ctx)
	} else {
		clt := debug.NewClient(a.buildTeleportDataDirPath())
		readiness, err = clt.GetReadiness(ctx)
	}
	if err != nil {
		// Socket genuinely absent (connection error) or endpoint doesn't exist (signal "not reachable" so the caller can retry).
		if isConnectionError(err) || trace.IsNotFound(err) {
			a.Logger.DebugContext(ctx, "Debug socket unavailable", "error", trace.UserMessage(err))
			return false, false, "", nil
		}
		return true, false, "", trace.Wrap(ErrJoinFailure, "readyz check failed: %v", err)
	}

	if readiness.Ready {
		a.Logger.InfoContext(ctx, "Teleport agent is ready and has joined the cluster")
		return true, false, readiness.Status, nil
	}

	status = readiness.Status

	// "starting" is transient: the process is up but hasn't joined yet. Signal the caller to retry
	// after a delay instead of treating this as a definitive failure.
	if strings.Contains(status, readyzStatusStartingKeyword) {
		a.Logger.InfoContext(ctx, "Teleport agent is still starting", "status", status)
		return true, true, status, nil
	}

	return true, false, "", trace.Wrap(ErrJoinFailure, "readyz reported not ready: %s", status)
}

func (a *AutoDiscoverNodeInstaller) getReadyzCheckTimeout() time.Duration {
	if a.readyzCheckTimeout > 0 {
		return a.readyzCheckTimeout
	}

	return defaultReadyzCheckTimeout
}

// isConnectionError returns true when the error indicates a socket-level connection
// failure (e.g. file not found, connection refused), as opposed to an HTTP or
// application-level error.
func isConnectionError(err error) bool {
	var opErr *net.OpError
	return errors.As(err, &opErr)
}

// appendJournal enriches err with recent service log lines for the given systemd unit.
// If no output is available, the original error is returned unchanged.
func (a *AutoDiscoverNodeInstaller) appendJournal(ctx context.Context, serviceName string, err error) error {
	journalOutput, captureErr := a.captureJournal(ctx, serviceName)
	if captureErr != nil {
		return trace.Wrap(captureErr)
	}
	return appendJournalOutput(err, journalOutput)
}

// appendJournalWithJoinFailureHint augments join-failure errors with a concise user-facing hint when journal output
// contains known token-expiry signals, then appends the captured journal output for diagnostics.
func (a *AutoDiscoverNodeInstaller) appendJournalWithJoinFailureHint(ctx context.Context, serviceName string, err error) error {
	journalOutput, captureErr := a.captureJournal(ctx, serviceName)
	if captureErr != nil {
		return trace.Wrap(captureErr)
	}
	err = appendJoinFailureHint(err, journalOutput)
	return appendJournalOutput(err, journalOutput)
}

func appendJournalOutput(err error, journalOutput string) error {
	if journalOutput == "" {
		return err
	}
	return trace.Wrap(err, "\n\nJournal output:\n%s", journalOutput)
}

func joinFailureHintFromJournal(journalOutput string) string {
	lower := strings.ToLower(journalOutput)
	if strings.Contains(lower, "token is expired or not found") || strings.Contains(lower, "token expired or not found") {
		return "token is expired or not found"
	}

	return ""
}

func appendJoinFailureHint(err error, journalOutput string) error {
	if !errors.Is(err, ErrJoinFailure) {
		return err
	}

	hint := joinFailureHintFromJournal(journalOutput)
	if hint == "" {
		return err
	}

	userMessage := trace.UserMessage(err)
	if strings.Contains(strings.ToLower(userMessage), hint) {
		return err
	}
	baseMessage := strings.TrimSpace(strings.TrimPrefix(userMessage, ErrJoinFailure.Error()+": "))
	if baseMessage == "" {
		baseMessage = strings.TrimSpace(userMessage)
	}

	return trace.Wrap(err, "%s: %s; %s", ErrJoinFailure.Error(), hint, baseMessage)
}

func isSystemdInvocationID(value string) bool {
	value = strings.ReplaceAll(value, "-", "")
	if len(value) != 32 {
		return false
	}

	_, err := hex.DecodeString(value)
	return err == nil
}

func buildJournalctlArgs(serviceName, invocationID string) []string {
	args := []string{
		"--unit", serviceName,
		"--no-pager",
		"--lines", fmt.Sprintf("%d", maxJournalLines),
	}

	if invocationID != "" {
		args = append(args, "_SYSTEMD_INVOCATION_ID="+invocationID)
	}

	return args
}

func (a *AutoDiscoverNodeInstaller) getServiceInvocationID(ctx context.Context, serviceName string) (string, error) {
	cmd := exec.CommandContext(ctx, a.binariesLocation.Systemctl, "show", serviceName, "--property", "InvocationID", "--value")
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	stdout := strings.TrimSpace(stdoutBuf.String())
	stderr := strings.TrimSpace(stderrBuf.String())
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", trace.Wrap(ctxErr)
		}

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			a.Logger.DebugContext(ctx, "systemctl show exited non-zero while retrieving service invocation ID",
				"service", serviceName,
				"exit_code", exitErr.ExitCode(),
				"stdout", stdout,
				"stderr", stderr,
			)
			return "", nil
		}

		a.Logger.DebugContext(ctx, "Could not retrieve service invocation ID", "service", serviceName, "error", err, "stdout", stdout, "stderr", stderr)
		return "", nil
	}

	invocationID := stdout
	if invocationID == "" || strings.EqualFold(invocationID, "n/a") {
		return "", nil
	}

	if !isSystemdInvocationID(invocationID) {
		a.Logger.DebugContext(ctx, "Ignoring invalid service invocation ID", "service", serviceName, "invocation_id", invocationID)
		return "", nil
	}

	return invocationID, nil
}

// captureJournal is a best-effort helper that runs journalctl to retrieve recent log
// lines for the given systemd unit.
// Stderr is logged internally but not returned, to keep caller-facing diagnostics
// focused on journal contents.
func (a *AutoDiscoverNodeInstaller) captureJournal(ctx context.Context, serviceName string) (string, error) {
	invocationID, err := a.getServiceInvocationID(ctx, serviceName)
	if err != nil {
		return "", trace.Wrap(err)
	}
	args := buildJournalctlArgs(serviceName, invocationID)

	cmd := exec.CommandContext(ctx, a.binariesLocation.Journalctl, args...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	stderrOutput := strings.TrimSpace(stderrBuf.String())
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", trace.Wrap(ctxErr)
		}

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			a.Logger.DebugContext(ctx, "journalctl exited non-zero", "service", serviceName, "exit_code", exitErr.ExitCode(), "stderr", stderrOutput)
		} else {
			a.Logger.WarnContext(ctx, "Failed to capture journal output", "service", serviceName, "error", err, "stderr", stderrOutput)
		}
	}

	return strings.TrimSpace(stdoutBuf.String()), nil
}

// enableAndRestartTeleportService will enable and (re)start the teleport.service.
// This function must be idempotent because we can call it in either one of the following scenarios:
// - teleport was just installed and teleport.service is inactive
// - teleport was already installed but the service is failing
func (ani *AutoDiscoverNodeInstaller) enableAndRestartTeleportService(ctx context.Context) error {
	serviceName := ani.buildTeleportSystemdUnitName()

	systemctlEnableNowCMD := exec.CommandContext(ctx, ani.binariesLocation.Systemctl, "enable", serviceName)
	systemctlEnableNowCMDOutput, err := systemctlEnableNowCMD.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(systemctlEnableNowCMDOutput))
	}

	systemctlRestartCMD := exec.CommandContext(ctx, ani.binariesLocation.Systemctl, "restart", serviceName)
	systemctlRestartCMDOutput, err := systemctlRestartCMD.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(systemctlRestartCMDOutput))
	}

	return nil
}

func (ani *AutoDiscoverNodeInstaller) configureTeleportNode(ctx context.Context, imdsClient imds.Client) error {
	nodeLabels, err := fetchNodeAutoDiscoverLabels(ctx, imdsClient)
	if err != nil {
		return trace.Wrap(err)
	}

	// The last step is to configure the `teleport.yaml`.
	// We could do this using the github.com/gravitational/teleport/lib/config package.
	// However, that would cause the configuration to use the current running binary which is different from the binary that was just installed.
	// That could cause problems if the versions are not compatible.
	// To prevent creating an invalid configuration, the installed binary must be used.

	labelEntries := make([]string, 0, len(nodeLabels))
	for labelKey, labelValue := range nodeLabels {
		labelEntries = append(labelEntries, labelKey+"="+labelValue)
	}
	sort.Strings(labelEntries)
	nodeLabelsCommaSeperated := strings.Join(labelEntries, ",")

	joinMethod, ok := imdsClientTypeToJoinMethod[imdsClient.GetType()]
	if !ok {
		return trace.BadParameter("Unsupported cloud provider: %v", imdsClient.GetType())
	}

	dataDirForNode := ani.buildTeleportDataDirPath()

	teleportYamlConfigurationPath := ani.buildTeleportConfigurationPath()
	teleportYamlConfigurationPathNew := teleportYamlConfigurationPath + teleportYamlConfigNewExtension

	teleportNodeConfigureArgs := []string{"node", "configure", "--output=file://" + teleportYamlConfigurationPathNew,
		fmt.Sprintf(`--data-dir=%s`, shsprintf.EscapeDefaultContext(dataDirForNode)),
		fmt.Sprintf(`--proxy=%s`, shsprintf.EscapeDefaultContext(ani.ProxyPublicAddr)),
		fmt.Sprintf(`--join-method=%s`, shsprintf.EscapeDefaultContext(string(joinMethod))),
		fmt.Sprintf(`--token=%s`, shsprintf.EscapeDefaultContext(ani.TokenName)),
		fmt.Sprintf(`--labels=%s`, shsprintf.EscapeDefaultContext(nodeLabelsCommaSeperated)),
	}
	if ani.AzureClientID != "" {
		teleportNodeConfigureArgs = append(teleportNodeConfigureArgs,
			fmt.Sprintf(`--azure-client-id=%s`, shsprintf.EscapeDefaultContext(ani.AzureClientID)))
	}

	ani.Logger.InfoContext(ctx,
		"Generating teleport configuration",
		"teleport", ani.binariesLocation.Teleport,
		"args", utils.RedactFlagArgs(teleportNodeConfigureArgs, teleportNodeConfigureArgRedactors),
	)
	teleportNodeConfigureCmd := exec.CommandContext(ctx, ani.binariesLocation.Teleport, teleportNodeConfigureArgs...)
	teleportNodeConfigureCmdOutput, err := teleportNodeConfigureCmd.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(teleportNodeConfigureCmdOutput))
	}

	defer func() {
		// If an error occurs before the os.Rename, let's remove the `.new` file to prevent any leftovers.
		// Error is ignored because the file might be already removed.
		_ = os.Remove(teleportYamlConfigurationPathNew)
	}()

	discoverNoticeFile := teleportYamlConfigurationPath + ".discover"
	// Check if file already exists and has the same content that we are about to write
	if _, err := os.Stat(teleportYamlConfigurationPath); err == nil {
		hashExistingFile, err := checksum(teleportYamlConfigurationPath)
		if err != nil {
			return trace.Wrap(err)
		}

		hashNewFile, err := checksum(teleportYamlConfigurationPathNew)
		if err != nil {
			return trace.Wrap(err)
		}

		if hashExistingFile == hashNewFile {
			if err := os.WriteFile(discoverNoticeFile, []byte(discoverNotice), 0o644); err != nil {
				return trace.Wrap(err)
			}

			return trace.AlreadyExists("teleport.yaml is up to date")
		}

		// If a previous /etc/teleport.yaml configuration file exists and is different from the target one, it might be because one of the following reasons:
		// - discover installation params (eg token name) were changed
		// - `$ teleport node configure` command produces a different output
		// - teleport was manually installed / configured
		//
		// For the first two scenarios, it's fine, and even desired in most cases, to restart teleport with the new configuration.
		//
		// However, for the last scenario (teleport was manually installed), this flow must not replace the currently running teleport service configuration.
		// To prevent this, this flow checks for the existence of the discover notice file, and only allows replacement if it does exist.
		if _, err := os.Stat(discoverNoticeFile); err != nil {
			ani.Logger.InfoContext(ctx, "Refusing to replace the existing teleport configuration. For the script to replace the existing configuration remove the Teleport configuration.",
				"teleport_configuration", teleportYamlConfigurationPath,
				"discover_notice_file", discoverNoticeFile)
			return trace.BadParameter("missing discover notice file")
		}
	}

	if err := os.Rename(teleportYamlConfigurationPathNew, teleportYamlConfigurationPath); err != nil {
		return trace.Wrap(err)
	}

	if err := os.WriteFile(discoverNoticeFile, []byte(discoverNotice), 0o644); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func checksum(filename string) (string, error) {
	f, err := utils.OpenFileAllowingUnsafeLinks(filename)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", trace.Wrap(err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func (ani *AutoDiscoverNodeInstaller) installTeleportFromRepo(ctx context.Context) error {
	// Read current system information.
	linuxInfo, err := ani.linuxDistribution()
	if err != nil {
		return trace.Wrap(err)
	}

	ani.Logger.InfoContext(ctx, "Operating system detected.",
		"id", linuxInfo.ID,
		"id_like", linuxInfo.IDLike,
		"codename", linuxInfo.VersionCodename,
		"version_id", linuxInfo.VersionID,
	)

	packageManager, err := packagemanager.PackageManagerForSystem(linuxInfo, ani.fsRootPrefix, ani.binariesLocation, ani.aptPublicKeyEndpoint)
	if err != nil {
		return trace.Wrap(err)
	}

	targetVersion := ""
	var packagesToInstall []packagemanager.PackageVersion
	if ani.AutoUpgrades {
		teleportAutoUpdaterPackage := ani.TeleportPackage + "-updater"

		ani.Logger.InfoContext(ctx, "Auto-upgrade enabled: fetching target version", "auto_upgrade_endpoint", ani.autoUpgradesChannelURL)
		targetVersion = ani.fetchTargetVersion(ctx)

		// No target version advertised.
		if targetVersion == constants.NoVersion {
			targetVersion = ""
		}
		ani.Logger.InfoContext(ctx, "Using teleport version", "version", targetVersion)
		packagesToInstall = append(packagesToInstall, packagemanager.PackageVersion{Name: teleportAutoUpdaterPackage, Version: targetVersion})
	}
	packagesToInstall = append(packagesToInstall, packagemanager.PackageVersion{Name: ani.TeleportPackage, Version: targetVersion})

	if err := packageManager.AddTeleportRepository(ctx, linuxInfo, ani.RepositoryChannel); err != nil {
		return trace.BadParameter("failed to add teleport repository to system: %v", err)
	}
	if err := packageManager.InstallPackages(ctx, packagesToInstall); err != nil {
		return trace.BadParameter("failed to install teleport: %v", err)
	}

	return nil
}

func (ani *AutoDiscoverNodeInstaller) getIMDSClient(ctx context.Context) (imds.Client, error) {
	// detect and fetch cloud provider metadata
	imdsClient, err := cloud.DiscoverInstanceMetadata(ctx, ani.imdsProviders)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.BadParameter("Auto Discover only runs on Cloud instances with IMDS/Metadata service enabled. Ensure the service is running and try again.")
		}
		return nil, trace.Wrap(err)
	}

	return imdsClient, nil
}

func (ani *AutoDiscoverNodeInstaller) fetchTargetVersion(ctx context.Context) string {
	upgradeURL, err := url.Parse(ani.autoUpgradesChannelURL)
	if err != nil {
		ani.Logger.WarnContext(ctx, "Failed to parse automatic upgrades default channel url, using api version",
			"channel_url", ani.autoUpgradesChannelURL,
			"error", err,
			"version", api.Version)
		return api.Version
	}

	// TODO(hugoShaka): convert this to a proxy version getter
	targetVersion, err := version.NewBasicHTTPVersionGetter(upgradeURL).GetVersion(ctx)
	if err != nil {
		ani.Logger.WarnContext(ctx, "Failed to query target version, using api version",
			"error", err,
			"version", api.Version)
		return api.Version
	}
	ani.Logger.InfoContext(ctx, "Found target version",
		"channel_url", ani.autoUpgradesChannelURL,
		"version", targetVersion)

	return strings.TrimSpace(strings.TrimPrefix(targetVersion.String(), "v"))
}

func fetchNodeAutoDiscoverLabels(ctx context.Context, imdsClient imds.Client) (map[string]string, error) {
	nodeLabels := make(map[string]string)

	switch imdsClient.GetType() {
	case types.InstanceMetadataTypeAzure:
		azureIMDSClient, ok := imdsClient.(*azureimds.InstanceMetadataClient)
		if !ok {
			return nil, trace.BadParameter("failed to obtain azure imds client")
		}

		instanceInfo, err := azureIMDSClient.GetInstanceInfo(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		nodeLabels[types.SubscriptionIDLabel] = instanceInfo.SubscriptionID
		nodeLabels[types.VMIDLabel] = instanceInfo.VMID
		nodeLabels[types.RegionLabel] = instanceInfo.Location
		nodeLabels[types.ResourceGroupLabel] = instanceInfo.ResourceGroupName

	case types.InstanceMetadataTypeEC2:
		awsIMDSClient, ok := imdsClient.(*awsimds.InstanceMetadataClient)
		if !ok {
			return nil, trace.BadParameter("failed to obtain ec2 imds client")
		}
		accountID, err := awsIMDSClient.GetAccountID(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		instanceID, err := awsIMDSClient.GetID(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		nodeLabels[types.AWSInstanceIDLabel] = instanceID
		nodeLabels[types.AWSAccountIDLabel] = accountID

	case types.InstanceMetadataTypeGCP:
		gcpIMDSClient, ok := imdsClient.(*gcpimds.InstanceMetadataClient)
		if !ok {
			return nil, trace.BadParameter("failed to obtain gcp imds client")
		}

		name, err := gcpIMDSClient.GetName(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		zone, err := gcpIMDSClient.GetZone(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		projectID, err := gcpIMDSClient.GetProjectID(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		nodeLabels[types.NameLabelDiscovery] = name
		nodeLabels[types.ZoneLabelDiscovery] = zone
		nodeLabels[types.ProjectIDLabelDiscovery] = projectID
		nodeLabels[types.ProjectIDLabel] = projectID

	default:
		return nil, trace.BadParameter("Unsupported cloud provider: %v", imdsClient.GetType())
	}

	return nodeLabels, nil
}

// buildAbsoluteFilePath creates the absolute file path
func (ani *AutoDiscoverNodeInstaller) buildAbsoluteFilePath(path string) string {
	return filepath.Join(ani.fsRootPrefix, path)
}

func (ani *AutoDiscoverNodeInstaller) buildTeleportConfigurationPath() string {
	filePath := defaults.ConfigFilePath
	if ani.InstallationManagedByTeleportUpdateWithSuffix != "" {
		fileName := "teleport_" + ani.InstallationManagedByTeleportUpdateWithSuffix + ".yaml"
		filePath = filepath.Join(filepath.Dir(defaults.ConfigFilePath), fileName)
	}
	return ani.buildAbsoluteFilePath(filePath)
}

func (ani *AutoDiscoverNodeInstaller) buildTeleportDataDirPath() string {
	dataDirPath := defaults.DataDir
	if ani.InstallationManagedByTeleportUpdateWithSuffix != "" {
		dataDirName := "teleport_" + ani.InstallationManagedByTeleportUpdateWithSuffix
		dataDirPath = filepath.Join(filepath.Dir(defaults.DataDir), dataDirName)
	}
	return ani.buildAbsoluteFilePath(dataDirPath)
}

func (ani *AutoDiscoverNodeInstaller) buildTeleportSystemdUnitName() string {
	if ani.InstallationManagedByTeleportUpdateWithSuffix != "" {
		return "teleport_" + ani.InstallationManagedByTeleportUpdateWithSuffix
	}

	return "teleport"
}

// linuxDistribution reads the current file system to detect the Linux Distro and Version of the current system.
//
// https://www.freedesktop.org/software/systemd/man/latest/os-release.html
func (ani *AutoDiscoverNodeInstaller) linuxDistribution() (*linux.OSRelease, error) {
	f, err := os.Open(ani.buildAbsoluteFilePath(etcOSReleaseFile))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer f.Close()

	osRelease, err := linux.ParseOSReleaseFromReader(f)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return osRelease, nil
}
