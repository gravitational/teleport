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
	"os"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/hostid"
)

// InstallRequest is what the install step needs.
type InstallRequest struct {
	// Proxy is the target cluster's proxy address.
	Proxy string `json:"proxy"`
	// JoinMethod is how the scoped agent joins the target cluster.
	JoinMethod string `json:"join_method"`
	// TokenName is the token's scope and name; for the token method the name
	// carries the secret.
	TokenName scopes.QualifiedName `json:"token_name"`
	// RegistrationSecret is set for the bound_keypair method only.
	RegistrationSecret string `json:"registration_secret,omitempty"`
	// CAPins are the target cluster's CA pins.
	CAPins []string `json:"ca_pins"`
	// SourceCluster is the name of the source cluster, which labels the
	// scoped agent.
	SourceCluster string `json:"source_cluster"`
	// SourceHostUUID is the host_uuid of the source agent the scoped agent
	// replaces, which labels the scoped agent.
	SourceHostUUID string `json:"source_host_uuid"`
	// Services are the ones the source config must still run, which the
	// token grants.
	Services []string `json:"services"`
	// BaseURL is the CDN base URL teleport-update downloads the scoped agent
	// from, empty for its default.
	BaseURL string `json:"base_url,omitempty"`
	// Wait is how long to wait for another install's lock on the host, and
	// then for the scoped agent to write its host_uuid.
	Wait time.Duration `json:"wait"`
}

// InstallResponse is what the install step started.
type InstallResponse struct {
	// HostUUID is the scoped agent's host_uuid.
	HostUUID string `json:"host_uuid"`
}

// install writes the scoped agent's config, starts the scoped agent beside
// the source agent, and waits for it to write its host_uuid. A scoped config
// already on the host is replaced only when it targets the same proxy and
// source host. A lock on it keeps two migrate commands from installing at
// once; install waits up to req.Wait for another one to finish.
func (h hostEnv) install(ctx context.Context, req InstallRequest) (InstallResponse, error) {
	// An apply stopped with Ctrl-C leaves its install running on the host, and
	// the rerun has already replaced that install's token, so the rerun waits
	// for it rather than leaving the host to an agent that cannot join.
	unlock, err := utils.FSTryWriteLockTimeout(ctx, h.path(scopedConfigPath+".lock"), req.Wait)
	if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
		return InstallResponse{}, trace.AlreadyExists("another migrate command is still installing on this host after %s", req.Wait)
	}
	if err != nil {
		return InstallResponse{}, trace.Wrap(err, "locking %q", scopedConfigPath+".lock")
	}
	defer unlock()

	if h.exists(scopedConfigPath) {
		if err := h.checkConfigOwner("scoped", scopedConfigPath, req.Proxy, req.SourceHostUUID, "roll that host back or remove the config by hand", false); err != nil {
			return InstallResponse{}, trace.Wrap(err)
		}
	}
	_, sourceConfigPath, err := h.sourceUnit(ctx)
	if err != nil {
		return InstallResponse{}, trace.Wrap(err)
	}
	sourceConfig, err := h.readAgentConfig(sourceConfigPath)
	if err != nil {
		return InstallResponse{}, trace.Wrap(err)
	}
	if services := enabledServices(sourceConfig); !slices.Equal(services, req.Services) {
		return InstallResponse{}, trace.CompareFailed("the agent config at %q changed during apply (it runs %v, the token grants %v); run plan --check --resource node/%s, then apply again", sourceConfigPath, services, req.Services, req.SourceHostUUID)
	}
	scopedConfig, err := renderScopedConfig(sourceConfig, req)
	if err != nil {
		return InstallResponse{}, trace.Wrap(err, "rendering the scoped agent's config")
	}
	if err := os.MkdirAll(h.path(scopedDataDir), 0o700); err != nil {
		return InstallResponse{}, trace.ConvertSystemError(err)
	}
	if err := writePrivate(h.path(scopedConfigPath), []byte(scopedConfig)); err != nil {
		return InstallResponse{}, trace.Wrap(err, "writing %q", scopedConfigPath)
	}
	enable := []string{"teleport-update", "--install-suffix", scopedInstallSuffix, "enable", "--proxy", req.Proxy}
	if req.BaseURL != "" {
		enable = append(enable, "--base-url", req.BaseURL)
	}
	for _, args := range [][]string{
		enable,
		{"systemctl", "enable", scopedUnit},
		{"systemctl", "restart", scopedUnit},
	} {
		if err := runCommand(ctx, args[0], args[1:]...); err != nil {
			return InstallResponse{}, trace.Wrap(err)
		}
	}
	if err := runCommand(ctx, "systemctl", "is-active", "--quiet", scopedUnit); err != nil {
		return InstallResponse{}, trace.Wrap(err, "%s is not running; check journalctl -u %s", scopedUnit, scopedUnit)
	}
	retry, err := retryutils.NewConstant(time.Second)
	if err != nil {
		return InstallResponse{}, trace.Wrap(err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, req.Wait)
	defer cancel()
	var id string
	err = retry.For(waitCtx, func() error {
		if id = h.readTrimmed(path.Join(scopedDataDir, hostid.FileName)); id == "" {
			return trace.NotFound("no host_uuid yet")
		}
		return nil
	})
	switch {
	case err == nil:
		return InstallResponse{HostUUID: id}, nil
	case ctx.Err() != nil:
		return InstallResponse{}, trace.Wrap(ctx.Err())
	default:
		return InstallResponse{}, trace.LimitExceeded("%s is running but wrote no host_uuid within %s; check journalctl -u %s", scopedUnit, req.Wait, scopedUnit)
	}
}

// renderScopedConfig returns the YAML of sourceConfig, the source agent's config, retargeted to join the target cluster
// as the scoped agent: its own data dir, labels naming the source agent, and
// every port the source pins shifted up so the two agents run side by side.
// It leaves sourceConfig unchanged.
func renderScopedConfig(sourceConfig *config.FileConfig, req InstallRequest) (string, error) {
	// A round trip through YAML deep-copies sourceConfig, so Reconfigure
	// leaves it unchanged.
	sourceYAML, err := sourceConfig.YAMLString()
	if err != nil {
		return "", trace.Wrap(err)
	}
	scoped, err := config.ReadConfig(strings.NewReader(sourceYAML))
	if err != nil {
		return "", trace.Wrap(err)
	}
	reconfigure := config.ReconfigureRequest{
		CAPins:                         req.CAPins,
		TokenName:                      req.TokenName.String(),
		JoinMethod:                     req.JoinMethod,
		BoundKeypairRegistrationSecret: req.RegistrationSecret,
		DataDir:                        scopedDataDir,
		ShiftPorts:                     true,
	}
	if scoped.Version == defaults.TeleportConfigVersionV3 {
		reconfigure.ProxyServer = req.Proxy
	} else {
		reconfigure.AuthServer = req.Proxy
	}
	if scoped.PIDFile != "" {
		reconfigure.PIDFile = path.Join(path.Dir(scoped.PIDFile), scopedPIDFileName)
	}
	if err := config.Reconfigure(scoped, reconfigure); err != nil {
		return "", trace.Wrap(err)
	}
	if scoped.SSH.Labels == nil {
		scoped.SSH.Labels = make(map[string]string, 2)
	}
	scoped.SSH.Labels[SourceClusterLabel] = req.SourceCluster
	scoped.SSH.Labels[SourceHostLabel] = req.SourceHostUUID
	return config.RenderValidated(scoped)
}
