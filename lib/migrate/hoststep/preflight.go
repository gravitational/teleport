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
	"cmp"
	"context"
	"fmt"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/server/installstatus"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/hostid"
)

// PreflightRequest is what the preflight step needs.
type PreflightRequest struct {
	// SourceHostUUID is the host_uuid of the source node the step was reached
	// through, which teleport.service's agent must hold.
	SourceHostUUID string `json:"source_host_uuid"`
}

// PreflightResponse is what the preflight step read on the host.
type PreflightResponse struct {
	// Facts are the source agent's facts. The step leaves At, Error and
	// Unreachable empty.
	Facts PreflightResult `json:"facts"`
	// Reason is why the host cannot take a scoped install, or "".
	Reason string `json:"reason,omitempty"`
}

// preflight reads the source agent's facts, then checks the host can take a
// scoped install. A failed check is Reason, with the facts read before it;
// only a step that cannot run at all is an error. A host without systemd has
// no teleport.service to read, so its facts come from the default config, for
// migrating it by hand.
func (h hostEnv) preflight(ctx context.Context, req PreflightRequest) (PreflightResponse, error) {
	suffixHostUUID := h.readTrimmed(path.Join(scopedDataDir, hostid.FileName))
	hasSystemd := h.exists("/run/systemd/system")
	var argv []string
	configPath := defaults.ConfigFilePath
	if hasSystemd {
		var err error
		argv, configPath, err = h.sourceUnit(ctx)
		if err != nil {
			return PreflightResponse{
				Facts:  PreflightResult{SuffixHostUUID: suffixHostUUID},
				Reason: err.Error(),
			}, nil
		}
	}
	fileConfig, err := h.readAgentConfig(configPath)
	if err != nil {
		return PreflightResponse{
			Facts:  PreflightResult{ConfigPath: configPath, SuffixHostUUID: suffixHostUUID},
			Reason: err.Error(),
		}, nil
	}
	facts := agentFacts(fileConfig)
	// teleport reads a token_name that is an absolute path from that file, as
	// servicecfg.Config.Token does; a delegated method's token name isn't a
	// secret. A scope-qualified name also starts with / but names no file.
	if path.IsAbs(facts.TokenName) {
		if name := h.readTrimmed(facts.TokenName); name != "" {
			facts.TokenName = name
		}
	}
	facts.ConfigPath = configPath
	facts.SuffixHostUUID = suffixHostUUID
	if !hasSystemd {
		return PreflightResponse{Facts: facts, Reason: "no systemd on the host"}, nil
	}
	// None of another agent's facts are recorded as this node's.
	if reason := h.otherAgentReason(facts.DataDir, req.SourceHostUUID); reason != "" {
		return PreflightResponse{Facts: PreflightResult{SuffixHostUUID: suffixHostUUID}, Reason: reason}, nil
	}
	return PreflightResponse{Facts: facts, Reason: h.installReason(ctx, argv)}, nil
}

// otherAgentReason returns why teleport.service's agent, whose data dir is
// dataDir, is not the source node sourceHostUUID, or "" when it is. A host can
// run the node from an install suffix beside a default agent of another
// cluster, and that agent's config must not be migrated in its place.
func (h hostEnv) otherAgentReason(dataDir, sourceHostUUID string) string {
	const remedy = "(this node may run from an install suffix); migrate this host by hand"
	switch got := h.readTrimmed(path.Join(dataDir, hostid.FileName)); {
	case got == "":
		return fmt.Sprintf("teleport.service runs another agent: its data dir %q has no host_uuid, and this node is %q %s", dataDir, sourceHostUUID, remedy)
	case got != sourceHostUUID:
		return fmt.Sprintf("teleport.service runs another agent: its data dir %q holds host_uuid %q, not this node's %q %s", dataDir, got, sourceHostUUID, remedy)
	}
	return ""
}

// agentFacts returns the facts a parsed agent config holds. A delegated join
// method's token_name names the operator's token in the target; a minted
// method's holds the agent's own secret, so it is left out.
func agentFacts(fileConfig *config.FileConfig) PreflightResult {
	facts := PreflightResult{
		DataDir:    path.Clean(cmp.Or(fileConfig.DataDir, defaults.DataDir)),
		JoinMethod: cmp.Or(string(fileConfig.JoinParams.Method), string(types.JoinMethodToken)),
		StaticKey:  fileConfig.JoinParams.BoundKeypair.StaticPrivateKeyPath != "",
		Services:   enabledServices(fileConfig),
	}
	if !IsMintedJoinMethod(facts.JoinMethod) {
		facts.TokenName = cmp.Or(fileConfig.JoinParams.TokenName, fileConfig.AuthToken)
	}
	return facts
}

// enabledServices lists the services the config runs. Enabled applies
// Teleport's defaults, so a config that leaves out auth_service, proxy_service
// or ssh_service runs that service.
func enabledServices(fileConfig *config.FileConfig) []string {
	var out []string
	for _, section := range []struct {
		name string
		on   bool
	}{
		{"auth_service", fileConfig.Auth.Enabled()},
		{"proxy_service", fileConfig.Proxy.Enabled()},
		{"ssh_service", fileConfig.SSH.Enabled()},
		{"app_service", fileConfig.Apps.Enabled()},
		{"kubernetes_service", fileConfig.Kube.Enabled()},
		{"db_service", fileConfig.Databases.Enabled()},
		{"windows_desktop_service", fileConfig.WindowsDesktop.Enabled()},
		{"discovery_service", fileConfig.Discovery.Enabled()},
		{"okta_service", fileConfig.Okta.Enabled()},
		{"jamf_service", fileConfig.Jamf.Enabled()},
		{"relay_service", fileConfig.Relay.Enabled},
		{"linux_desktop_service", fileConfig.LinuxDesktop.Enabled()},
		{"plugin_service", fileConfig.Plugins.Enabled},
	} {
		if section.on {
			out = append(out, section.name)
		}
	}
	return out
}

// isAllowedUnitFlag reports whether teleport.service may start teleport with
// arg: the scoped agent's config is built from the config file, so any other
// flag would be lost.
func isAllowedUnitFlag(arg string) bool {
	switch {
	case arg == "-c", arg == "--config", strings.HasPrefix(arg, "--config="),
		arg == "--pid-file", strings.HasPrefix(arg, "--pid-file="),
		arg == "-d", arg == "--debug", !strings.HasPrefix(arg, "-"):
		return true
	}
	return false
}

// hasSELinuxModule reports whether semodule lists Teleport's SELinux module. A
// host without semodule reports false: teleport-update cannot remove the module
// without it either.
func hasSELinuxModule(ctx context.Context) (bool, error) {
	if _, err := exec.LookPath("semodule"); err != nil {
		return false, nil
	}
	out, err := exec.CommandContext(ctx, "semodule", "-l").Output()
	if err != nil {
		return false, trace.Wrap(err)
	}
	for line := range strings.Lines(string(out)) {
		if strings.HasPrefix(line, "teleport_ssh") {
			return true, nil
		}
	}
	return false, nil
}

// installReason returns why the host cannot take a scoped install beside
// the source agent, or "". argv is teleport.service's ExecStart. Disk is
// measured where the scoped install goes: the parent of the data dirs and
// /opt, not /opt/teleport, which a host that never ran teleport-update lacks.
func (h hostEnv) installReason(ctx context.Context, argv []string) string {
	if _, err := exec.LookPath("teleport-update"); err != nil {
		return "teleport-update is not installed"
	}
	hasModule, err := hasSELinuxModule(ctx)
	if err != nil {
		return fmt.Sprintf("could not list the SELinux modules (semodule -l): %v", err)
	}
	if hasModule {
		return "Teleport's SELinux module is installed; the scoped install's teleport-update enable would remove it (semodule -r teleport_ssh) from under the source agent"
	}
	for _, arg := range argv[min(1, len(argv)):] {
		if !isAllowedUnitFlag(arg) {
			// Only the flag's name: the value of --token=... is a credential.
			flag, _, _ := strings.Cut(arg, "=")
			return fmt.Sprintf("teleport starts with %s on its command line, which the scoped agent's config would lose; move it into the config file or migrate by hand", flag)
		}
	}
	const minFreeDiskKiB = installstatus.InstallerMinFreeDiskMB * 1024
	for _, dir := range []string{path.Dir(defaults.DataDir), "/opt"} {
		free, err := utils.FreeDiskWithReserve(h.path(dir), 0)
		if err != nil {
			return fmt.Sprintf("could not read the free disk space under %q: %v", dir, err)
		}
		if kib := free / 1024; kib < minFreeDiskKiB {
			return fmt.Sprintf("%d MB free under %q, %d MB needed", kib/1024, dir, minFreeDiskKiB/1024)
		}
	}
	return ""
}

// PreflightResult is what preflight read from a source host over SSH, and why
// the host failed it, if it did.
type PreflightResult struct {
	// At is when the preflight ran.
	At time.Time `json:"at"`
	// Error is why the host failed preflight, or why preflight could not
	// finish; the facts are what this run read before it stopped, or what an
	// earlier run learned when this one could not reach the host.
	Error string `json:"error,omitempty"`
	// Unreachable is set when Error is a connection failure rather than
	// something the host reported.
	Unreachable bool `json:"unreachable,omitempty"`
	// ConfigPath is the source agent's config file.
	ConfigPath string `json:"config_path"`
	// DataDir is the source agent's data directory.
	DataDir string `json:"data_dir"`
	// JoinMethod is the join method in the source agent's config.
	JoinMethod string `json:"join_method"`
	// TokenName is the join token the config names, when it names one.
	TokenName string `json:"token_name,omitempty"`
	// StaticKey is set when a bound_keypair agent joins with a static,
	// pre-registered key (static_key_path) rather than a registration secret.
	StaticKey bool `json:"static_key,omitempty"`
	// Services are the services the source agent's config runs.
	Services []string `json:"services"`
	// SuffixHostUUID is the host_uuid of a scoped install already on the
	// host, if any. Plan pairs the host with the target agent of that ID when
	// no agent carries the host's correlation labels, as after a migration by
	// hand.
	SuffixHostUUID string `json:"suffix_host_uuid,omitempty"`
}
