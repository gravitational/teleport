// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package hoststep

import (
	"context"
	"errors"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"go.yaml.in/yaml/v3"

	"github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/hostid"
)

// promotedUnit is the unit the promoted agent runs as.
const promotedUnit = "teleport.service"

// PromoteRequest is what the promote step needs.
type PromoteRequest struct {
	// Proxy is the target cluster's proxy address.
	Proxy string `json:"proxy"`
	// JoinMethod is how the promoted agent joins the target cluster.
	JoinMethod string `json:"join_method"`
	// TokenName is the token's scope and name; for the token method the name
	// carries the secret.
	TokenName scopes.QualifiedName `json:"token_name"`
	// RegistrationSecret is set for the bound_keypair method only.
	RegistrationSecret string `json:"registration_secret,omitempty"`
	// CAPins are the target cluster's CA pins.
	CAPins []string `json:"ca_pins"`
	// SourceHostUUID is the host_uuid of the source agent the migration
	// replaced, which labels both the scoped and the promoted agent.
	SourceHostUUID string `json:"source_host_uuid"`
	// Wait is how long to wait for another install's lock on the host, and
	// then for the promoted agent to write its host_uuid.
	Wait time.Duration `json:"wait"`
	// Abort stops and disables the promoted agent an earlier promote step of
	// this migration started, and installs nothing.
	Abort bool `json:"abort,omitempty"`
	// RecordOnly is set when the step runs through an agent that was promoted
	// by hand. That agent is the host's only one, so the step only returns its
	// host_uuid; it installs, stops and restarts nothing.
	RecordOnly bool `json:"record_only,omitempty"`
}

// PromoteRefusal is how promote refuses a host it cannot promote safely. The
// checks return only the reason; the promote step and its caller put this
// advice before it, so the advice is written in one place.
const PromoteRefusal = "refusing to promote this host automatically; promote it by hand, or clear what is in the way and promote again"

// promote installs a second agent on the default paths beside the scoped
// agent, as apply installed the scoped agent beside the source one, and
// returns its host_uuid. The scoped agent keeps running: the caller removes it
// with the uninstall step once the promoted agent has joined. The step:
//   - checks the host and renders the promoted config (preparePromote)
//   - installs and starts the promoted agent (installPromoted)
//   - waits for its host_uuid (waitPromotedHostUUID)
//
// A refusal from preparePromote changes nothing on the host. Any failure after
// the install starts stops the promoted agent again before the step returns.
// req.Abort and req.RecordOnly run only stopPromoted and promotedByHand.
func (h hostEnv) promote(ctx context.Context, req PromoteRequest) (InstallResponse, error) {
	unlock, err := utils.FSTryWriteLockTimeout(ctx, h.path(scopedConfigPath+".lock"), req.Wait)
	if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
		return InstallResponse{}, trace.AlreadyExists("another migrate command is still installing on this host after %s", req.Wait)
	}
	if err != nil {
		return InstallResponse{}, trace.Wrap(err, "locking %q", scopedConfigPath+".lock")
	}
	defer unlock()

	switch {
	case req.RecordOnly:
		resp, err := h.promotedByHand(ctx, req)
		return resp, trace.Wrap(err, PromoteRefusal)
	case req.Abort:
		return InstallResponse{}, trace.Wrap(h.stopPromoted(ctx, req))
	}
	promoted, baseURL, err := h.preparePromote(ctx, req)
	if err != nil {
		return InstallResponse{}, trace.Wrap(err, PromoteRefusal)
	}
	if err := h.installPromoted(ctx, req, promoted, baseURL); err != nil {
		return InstallResponse{}, trace.Wrap(err)
	}
	id, err := h.waitPromotedHostUUID(ctx, req)
	return InstallResponse{HostUUID: id}, trace.Wrap(err)
}

// preparePromote refuses a host promote cannot install on safely, and
// otherwise returns the promoted config and the download base URL to install
// with. It changes nothing. The rules:
//   - the scoped agent is running, from a scoped config that names this
//     migration's proxy and source host, by the tool's label or by the one
//     teleport reconfigure sets for a host migrated by hand
//   - the default paths are free (checkDefaultPathsFree)
//   - the scoped install's teleport-update is there to install with
//   - the promoted config can be rendered (renderPromotedConfig)
func (h hostEnv) preparePromote(ctx context.Context, req PromoteRequest) (promoted, baseURL string, err error) {
	if err := h.checkScopedServes(ctx, req); err != nil {
		return "", "", trace.Wrap(err)
	}
	if err := h.checkDefaultPathsFree(ctx, req); err != nil {
		return "", "", trace.Wrap(err)
	}
	if !isExecutable(h.path(scopedUpdaterPath)) {
		return "", "", trace.NotFound("no scoped install at %s", scopedInstallDir)
	}
	scoped, err := h.readAgentConfig(scopedConfigPath)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	if promoted, err = renderPromotedConfig(scoped, req); err != nil {
		return "", "", trace.Wrap(err, "rendering the promoted agent's config")
	}
	if baseURL, err = h.scopedBaseURL(); err != nil {
		return "", "", trace.Wrap(err)
	}
	return promoted, baseURL, nil
}

// checkScopedServes refuses unless this migration's scoped agent is running.
// The promote step reaches the host through that agent, and it is what keeps
// serving when the promoted agent is stopped again.
func (h hostEnv) checkScopedServes(ctx context.Context, req PromoteRequest) error {
	if !h.exists(scopedConfigPath) {
		return trace.NotFound("the scoped config %s is gone", scopedConfigPath)
	}
	if err := h.checkConfigOwner("scoped", scopedConfigPath, req.Proxy, req.SourceHostUUID, "", true); err != nil {
		return trace.Wrap(err)
	}
	active, err := unitActive(ctx, scopedUnit)
	if err != nil {
		return trace.Wrap(err)
	}
	if !active {
		return trace.CompareFailed("%s is not running", scopedUnit)
	}
	return nil
}

// installPromoted writes promoted to /etc/teleport.yaml, installs the default
// installation from baseURL with the scoped install's teleport-update, and
// starts teleport.service. --overwrite is passed because the default paths may
// still hold the retired source agent's binaries from a tarball install. If
// the install or the start fails, it stops the promoted agent again
// (stopAfterFailure).
func (h hostEnv) installPromoted(ctx context.Context, req PromoteRequest, promoted, baseURL string) error {
	if err := os.MkdirAll(h.path(defaults.DataDir), 0o700); err != nil {
		return trace.ConvertSystemError(err)
	}
	if err := writePrivate(h.path(defaults.ConfigFilePath), []byte(promoted)); err != nil {
		return trace.Wrap(err, "writing %q", defaults.ConfigFilePath)
	}
	enable := []string{h.path(scopedUpdaterPath), "enable", "--proxy", req.Proxy, "--overwrite"}
	if baseURL != "" {
		enable = append(enable, "--base-url", baseURL)
	}
	for _, args := range [][]string{
		enable,
		{"systemctl", "enable", promotedUnit},
		{"systemctl", "restart", promotedUnit},
	} {
		if err := runCommand(ctx, args[0], args[1:]...); err != nil {
			return h.stopAfterFailure(ctx, req, trace.Wrap(err))
		}
	}
	if err := runCommand(ctx, "systemctl", "is-active", "--quiet", promotedUnit); err != nil {
		return h.stopAfterFailure(ctx, req, trace.Wrap(err, "%s is not running; check journalctl -u %s", promotedUnit, promotedUnit))
	}
	return nil
}

// waitPromotedHostUUID waits up to req.Wait for the promoted agent to write
// its host_uuid and returns it. If none appears, it stops the promoted agent
// again. The retry's own error is only "no host_uuid yet" or the end of the
// wait, which the returned error already says.
func (h hostEnv) waitPromotedHostUUID(ctx context.Context, req PromoteRequest) (string, error) {
	retry, err := retryutils.NewConstant(time.Second)
	if err != nil {
		return "", trace.Wrap(err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, req.Wait)
	defer cancel()
	var id string
	err = retry.For(waitCtx, func() error {
		if id = h.readTrimmed(path.Join(defaults.DataDir, hostid.FileName)); id == "" {
			return trace.NotFound("no host_uuid yet")
		}
		return nil
	})
	if err == nil {
		return id, nil
	}
	return "", h.stopAfterFailure(ctx, req, trace.LimitExceeded("%s wrote no host_uuid within %s; check journalctl -u %s, then promote again", promotedUnit, req.Wait, promotedUnit))
}

// stopAfterFailure stops and disables a promoted agent the install may have
// started, and returns err with what became of that agent.
func (h hostEnv) stopAfterFailure(ctx context.Context, req PromoteRequest, err error) error {
	if stopErr := h.stopPromoted(ctx, req); stopErr != nil {
		return trace.NewAggregate(err, trace.Wrap(stopErr, "stopping %s; stop and disable it by hand", promotedUnit))
	}
	return trace.Wrap(err, "%s was stopped and disabled again and the scoped agent still serves", promotedUnit)
}

// stopPromoted stops and disables teleport.service when /etc/teleport.yaml is
// this migration's promoted config. It is how a promoted agent that never
// joined stops retrying beside the scoped agent; the promoted install stays
// for the next promote to replace. It refuses unless the scoped agent is
// running, since teleport.service may otherwise be the host's only agent.
func (h hostEnv) stopPromoted(ctx context.Context, req PromoteRequest) error {
	if !h.exists(defaults.ConfigFilePath) {
		return nil
	}
	if err := h.checkScopedServes(ctx, req); err != nil {
		return trace.Wrap(err, "%s may be this host's only agent", promotedUnit)
	}
	if err := h.checkConfigOwner("promoted", defaults.ConfigFilePath, req.Proxy, req.SourceHostUUID, "", true); err != nil {
		return trace.Wrap(err)
	}
	// The unit may not exist: retire's teleport-update uninstall removed it,
	// and a failed enable may not have written it again. stop and disable fail
	// on such a unit, so their results are checked instead of their exit
	// codes; systemd reports a unit it does not know as inactive and not
	// enabled.
	_ = runCommand(ctx, "systemctl", "stop", promotedUnit)
	_ = runCommand(ctx, "systemctl", "disable", promotedUnit)
	stopped, state, err := unitStopped(ctx, promotedUnit)
	if err != nil {
		return trace.Wrap(err)
	}
	if !stopped {
		return trace.CompareFailed("%s is %s after systemctl stop %s", promotedUnit, state, promotedUnit)
	}
	enabled, err := unitEnabled(ctx, promotedUnit)
	if err != nil {
		return trace.Wrap(err)
	}
	if enabled {
		return trace.CompareFailed("%s is still enabled after systemctl disable %s", promotedUnit, promotedUnit)
	}
	return nil
}

// promotedByHand returns the host_uuid of the agent a by-hand promote left on
// the host, and changes nothing. That agent runs as teleport.service from this
// migration's /etc/teleport.yaml, with a host_uuid in /var/lib/teleport. Any
// scoped install still on the host is left for the caller's uninstall step.
func (h hostEnv) promotedByHand(ctx context.Context, req PromoteRequest) (InstallResponse, error) {
	if !h.exists(defaults.ConfigFilePath) {
		return InstallResponse{}, trace.NotFound("no promoted config %s is on this host", defaults.ConfigFilePath)
	}
	if err := h.checkConfigOwner("promoted", defaults.ConfigFilePath, req.Proxy, req.SourceHostUUID, "", true); err != nil {
		return InstallResponse{}, trace.Wrap(err)
	}
	active, err := unitActive(ctx, promotedUnit)
	if err != nil {
		return InstallResponse{}, trace.Wrap(err)
	}
	id := h.readTrimmed(path.Join(defaults.DataDir, hostid.FileName))
	if !active || id == "" {
		return InstallResponse{}, trace.CompareFailed("%s is not running with a host_uuid in %s", promotedUnit, defaults.DataDir)
	}
	return InstallResponse{HostUUID: id}, nil
}

// checkDefaultPathsFree refuses default paths that hold anything but an
// earlier promote of this migration:
//   - a config at /etc/teleport.yaml must be that promote's
//   - without one, teleport.service must not be running
//   - without one, /var/lib/teleport must hold no host_uuid
func (h hostEnv) checkDefaultPathsFree(ctx context.Context, req PromoteRequest) error {
	if h.exists(defaults.ConfigFilePath) {
		return trace.Wrap(h.checkConfigOwner("promoted", defaults.ConfigFilePath, req.Proxy, req.SourceHostUUID, "", true))
	}
	active, err := unitActive(ctx, promotedUnit)
	if err != nil {
		return trace.Wrap(err)
	}
	if active {
		return trace.CompareFailed("%s is running without %s", promotedUnit, defaults.ConfigFilePath)
	}
	if h.exists(path.Join(defaults.DataDir, hostid.FileName)) {
		return trace.CompareFailed("%s holds another agent's %s", defaults.DataDir, hostid.FileName)
	}
	return nil
}

// scopedBaseURL returns the download base URL the scoped install's
// update.yaml names, so the promoted install downloads from the same mirror.
// It returns "" when the file names none or does not exist.
func (h hostEnv) scopedBaseURL() (string, error) {
	updateYAML := path.Join(scopedInstallDir, "update.yaml")
	body, err := os.ReadFile(h.path(updateYAML))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", trace.Wrap(trace.ConvertSystemError(err), "reading %s", updateYAML)
	}
	var cfg struct {
		Spec struct {
			BaseURL string `yaml:"base_url"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(body, &cfg); err != nil {
		return "", trace.Wrap(err, "parsing %s", updateYAML)
	}
	return cfg.Spec.BaseURL, nil
}

// renderPromotedConfig returns the YAML of scoped, the scoped agent's config,
// moved onto the default paths and joining with req's token: data_dir set to
// the default, a pid file named for the scoped install renamed to
// teleport.pid, and each listen address apply shifted moved back down. A
// disabled service's address was never shifted and is left alone. Labels,
// scope and services stay as the scoped agent has them. It leaves scoped
// unchanged.
func renderPromotedConfig(scoped *config.FileConfig, req PromoteRequest) (string, error) {
	// A round trip through YAML deep-copies scoped, so Reconfigure leaves it
	// unchanged.
	scopedYAML, err := scoped.YAMLString()
	if err != nil {
		return "", trace.Wrap(err)
	}
	promoted, err := config.ReadConfig(strings.NewReader(scopedYAML))
	if err != nil {
		return "", trace.Wrap(err)
	}
	reconfigure := config.ReconfigureRequest{
		CAPins:                         req.CAPins,
		TokenName:                      req.TokenName.String(),
		JoinMethod:                     req.JoinMethod,
		BoundKeypairRegistrationSecret: req.RegistrationSecret,
		DataDir:                        defaults.DataDir,
		PIDFile:                        promoted.PIDFile,
	}
	if path.Base(promoted.PIDFile) == scopedPIDFileName {
		reconfigure.PIDFile = path.Join(path.Dir(promoted.PIDFile), promotedPIDFileName)
	}
	if err := unshiftListenAddr(promoted.DiagAddr, &reconfigure.DiagAddr, true); err != nil {
		return "", trace.Wrap(err, "unshifting diag_addr")
	}
	if err := unshiftListenAddr(promoted.SSH.ListenAddress, &reconfigure.SSHListenAddr, promoted.SSH.Enabled()); err != nil {
		return "", trace.Wrap(err, "unshifting ssh_service.listen_addr")
	}
	if err := unshiftListenAddr(promoted.Kube.ListenAddress, &reconfigure.KubeListenAddr, promoted.Kube.Enabled()); err != nil {
		return "", trace.Wrap(err, "unshifting kubernetes_service.listen_addr")
	}
	if err := unshiftListenAddr(promoted.Metrics.ListenAddress, &reconfigure.MetricsListenAddr, promoted.Metrics.Enabled()); err != nil {
		return "", trace.Wrap(err, "unshifting metrics_service.listen_addr")
	}
	if err := config.Reconfigure(promoted, reconfigure); err != nil {
		return "", trace.Wrap(err)
	}
	out, err := config.RenderValidated(promoted)
	return out, trace.Wrap(err)
}

// unshiftListenAddr sets *dst to src with its port moved down by unshiftPort
// when the service is enabled and src is set, and to src otherwise.
func unshiftListenAddr(src string, dst *string, enabled bool) error {
	*dst = src
	if src == "" || !enabled {
		return nil
	}
	unshifted, err := unshiftPort(src)
	if err != nil {
		return trace.Wrap(err)
	}
	*dst = unshifted
	return nil
}

// unshiftPort moves addr's port down by config.ShiftedPortOffset. A port at or
// below the offset was not set by apply, and is refused rather than guessed at.
func unshiftPort(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", trace.Wrap(err)
	}
	shifted, err := strconv.Atoi(port)
	if err != nil || shifted <= config.ShiftedPortOffset {
		return "", trace.BadParameter("listen address %q was not shifted by apply", addr)
	}
	return net.JoinHostPort(host, strconv.Itoa(shifted-config.ShiftedPortOffset)), nil
}
