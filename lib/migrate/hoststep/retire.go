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
	"bufio"
	"cmp"
	"context"
	"errors"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/gravitational/trace"
	"go.yaml.in/yaml/v3"

	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils/hostid"
)

// RetireRequest is what the retire step needs.
type RetireRequest struct {
	// TargetHostUUID is the host_uuid the scoped agent must carry.
	TargetHostUUID string `json:"target_host_uuid"`
	// SourceHostUUID is the host_uuid the source agent's data dir must hold.
	SourceHostUUID string `json:"source_host_uuid"`
	// ConfigPath is the source config preflight read, used once an earlier
	// retire removed teleport.service.
	ConfigPath string `json:"config_path"`
}

// retire checks that the scoped agent can stand alone, then removes the
// source agent. It is the one step that cannot be undone, so every refusal
// leaves the host as it found it. The fetch cache stays, so a later promote
// reuses it.
func (h hostEnv) retire(ctx context.Context, req RetireRequest) (struct{}, error) {
	// An empty ID would match a host with no install.
	if req.TargetHostUUID == "" || req.SourceHostUUID == "" {
		return struct{}{}, trace.BadParameter("retire needs both the target and the source host_uuid")
	}
	configPath, err := h.sourceConfigPath(ctx, req.ConfigPath)
	if err != nil {
		return struct{}{}, trace.Wrap(err)
	}
	if err := h.checkScopedAgent(ctx, req.TargetHostUUID, configPath); err != nil {
		return struct{}{}, trace.Wrap(err)
	}
	if err := h.removeSource(ctx, req.SourceHostUUID, configPath); err != nil {
		return struct{}{}, trace.Wrap(err)
	}
	return struct{}{}, nil
}

// sourceConfigPath returns the config teleport.service starts with, refusing a
// relative path, which would resolve against the working directory. Without an
// ExecStart to read it falls back to recorded, the path preflight read, and to
// /etc/teleport.yaml when recorded is empty. Preflight refuses a host with no
// ExecStart, so only a rerun gets there, after an earlier retire's
// teleport-update uninstall removed the unit.
func (h hostEnv) sourceConfigPath(ctx context.Context, recorded string) (string, error) {
	_, configPath, err := h.sourceUnit(ctx)
	switch {
	case trace.IsNotFound(err):
		configPath = cmp.Or(recorded, defaults.ConfigFilePath)
	case err != nil:
		return "", trace.Wrap(err)
	}
	if !path.IsAbs(configPath) {
		return "", trace.BadParameter("teleport.service starts with the relative config path %q; retire by hand", configPath)
	}
	return configPath, nil
}

// sourceDataDir returns the data dir fileConfig configures, the default when
// it sets none. It does not check it; checkDataDir does.
func sourceDataDir(fileConfig *config.FileConfig) string {
	return path.Clean(cmp.Or(fileConfig.DataDir, defaults.DataDir))
}

// checkDataDir refuses a source data dir retire must never remove, and names
// the rule it broke. Retire removes the whole directory, so one this shallow
// holds files that are not the agent's, even with its host_uuid in there:
//   - not an absolute path
//   - the root, or a directory directly under it
//   - /var/lib, where a data dir with its last element dropped lands
func checkDataDir(dataDir string) error {
	const byHand = "retire by hand"
	switch {
	case !path.IsAbs(dataDir):
		return trace.BadParameter("refusing to operate on the data dir %q: it is not an absolute path; %s", dataDir, byHand)
	case path.Dir(dataDir) == "/":
		return trace.BadParameter("refusing to operate on the data dir %q: the root and the directories directly under it hold files that are not the agent's; %s", dataDir, byHand)
	case dataDir == path.Dir(defaults.DataDir):
		return trace.BadParameter("refusing to operate on the data dir %q: it is the parent of the default data dir %q; %s", dataDir, defaults.DataDir, byHand)
	}
	return nil
}

// checkScopedAgent checks the scoped install carries the target agent, that
// its unit is enabled and active, and that its config neither resolves to nor
// reads anything inside the data dir of the source config at sourceConfigPath,
// which retire removes. It changes nothing.
func (h hostEnv) checkScopedAgent(ctx context.Context, targetHostUUID, sourceConfigPath string) error {
	if got := h.readTrimmed(path.Join(scopedDataDir, hostid.FileName)); got != targetHostUUID {
		return trace.NotFound("no install at teleport_migrated carries target agent %s; retire by hand", targetHostUUID)
	}
	// The state itself, not unitEnabled's boolean, so the refusal can name it.
	state, err := h.unitProperty(ctx, scopedUnit, "UnitFileState")
	if err != nil {
		return trace.Wrap(err)
	}
	if state != "enabled" {
		return trace.CompareFailed("%s is %s, so the scoped agent would not come back after a reboot; run systemctl enable %s, then retire", scopedUnit, cmp.Or(state, "unknown"), scopedUnit)
	}
	active, err := h.unitActive(ctx, scopedUnit)
	if err != nil {
		return trace.Wrap(err)
	}
	if !active {
		return trace.CompareFailed("%s is not active, so systemd is not running the scoped agent; run it under %s, then retire", scopedUnit, scopedUnit)
	}
	scopedConfig, err := os.ReadFile(h.path(scopedConfigPath))
	if err != nil || strings.TrimSpace(string(scopedConfig)) == "" {
		return trace.NotFound("could not read %s, which the scoped agent needs once the source is gone; restore it, restart %s, then retire", scopedConfigPath, scopedUnit)
	}
	// The running agent may have started from an older copy; this one is what
	// a restart reads.
	if _, err := h.readAgentConfig(scopedConfigPath); err != nil {
		return trace.Wrap(err, "%s could not restart from %s once the source is gone; fix the file, restart %s, then retire", scopedUnit, scopedConfigPath, scopedUnit)
	}
	if !h.exists(sourceConfigPath) {
		// A source that is already gone has no data dir to read from.
		return nil
	}
	fileConfig, err := h.readAgentConfig(sourceConfigPath)
	if err != nil {
		return trace.Wrap(err)
	}
	dataDir := sourceDataDir(fileConfig)
	// The checks below compare paths against the data dir, so it is vetted here
	// as well as in removeSource.
	if err := checkDataDir(dataDir); err != nil {
		return trace.Wrap(err, "reading %s", sourceConfigPath)
	}
	// Apply writes the scoped config in place, but a symlink to a file inside
	// the data dir would go with it.
	if h.exists(dataDir) {
		resolvedConfig, err := filepath.EvalSymlinks(h.path(scopedConfigPath))
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		resolvedDataDir, err := filepath.EvalSymlinks(h.path(dataDir))
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		if strings.HasPrefix(resolvedConfig, resolvedDataDir+"/") {
			return trace.BadParameter("%s links to a file inside the source data dir %s, which retire removes; make it a regular file outside it, restart %s, then retire", scopedConfigPath, dataDir, scopedUnit)
		}
	}
	refs, err := dataDirRefs(dataDir, string(scopedConfig))
	if err != nil {
		return trace.Wrap(err, "reading %s", scopedConfigPath)
	}
	if len(refs) > 0 {
		return trace.BadParameter("%s reads %s inside the source data dir %s, which retire removes; move them outside it, update the config, restart %s, then retire", scopedConfigPath, strings.Join(refs, ", "), dataDir, scopedUnit)
	}
	return nil
}

// alreadyRetired reports whether the source agent is gone: no config, no
// default install, and teleport.service neither enabled nor active. A systemd
// that cannot be asked is an error, never a source treated as gone.
func (h hostEnv) alreadyRetired(ctx context.Context, sourceConfigPath string) (bool, error) {
	if h.exists(sourceConfigPath) || h.exists(sourceInstallDir) {
		return false, nil
	}
	active, err := h.unitActive(ctx, "teleport.service")
	if err != nil {
		return false, trace.Wrap(err)
	}
	enabled, err := h.unitEnabled(ctx, "teleport.service")
	if err != nil {
		return false, trace.Wrap(err)
	}
	return !active && !enabled, nil
}

// removeSource removes the source agent: its default install (through
// teleport-update uninstall --force), config and data dir. Every check that
// can run before anything is stopped runs first. A source already gone changes
// nothing.
func (h hostEnv) removeSource(ctx context.Context, sourceHostUUID, sourceConfigPath string) error {
	gone, err := h.alreadyRetired(ctx, sourceConfigPath)
	if err != nil {
		return trace.Wrap(err, "checking whether the source agent is already gone")
	}
	if gone {
		return nil
	}
	// Read once, before the stop: a config that no longer parses, or that
	// gained a service since preflight, is refused while the source still runs.
	fileConfig, err := h.readAgentConfig(sourceConfigPath)
	if err != nil {
		return trace.Wrap(err)
	}
	dataDir, err := h.checkSourceRemoval(ctx, fileConfig, sourceHostUUID, sourceConfigPath)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := h.stopSource(ctx); err != nil {
		return trace.Wrap(err)
	}
	// The data dir goes first. alreadyRetired reads a missing config as a
	// source already gone, so with the config removed first, a removal cut
	// short would leave <dataDir>.retired on a host the next run calls retired.
	if err := h.removeDataDir(dataDir); err != nil {
		return trace.Wrap(err, "removing the source data dir")
	}
	if err := os.Remove(h.path(sourceConfigPath)); err != nil && !os.IsNotExist(err) {
		return trace.Wrap(trace.ConvertSystemError(err), "removing the source config")
	}
	return nil
}

// checkSourceRemoval returns the data dir fileConfig configures, once it has
// checked what removeSource needs before it stops or removes anything:
//   - the config runs no service besides ssh_service, the one the scoped agent
//     is known to carry
//   - the data dir is one retire may remove, and does not overlap the scoped
//     install
//   - the data dir on disk is the source's own, with nothing mounted at or
//     under it
//
// Returning the dir keeps removeSource from using one that failed a check. It
// changes nothing.
func (h hostEnv) checkSourceRemoval(ctx context.Context, fileConfig *config.FileConfig, sourceHostUUID, sourceConfigPath string) (string, error) {
	if err := RefuseOtherServices(enabledServices(fileConfig)); err != nil {
		return "", trace.Wrap(err)
	}
	// Read again here, not taken from checkScopedAgent: configuration
	// management may have rewritten the config with a new data_dir since.
	dataDir := sourceDataDir(fileConfig)
	if err := checkDataDir(dataDir); err != nil {
		return "", trace.Wrap(err, "reading %s", sourceConfigPath)
	}
	if err := refuseOverlap(dataDir, sourceConfigPath); err != nil {
		return "", trace.Wrap(err)
	}
	// The unit may already be stopped and the data dir gone from an earlier
	// run; short of that, the data dir must be the source's own.
	if h.exists(dataDir) {
		if err := h.checkDataDirOnDisk(ctx, dataDir, sourceHostUUID); err != nil {
			return "", trace.Wrap(err)
		}
	} else {
		active, err := h.unitActive(ctx, "teleport.service")
		if err != nil {
			return "", trace.Wrap(err)
		}
		if active {
			return "", trace.CompareFailed("teleport.service is running but %q is gone; run plan --check --resource node/%s", dataDir, sourceHostUUID)
		}
	}
	// os.RemoveAll does not stop at a mount, so one under the data dir would
	// lose its contents.
	mounts, err := h.mountsUnder(dataDir)
	if err != nil {
		return "", trace.Wrap(err, "reading /proc/self/mountinfo; check for mounts under %q by hand, then retire", dataDir)
	}
	if len(mounts) > 0 {
		return "", trace.BadParameter("%q is mounted under the data dir %q, which retire removes; unmount it, then retire", mounts, dataDir)
	}
	return dataDir, nil
}

// checkDataDirOnDisk checks the data dir that exists at dataDir is the source
// agent's own and that retire can remove all of it: no symlink in its path, the
// source agent's host_uuid in it, not a mount point, and no <dataDir>.retired
// beside it.
func (h hostEnv) checkDataDirOnDisk(ctx context.Context, dataDir, sourceHostUUID string) error {
	// Removing a symlink removes only the link, which would leave the source
	// agent's identity where it points, and mountinfo lists mounts by their
	// resolved path, so a symlink anywhere above hides them.
	root, err := filepath.EvalSymlinks(cmp.Or(h.rootPath, "/"))
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	resolved, err := filepath.EvalSymlinks(h.path(dataDir))
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	if resolved != filepath.Join(root, dataDir) {
		return trace.BadParameter("%q goes through a symlink, so retire could remove only the link or miss a mount inside it; retire by hand", dataDir)
	}
	got := h.readTrimmed(path.Join(dataDir, hostid.FileName))
	switch {
	case got == "":
		return trace.BadParameter("%q has no host_uuid; retire by hand", dataDir)
	case got != sourceHostUUID:
		return trace.BadParameter("%q holds host_uuid %q, expected %q", dataDir, got, sourceHostUUID)
	}
	if mount, err := h.isMountPoint(ctx, h.path(dataDir)); err != nil || mount {
		return trace.BadParameter("%q is a mount point, or its mount could not be checked; retire by hand", dataDir)
	}
	// A run cut short leaves .retired only once the data dir is gone, so one
	// beside the data dir is not retire's to remove.
	if h.exists(dataDir + ".retired") {
		return trace.BadParameter("%q already exists, and retire moves the data dir there before removing it; move it away, then retire", dataDir+".retired")
	}
	return nil
}

// stopSource stops and disables teleport.service and removes the default
// install teleport-update owns. The check that the unit stopped runs before the
// disable, so a refusal there leaves it enabled.
func (h hostEnv) stopSource(ctx context.Context) error {
	const restart = "teleport.service is stopped but still enabled; run systemctl start teleport.service to bring the source agent back"
	// A unit that fails to stop is caught by the stopped check below.
	_ = h.runCommand(ctx, "systemctl", "stop", "teleport.service")
	stopped, state, err := h.unitStopped(ctx, "teleport.service")
	if err != nil {
		return trace.Wrap(err, "nothing was removed, and the stop of teleport.service could not be confirmed; %s", restart)
	}
	switch {
	case !stopped && state == "active":
		return trace.CompareFailed("teleport.service is still active after systemctl stop teleport.service; something restarts it (often configuration management); stop that, then retire again")
	case !stopped:
		// The stop landed, or something raced it, and systemd has not settled
		// on inactive or failed yet.
		return trace.CompareFailed("teleport.service is %s after systemctl stop teleport.service, so it is still starting or shutting down; wait for it, then retire again; %s", state, restart)
	}
	// disable fails once an earlier retire's teleport-update uninstall removed
	// the unit, so the result is checked instead of its exit code.
	_ = h.runCommand(ctx, "systemctl", "disable", "teleport.service")
	enabled, err := h.unitEnabled(ctx, "teleport.service")
	if err != nil {
		return trace.Wrap(err, "nothing was removed; %s", restart)
	}
	if enabled {
		return trace.CompareFailed("systemctl disable teleport.service left it enabled; disable it, then retire; %s", restart)
	}
	if h.exists(sourceInstallDir) {
		if err := h.runCommand(ctx, "teleport-update", "uninstall", "--force"); err != nil {
			return trace.Wrap(err, "teleport.service is stopped and disabled; run systemctl enable teleport.service, then systemctl start teleport.service, to bring the source agent back")
		}
	}
	return nil
}

// removeDataDir removes the source agent's data dir, renaming it to
// <dataDir>.retired first, so a removal cut short leaves nothing half-removed
// in place. Where rename cannot work (EXDEV on an overlayfs root), the data dir
// is removed where it is and that guarantee is given up. A data dir already
// gone is not an error.
func (h hostEnv) removeDataDir(dataDir string) error {
	retired := h.path(dataDir + ".retired")
	if h.exists(dataDir) {
		if err := os.Rename(h.path(dataDir), retired); errors.Is(err, syscall.EXDEV) {
			if err := os.RemoveAll(h.path(dataDir)); err != nil {
				return trace.ConvertSystemError(err)
			}
		} else if err != nil {
			return trace.ConvertSystemError(err)
		}
	}
	return trace.Wrap(trace.ConvertSystemError(os.RemoveAll(retired)))
}

// refuseOverlap refuses when the source agent's config is the scoped agent's,
// or its data dir is, contains or lies inside the scoped data dir, config or
// install dir, since removing one would remove the other.
func refuseOverlap(dataDir, configPath string) error {
	if path.Clean(configPath) == scopedConfigPath {
		return trace.BadParameter("source config %s is the scoped agent's config; handle this host by hand", configPath)
	}
	cleanDataDir := path.Clean(dataDir)
	for _, scopedPath := range []string{scopedDataDir, scopedConfigPath, scopedInstallDir} {
		switch {
		case cleanDataDir == scopedPath:
			return trace.BadParameter("source data dir %s is the scoped agent's %s; handle this host by hand", dataDir, scopedPath)
		case cleanDataDir == "/" || strings.HasPrefix(scopedPath, cleanDataDir+"/"):
			return trace.BadParameter("source data dir %s contains the scoped agent's %s; handle this host by hand", dataDir, scopedPath)
		case strings.HasPrefix(cleanDataDir, scopedPath+"/"):
			return trace.BadParameter("source data dir %s is inside the scoped agent's %s; handle this host by hand", dataDir, scopedPath)
		}
	}
	return nil
}

// isMountPoint reports whether dir is a mount point. It asks mountpoint when
// installed, which also sees a bind mount on the same device: exit 0 is a
// mount and 32, util-linux's code, is not. Any other exit (busybox and
// sysvinit use 1 for "not a mount", util-linux for an error) or a signal
// falls back to comparing dir's device with its parent's, as does a host
// without mountpoint.
func (h hostEnv) isMountPoint(ctx context.Context, dir string) (bool, error) {
	if bin, err := h.lookPath("mountpoint"); err == nil {
		err := h.run(ctx, exec.CommandContext(ctx, bin, "-q", dir))
		// Any exit error, not only *exec.ExitError.
		exitErr, isExit := errors.AsType[interface {
			error
			ExitCode() int
		}](err)
		switch {
		case err == nil:
			return true, nil
		case isExit && exitErr.ExitCode() == 32:
			return false, nil
		}
	}
	same, err := pathsOnSameDevice(dir, filepath.Dir(dir))
	if err != nil {
		return false, trace.Wrap(err)
	}
	return !same, nil
}

// mountsUnder returns the mount points below dir that /proc/self/mountinfo
// lists, not dir itself.
func (h hostEnv) mountsUnder(dir string) ([]string, error) {
	f, err := os.Open(h.path("/proc/self/mountinfo"))
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	defer f.Close()
	var mounts []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		// The fifth field is the mount point, with space, tab, newline and
		// backslash octal-escaped.
		fields := strings.Fields(scanner.Text())
		if len(fields) < 5 {
			continue
		}
		if mount := mountinfoUnescaper.Replace(fields[4]); strings.HasPrefix(mount, dir+"/") {
			mounts = append(mounts, mount)
		}
	}
	return mounts, trace.Wrap(scanner.Err())
}

// mountinfoUnescaper undoes the kernel's escaping of mountinfo paths.
var mountinfoUnescaper = strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)

// dataDirRefs returns, sorted and distinct, the paths inside dataDir, or
// equal to it, that any scalar in the YAML document raw names. Every scalar
// is checked, rather than known file fields, so a field added later is
// caught too. It does not catch a path inside a longer scalar, a relative
// path, or one that reaches the data dir through a symlink.
func dataDirRefs(dataDir, raw string) ([]string, error) {
	clean := path.Clean(dataDir)
	prefix := clean + "/"
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, trace.Wrap(err)
	}
	var refs []string
	for _, value := range scalarValues(&doc) {
		if cleaned := path.Clean(value); cleaned == clean || strings.HasPrefix(cleaned, prefix) {
			refs = append(refs, cleaned)
		}
	}
	slices.Sort(refs)
	return slices.Compact(refs), nil
}

// scalarValues returns the value of every scalar under node.
func scalarValues(node *yaml.Node) []string {
	var values []string
	if node.Kind == yaml.ScalarNode {
		values = append(values, node.Value)
	}
	for _, child := range node.Content {
		values = append(values, scalarValues(child)...)
	}
	return values
}

// RefuseOtherServices refuses services that hold any besides ssh_service:
// retire cannot yet confirm that the scoped agent registered them on the
// target, so removing the source could lose them.
func RefuseOtherServices(services []string) error {
	var others []string
	for _, service := range services {
		if service != "ssh_service" {
			others = append(others, service)
		}
	}
	if len(others) > 0 {
		return trace.BadParameter("also runs %s, whose registration on the target retire cannot confirm yet; retire this host by hand", strings.Join(others, ", "))
	}
	return nil
}
