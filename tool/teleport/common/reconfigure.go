/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package common

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/renameio/v2"
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
)

// reconfigureFlags holds all flags for the "teleport reconfigure" command.
type reconfigureFlags struct {
	input          string
	output         string
	overwrite      bool
	enableService  []string
	disableService []string
	roles          string

	// Named value flags for the fields a cluster migration / re-enrollment
	// touches. Kept intentionally minimal: flags are an API contract that can't
	// be removed without a breaking change.
	token              string
	joinMethod         string
	registrationSecret string
	authServer         string
	proxy              string
	dataDir            string
	configVersion      string

	// SSH networking flags, used to move a side-by-side agent off a colliding
	// listen port (default 3022) and keep it reachable for direct dial.
	sshListenAddr string
	sshPublicAddr string
	forceListen   bool
}

// roleServiceMap maps role names to their YAML service section keys.
var roleServiceMap = map[string]string{
	defaults.RoleNode:           "ssh_service",
	defaults.RoleProxy:          "proxy_service",
	defaults.RoleAuthService:    "auth_service",
	defaults.RoleApp:            "app_service",
	defaults.RoleDatabase:       "db_service",
	"kube":                      "kubernetes_service",
	defaults.RoleWindowsDesktop: "windows_desktop_service",
	defaults.RoleDiscovery:      "discovery_service",
}

// serviceSection returns a pointer to the embedded Service for the named
// service section, or nil if the name is not a service this command manages.
// Every FileConfig service section embeds config.Service, so this is the single
// place enable/disable/roles reach through to flip the "enabled" field.
func serviceSection(fc *config.FileConfig, key string) *config.Service {
	switch key {
	case "ssh_service":
		return &fc.SSH.Service
	case "proxy_service":
		return &fc.Proxy.Service
	case "auth_service":
		return &fc.Auth.Service
	case "app_service":
		return &fc.Apps.Service
	case "db_service":
		return &fc.Databases.Service
	case "kubernetes_service":
		return &fc.Kube.Service
	case "windows_desktop_service":
		return &fc.WindowsDesktop.Service
	case "discovery_service":
		return &fc.Discovery.Service
	case "okta_service":
		return &fc.Okta.Service
	default:
		return nil
	}
}

// onReconfigure is the handler for "teleport reconfigure". It parses the input
// through the same loader Teleport uses at startup (config.ReadConfig), applies
// the requested changes to the typed configuration, and marshals a complete new
// file, leaving the original untouched. Because it round-trips through the typed
// schema rather than editing YAML text, it drops comments and normalizes
// formatting.
func onReconfigure(flags reconfigureFlags) error {
	if flags.output == "" {
		flags.output = teleport.SchemeStdout
	}
	flags.output = normalizeOutput(flags.output)

	data, err := os.ReadFile(flags.input)
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	// Parse with the same loader Teleport uses at startup. Reconfigure only
	// operates on a config it can fully model; one this build can't parse is
	// refused rather than silently rewritten.
	fc, err := config.ReadConfig(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err, "parsing input file %s", flags.input)
	}

	// Record which service sections the input actually contained, so
	// --enable-service/--disable-service can refuse a section that isn't there.
	// The typed struct always has every field, so it can't answer this on its
	// own; --roles adds to the set as it creates sections.
	present, err := presentServices(data)
	if err != nil {
		return trace.Wrap(err)
	}

	if flags.roles != "" {
		if err := applyRoles(fc, present, flags.roles); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := applyNamedFlags(fc, flags); err != nil {
		return trace.Wrap(err)
	}

	for _, svc := range flags.enableService {
		if err := setServiceEnabled(fc, present, svc, true); err != nil {
			return trace.Wrap(err)
		}
	}
	for _, svc := range flags.disableService {
		if err := setServiceEnabled(fc, present, svc, false); err != nil {
			return trace.Wrap(err)
		}
	}

	// Cross-field checks that depend on the config's final state. These mirror
	// rules that only ApplyFileConfig enforces at startup, which we can't run
	// here (see validateReconfigure).
	if err := validateReconfigure(fc); err != nil {
		return trace.Wrap(err)
	}

	output, err := fc.YAMLString()
	if err != nil {
		return trace.Wrap(err)
	}

	// Structural backstop: re-parse the marshaled result to catch YAML corruption
	// or a field this build can't model. This is not a full startup validation.
	// The semantic guarantees (single endpoint, valid version, no auth_token +
	// join_params conflict) come from applying the flags by construction and from
	// validateReconfigure, because the deeper rules live in ApplyFileConfig, which
	// reads host-local files and so can't run against a config bound for another
	// host.
	if _, err := config.ReadConfig(strings.NewReader(output)); err != nil {
		return trace.Wrap(err, "the requested changes produced an invalid configuration")
	}

	return writeReconfigureOutput(flags, []byte(output))
}

// presentServices returns the set of top-level sections present in the raw
// config, used to tell an existing service section from an absent one.
func presentServices(data []byte) (map[string]bool, error) {
	var raw map[string]yaml.Node
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, trace.Wrap(err, "reading config sections")
	}
	present := make(map[string]bool, len(raw))
	for key := range raw {
		present[key] = true
	}
	return present, nil
}

// applyNamedFlags applies the value flags to the typed teleport section, keeping
// the schema invariants lib/config enforces at startup so the output is valid by
// construction: a single auth_server or proxy_server (never both, and never the
// legacy plural auth_servers), and a join_params block that always carries a
// method and never coexists with the legacy auth_token. Cross-field checks that
// depend on the final state (endpoint vs. config version) live in
// validateReconfigure.
func applyNamedFlags(fc *config.FileConfig, flags reconfigureFlags) error {
	g := &fc.Global

	// auth_server and proxy_server are mutually exclusive; reject rather than
	// letting the second flag silently win over the first.
	if flags.proxy != "" && flags.authServer != "" {
		return trace.BadParameter("only one of --proxy or --auth-server may be set")
	}

	if flags.configVersion != "" {
		if err := defaults.ValidateConfigVersion(flags.configVersion); err != nil {
			return trace.Wrap(err)
		}
		fc.Version = flags.configVersion
	}

	if flags.dataDir != "" {
		g.DataDir = flags.dataDir
	}

	// v3 uses one of auth_server / proxy_server and rejects the legacy plural
	// auth_servers. Setting either endpoint drops the opposing one and the
	// legacy field so the result stays valid.
	if flags.proxy != "" {
		g.ProxyServer = flags.proxy
		g.AuthServer = ""
		g.AuthServers = nil
	}
	if flags.authServer != "" {
		g.AuthServer = flags.authServer
		g.ProxyServer = ""
		g.AuthServers = nil
	}

	if err := applyJoinFlags(g, flags); err != nil {
		return trace.Wrap(err)
	}

	applySSHFlags(fc, flags)

	return nil
}

// applyJoinFlags updates the join configuration (auth_token / join_params) from
// the --token, --join-method, and --registration-secret flags. It preserves two
// invariants Teleport enforces at startup but ReadConfig does not: auth_token and
// join_params are never both set, and join_params carries no sub-block belonging
// to a different join method (which would leak the source host's credentials).
func applyJoinFlags(g *config.Global, flags reconfigureFlags) error {
	if flags.joinMethod != "" {
		method := types.JoinMethod(flags.joinMethod)
		if err := types.ValidateJoinMethod(method); err != nil {
			return trace.Wrap(err)
		}
		g.JoinParams.Method = method
	}

	if flags.registrationSecret != "" {
		g.JoinParams.BoundKeypair.RegistrationSecretValue = flags.registrationSecret
		if g.JoinParams.Method == "" {
			g.JoinParams.Method = types.JoinMethodBoundKeypair
		}
	}

	if flags.token != "" {
		// Keep the legacy auth_token format only when the config already uses it
		// and nothing else pulled us into the modern join_params block.
		if flags.joinMethod == "" && flags.registrationSecret == "" &&
			g.AuthToken != "" && g.JoinParams == (config.JoinParams{}) {
			g.AuthToken = flags.token
		} else {
			g.JoinParams.TokenName = flags.token
		}
	}

	// Once any join_params field is set, the block must carry a method and must
	// not coexist with the legacy auth_token. Migrate a stray auth_token into the
	// block rather than emitting a config Teleport rejects at startup (the
	// --join-method-without---token case).
	if g.JoinParams != (config.JoinParams{}) {
		if g.AuthToken != "" {
			if g.JoinParams.TokenName == "" {
				g.JoinParams.TokenName = g.AuthToken
			}
			g.AuthToken = ""
		}
		if g.JoinParams.Method == "" {
			g.JoinParams.Method = types.JoinMethodToken
		}
	}

	// A method change clears the sub-blocks and secrets that belong to a
	// different method, so the source host's credentials don't leak forward.
	if flags.joinMethod != "" {
		clearForeignJoinParams(&g.JoinParams)
	}

	return nil
}

// clearForeignJoinParams zeroes the method-specific sub-blocks and secrets that
// don't belong to jp.Method. token_secret is only meaningful for the token
// method, so it is preserved there and cleared everywhere else.
func clearForeignJoinParams(jp *config.JoinParams) {
	switch jp.Method {
	case types.JoinMethodBoundKeypair:
		jp.Azure = config.AzureJoinParams{}
		jp.TokenSecret = ""
	case types.JoinMethodAzure:
		jp.BoundKeypair = config.BoundKeypairParams{}
		jp.TokenSecret = ""
	case types.JoinMethodToken:
		jp.BoundKeypair = config.BoundKeypairParams{}
		jp.Azure = config.AzureJoinParams{}
	default:
		jp.BoundKeypair = config.BoundKeypairParams{}
		jp.Azure = config.AzureJoinParams{}
		jp.TokenSecret = ""
	}
}

// applySSHFlags updates ssh_service networking fields. These let a side-by-side
// agent move off a colliding listen port (SSH nodes default to 3022) and stay
// reachable for direct dial.
func applySSHFlags(fc *config.FileConfig, flags reconfigureFlags) {
	if flags.sshListenAddr != "" {
		fc.SSH.ListenAddress = flags.sshListenAddr
	}
	if flags.sshPublicAddr != "" {
		fc.SSH.PublicAddr = apiutils.Strings{flags.sshPublicAddr}
	}
	// Only ever enable force_listen; never write false, which would clobber a
	// source config that already sets it (exactly the hosts that collide).
	if flags.forceListen {
		fc.SSH.ForceListen = true
	}
}

// validateReconfigure runs the cross-field checks that depend on the config's
// final state. ReadConfig can't catch these: the rules live in ApplyFileConfig,
// which reads host-local files (proxy https_keypairs, HSM PIN, CA files) and so
// can't be run against a config destined for another host. We mirror only the
// rules reconfigure itself can break.
func validateReconfigure(fc *config.FileConfig) error {
	if fc.ProxyServer != "" || fc.AuthServer != "" {
		if fc.Version != defaults.TeleportConfigVersionV3 {
			return trace.BadParameter(
				"proxy_server/auth_server require config version v3, but the config is %q; pass --config-version v3",
				fc.Version)
		}
	}
	return nil
}

// applyRoles enables the service section for each named role, creating it (as a
// minimal enabled section, with the rest filled by Teleport's own defaults at
// startup) when the input didn't have it.
func applyRoles(fc *config.FileConfig, present map[string]bool, rolesStr string) error {
	for _, role := range strings.Split(rolesStr, ",") {
		role = strings.TrimSpace(role)
		key, ok := roleServiceMap[role]
		if !ok {
			return trace.BadParameter("unknown role %q", role)
		}
		svc := serviceSection(fc, key)
		if svc == nil {
			return trace.BadParameter("unknown service %q for role %q", key, role)
		}
		svc.EnabledFlag = "yes"
		present[key] = true
	}
	return nil
}

// setServiceEnabled toggles a service section's "enabled" field. It refuses a
// section that wasn't in the input (or created by --roles), so a mistyped
// service name is an error rather than a new, half-empty section.
func setServiceEnabled(fc *config.FileConfig, present map[string]bool, key string, enabled bool) error {
	svc := serviceSection(fc, key)
	if svc == nil {
		return trace.BadParameter("unknown service %q", key)
	}
	if !present[key] {
		if enabled {
			return trace.BadParameter("service %q does not exist in the config; use --roles to create it first", key)
		}
		return trace.BadParameter("service %q does not exist in the config", key)
	}
	if enabled {
		svc.EnabledFlag = "yes"
	} else {
		svc.EnabledFlag = "no"
	}
	return nil
}

func writeReconfigureOutput(flags reconfigureFlags, data []byte) error {
	output := flags.output
	switch output {
	case teleport.SchemeStdout, "stdout://":
		_, err := os.Stdout.Write(data)
		return trace.Wrap(err)
	}

	path := output
	if strings.HasPrefix(output, "file://") {
		path = strings.TrimPrefix(output, "file://")
	}

	if !filepath.IsAbs(path) {
		return trace.BadParameter("output path must be absolute: %s", path)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return trace.ConvertSystemError(err)
	}

	if flags.overwrite {
		// Write to a same-directory temp file and atomically rename over the target,
		// so an interrupted or failed write (ENOSPC, NFS error, signal) never leaves
		// the live config truncated. This is the intended in-place migration path.
		if err := renameio.WriteFile(path, data, teleport.FileMaskOwnerOnly, renameio.WithTempDir(dir)); err != nil {
			return trace.ConvertSystemError(err)
		}
		return nil
	}

	// Use O_EXCL so the kernel rejects symlinks atomically — os.Stat+os.WriteFile
	// has a TOCTOU race where a dangling symlink appears nonexistent but WriteFile
	// follows it to an arbitrary target.
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, teleport.FileMaskOwnerOnly)
	if err != nil {
		err = trace.ConvertSystemError(err)
		if trace.IsAlreadyExists(err) {
			return trace.AlreadyExists("output file %s already exists; use --overwrite to replace it", path)
		}
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return trace.ConvertSystemError(err)
}
