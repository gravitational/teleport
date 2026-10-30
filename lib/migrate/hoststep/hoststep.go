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

// Package hoststep runs the steps of teleport migrate on a source host, as
// teleport migrate host <step>: a JSON request on stdin, a JSON result on stdout.
package hoststep

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/config/systemd"
	"github.com/gravitational/teleport/lib/defaults"
)

// Preflight is the host step that reads the source agent's facts and
// checks the host can take a scoped install.
const Preflight = "preflight"

// Install is the host step that installs the scoped agent beside the
// source agent.
const Install = "install"

// Uninstall is the host step that removes the scoped agent's install.
const Uninstall = "uninstall"

// Retire is the host step that permanently removes the source agent once
// the scoped agent can stand alone.
const Retire = "retire"

// scopedInstallSuffix is the install suffix of the scoped agent.
const scopedInstallSuffix = "migrated"

// scopedDataDir is the scoped agent's data directory.
const scopedDataDir = "/var/lib/teleport_" + scopedInstallSuffix

// scopedConfigPath is the scoped agent's config file.
const scopedConfigPath = "/etc/teleport_" + scopedInstallSuffix + ".yaml"

// scopedPIDFileName is the name of the scoped agent's PID file, in the
// directory of the source agent's.
const scopedPIDFileName = "teleport_" + scopedInstallSuffix + ".pid"

// scopedInstallDir is the scoped agent's install directory.
const scopedInstallDir = updaterDir + "/" + scopedInstallSuffix

// updaterDir is teleport-update's default install dir, which the tool assumes
// throughout: apply installs the scoped agent under it, and retire removes the
// source agent's default install from it.
const updaterDir = "/opt/teleport"

// sourceInstallDir is the source agent's install under updaterDir, the default
// namespace teleport-update manages.
const sourceInstallDir = updaterDir + "/default"

// scopedUnit is the scoped agent's systemd unit.
const scopedUnit = "teleport_" + scopedInstallSuffix + ".service"

// scopedUpdaterTimer is the timer that runs the scoped agent's updater.
const scopedUpdaterTimer = "teleport-update_" + scopedInstallSuffix + ".timer"

// scopedUpdaterService is the service the scoped agent's updater timer starts.
const scopedUpdaterService = "teleport-update_" + scopedInstallSuffix + ".service"

// RunHostStep runs one migration step on this host, as `teleport migrate host
// <step>`, reading the step's JSON request from in and writing its JSON result
// to out.
func RunHostStep(ctx context.Context, step string, in io.Reader, out io.Writer) error {
	h := hostEnv{}
	switch step {
	case Preflight:
		return runStep(ctx, in, out, h.preflight)
	case Install:
		return runStep(ctx, in, out, h.install)
	case Uninstall:
		return runStep(ctx, in, out, h.uninstall)
	case Retire:
		return runStep(ctx, in, out, h.retire)
	}
	return trace.BadParameter("unknown host step %q", step)
}

// runStep decodes fn's request from in, runs fn, and encodes its result to out.
func runStep[Request, Response any](ctx context.Context, in io.Reader, out io.Writer, fn func(context.Context, Request) (Response, error)) error {
	var request Request
	if err := json.NewDecoder(in).Decode(&request); err != nil {
		return trace.Wrap(err, "reading the request")
	}
	response, err := fn(ctx, request)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(json.NewEncoder(out).Encode(response))
}

// hostEnv is the host a step runs on. rootPath is prefixed to every file path
// a step uses, but not to the commands it runs, which are found on PATH; it is
// "" on a real host.
type hostEnv struct {
	rootPath string
}

// path returns p under the host's root.
func (h hostEnv) path(p string) string { return filepath.Join(h.rootPath, p) }

// exists reports whether p exists under the host's root.
func (h hostEnv) exists(p string) bool {
	_, err := os.Lstat(h.path(p))
	return err == nil
}

// readTrimmed returns p's contents with surrounding space removed, or "" when
// p cannot be read.
func (h hostEnv) readTrimmed(p string) string {
	b, err := os.ReadFile(h.path(p))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// envAssignment matches each variable an Environment= value assigns, quoted or
// not.
var envAssignment = regexp.MustCompile(`(?:^|[\s"'])([A-Za-z_][A-Za-z0-9_]*)=`)

// updaterEnvKeys are the variables teleport-update's own drop-in sets on
// teleport.service; the scoped install gets its own.
var updaterEnvKeys = []string{"TELEPORT_UPDATE_CONFIG_FILE", "TELEPORT_UPDATE_INSTALL_DIR"}

// sourceUnit returns teleport.service's ExecStart argv and the config file it
// starts with, as systemd loaded them: systemctl show resolves drop-ins,
// quoting and specifiers, but joins argv with spaces, so an argument holding a
// space reads as two. Without -c or --config the config is /etc/teleport.yaml.
// It is a NotFound error when systemd has not loaded teleport.service or the
// unit has no ExecStart, and a BadParameter error when the unit has more than
// one ExecStart, ExecStart expands an environment variable, whose value the
// tool cannot see, or ignores teleport's failure with a - prefix, or the unit
// runs teleport as a user other than root or has environment that differs
// from the scoped agent's unit, which runs as root and reads only
// /etc/default/teleport.
func (h hostEnv) sourceUnit(ctx context.Context) (argv []string, configPath string, err error) {
	out, err := exec.CommandContext(ctx, "systemctl", "show", "--property=LoadState,ExecStart,User,DynamicUser,Environment,EnvironmentFiles,PassEnvironment", "teleport.service").Output()
	if err != nil {
		// systemctl says why on stderr.
		reason := err.Error()
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok && len(bytes.TrimSpace(exitErr.Stderr)) > 0 {
			reason = string(bytes.TrimSpace(exitErr.Stderr))
		}
		return nil, "", trace.NotFound("systemctl show teleport.service failed: %s; migrate this host by hand", reason)
	}
	var loadState, user, dynamicUser, env, lostEnv string
	var execStarts []string
	readsDefaultEnv := false
	for line := range strings.SplitSeq(string(out), "\n") {
		key, value, _ := strings.Cut(line, "=")
		switch key {
		case "LoadState":
			loadState = value
		case "ExecStart":
			if value != "" {
				execStarts = append(execStarts, value)
			}
		case "User":
			user = value
		case "DynamicUser":
			dynamicUser = value
		case "Environment":
			env += value + "\n"
			for _, m := range envAssignment.FindAllStringSubmatch(value, -1) {
				if lostEnv == "" && !slices.Contains(updaterEnvKeys, m[1]) {
					lostEnv = "sets " + m[1] + " with Environment="
				}
			}
		case "PassEnvironment":
			// Space-separated names, passed from systemd's own environment.
			if name, _, _ := strings.Cut(value, " "); name != "" && lostEnv == "" {
				lostEnv = "passes " + name + " with PassEnvironment="
			}
		case "EnvironmentFiles":
			// "<path> (ignore_errors=yes)"
			envFile, _, _ := strings.Cut(value, " (")
			readsDefaultEnv = readsDefaultEnv || envFile == systemd.DefaultEnvironmentFile
			b, err := os.ReadFile(envFile)
			if errors.Is(err, fs.ErrNotExist) {
				// A missing file sets nothing the scoped agent could lose.
				continue
			}
			if err != nil {
				return nil, "", trace.BadParameter("reading teleport.service's EnvironmentFile %q failed: %v; migrate this host by hand", envFile, err)
			}
			if lostEnv == "" && envFile != systemd.DefaultEnvironmentFile {
				lostEnv = "reads " + envFile + " with EnvironmentFile="
			}
			env += string(b) + "\n"
		}
	}
	if loadState != "loaded" {
		return nil, "", trace.NotFound("systemd did not load teleport.service (LoadState=%s); migrate this host by hand", loadState)
	}
	if len(execStarts) == 0 {
		return nil, "", trace.NotFound("teleport does not run under teleport.service; migrate this host by hand")
	}
	if len(execStarts) > 1 {
		return nil, "", trace.BadParameter("teleport.service has %d ExecStart commands; migrate this host by hand", len(execStarts))
	}
	if (user != "" && user != "root") || dynamicUser == "yes" {
		return nil, "", trace.BadParameter("teleport.service runs teleport as User=%q DynamicUser=%s, and the scoped agent's unit would run it as root; migrate this host by hand", user, dynamicUser)
	}
	if !readsDefaultEnv && h.readTrimmed(systemd.DefaultEnvironmentFile) != "" {
		return nil, "", trace.BadParameter("teleport.service does not read %s, which the scoped agent's unit would; empty the file or migrate this host by hand", systemd.DefaultEnvironmentFile)
	}
	// teleport start reads a whole config from TELEPORT_CONFIG in place of the
	// config file, so the file's facts would be wrong.
	if strings.Contains(env, defaults.ConfigEnvar+"=") {
		return nil, "", trace.BadParameter("teleport.service sets %s, which replaces the config file; migrate this host by hand", defaults.ConfigEnvar)
	}
	// Only the variable's name: its value can be a credential.
	if lostEnv != "" {
		return nil, "", trace.BadParameter("teleport.service %s, which the scoped agent's unit would lose; move it into %s or migrate by hand", lostEnv, systemd.DefaultEnvironmentFile)
	}
	// "{ path=<path> ; argv[]=<argv> ; ignore_errors=<yes|no> ; ... }"
	_, args, _ := strings.Cut(execStarts[0], " ; argv[]=")
	args, rest, _ := strings.Cut(args, " ; ignore_errors=")
	// The argument is left out: it can hold a secret.
	if strings.Contains(args, "$") {
		return nil, "", trace.BadParameter("teleport.service starts teleport with an environment variable, whose value the tool cannot see; migrate this host by hand")
	}
	if strings.HasPrefix(rest, "yes") {
		return nil, "", trace.BadParameter("teleport.service ignores teleport's failure (ExecStart=-), which the scoped agent's unit would not; migrate this host by hand")
	}
	argv = strings.Fields(args)
	configPath = defaults.ConfigFilePath
	for i, arg := range argv {
		switch {
		case (arg == "-c" || arg == "--config") && i+1 < len(argv):
			configPath = argv[i+1]
		case strings.HasPrefix(arg, "--config="):
			configPath = strings.TrimPrefix(arg, "--config=")
		}
	}
	return argv, configPath, nil
}

// readAgentConfig reads and parses the agent config at configPath.
// config.ReadConfig is strict, so a key this build does not know is an
// error: the tool must be at least as new as the newest config it reads.
func (h hostEnv) readAgentConfig(configPath string) (*config.FileConfig, error) {
	f, err := os.Open(h.path(configPath))
	if err != nil {
		return nil, trace.NotFound("could not read the agent config at %q: %v", configPath, err)
	}
	defer f.Close()
	// The parser's error is left out: it can quote a secret from the file.
	fileConfig, err := config.ReadConfig(f)
	if err != nil {
		return nil, trace.BadParameter("parsing the agent config at %q failed (an unknown key means the agent is newer than this migrate; upgrade the tool)", configPath)
	}
	return fileConfig, nil
}

// IsMintedJoinMethod reports whether method is one whose join token the tool
// creates for the host, rather than one the operator names in join_params.
func IsMintedJoinMethod(method string) bool {
	return method == string(types.JoinMethodToken) || method == string(types.JoinMethodBoundKeypair)
}

// checkScopedConfigOwner refuses unless the scoped config on the host targets
// wantProxy and labels the scoped agent with wantSourceHost. remedy ends the
// refusal of another migration's config, saying what to do instead; a config
// without the tool's label can only be removed by hand.
func (h hostEnv) checkScopedConfigOwner(wantProxy, wantSourceHost, remedy string) error {
	existing, err := h.readAgentConfig(scopedConfigPath)
	if err != nil {
		return trace.Wrap(err)
	}
	// Reconfigure writes a v1 or v2 config's target to auth_servers.
	var proxy string
	switch {
	case existing.Version == defaults.TeleportConfigVersionV3:
		proxy = existing.ProxyServer
	case slices.Contains(existing.AuthServers, wantProxy):
		proxy = wantProxy
	default:
		proxy = cmp.Or(existing.AuthServer, strings.Join(existing.AuthServers, ","))
	}
	sourceHost := existing.SSH.Labels[SourceHostLabel]
	if sourceHost == "" {
		return trace.CompareFailed("the scoped config at %q has no %s label, so teleport migrate did not install it; remove the scoped install by hand", scopedConfigPath, SourceHostLabel)
	}
	if proxy != wantProxy || sourceHost != wantSourceHost {
		return trace.CompareFailed("the scoped config at %q belongs to another migration (proxy %q, source host %q); %s", scopedConfigPath, proxy, sourceHost, remedy)
	}
	return nil
}

// runCommand runs name with args and returns an error holding the command line
// and its combined output when it fails.
func runCommand(ctx context.Context, name string, args ...string) error {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return trace.Wrap(err, "%s %s: %s", name, strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return nil
}

// isExecutable reports whether p is a regular file someone may execute.
func isExecutable(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0
}

// unitProperty returns one systemd property of unit, as systemd reports it for
// a unit that is not loaded: ActiveState=inactive, UnitFileState empty. show
// exits 0 for a unit that does not exist, so a failure here is systemd itself,
// which the callers must not read as a state: is-active and is-enabled cannot
// tell the two apart.
func unitProperty(ctx context.Context, unit, property string) (string, error) {
	out, err := exec.CommandContext(ctx, "systemctl", "show", "-p", property, unit).Output()
	if err != nil {
		// systemctl says why on stderr, a host without systemd among the reasons.
		reason := err.Error()
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok && len(bytes.TrimSpace(exitErr.Stderr)) > 0 {
			reason = string(bytes.TrimSpace(exitErr.Stderr))
		}
		return "", trace.NotFound("asking systemd for %s of %s failed: %s; handle this host by hand", property, unit, reason)
	}
	// systemd 219 has no --value, so the output is <property>=<state>.
	state, ok := strings.CutPrefix(strings.TrimSpace(string(out)), property+"=")
	if !ok {
		return "", trace.BadParameter("asking systemd for %s of %s: unexpected output %.80q", property, unit, out)
	}
	return state, nil
}

// unitActive reports whether systemd says unit is active. reloading and
// refreshing are not active here, though is-active exits 0 for them; a unit
// caught mid-reload is refused and the step retried.
func unitActive(ctx context.Context, unit string) (bool, error) {
	state, err := unitProperty(ctx, unit, "ActiveState")
	return state == "active", trace.Wrap(err)
}

// unitStopped reports whether systemd says unit has stopped, and the state it
// is in. Only inactive and failed mean the process is gone; activating,
// deactivating and reloading are transitions, and the callers go on to delete
// what the unit was running from.
func unitStopped(ctx context.Context, unit string) (bool, string, error) {
	state, err := unitProperty(ctx, unit, "ActiveState")
	if err != nil {
		return false, "", trace.Wrap(err)
	}
	return state == "inactive" || state == "failed", state, nil
}

// unitEnabled reports whether systemd says unit is enabled, the one state that
// starts it at boot without a runtime-only link. enabled-runtime and static,
// which do not survive a reboot, are not enabled here, though is-enabled exits
// 0 for them.
func unitEnabled(ctx context.Context, unit string) (bool, error) {
	state, err := unitProperty(ctx, unit, "UnitFileState")
	return state == "enabled", trace.Wrap(err)
}

// Correlation labels: the labels a migrated scoped agent carries, so plan can
// pair a target node with its source host.
const (
	// SourceClusterLabel names the source cluster the agent migrated from.
	SourceClusterLabel = types.TeleportInternalLabelPrefix + "migrate-source-cluster"
	// SourceHostLabel holds the UUID of the source host the agent migrated from.
	SourceHostLabel = types.TeleportInternalLabelPrefix + "migrate-source-host-uuid"
)

// CacheRoot holds the teleport binaries the oneoff script downloads, one
// directory per version.
const CacheRoot = "/var/cache/teleport-migrate"
