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
	"os"
	"path/filepath"
	"strings"

	"github.com/google/renameio/v2"
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/config/yamlmod"
	"github.com/gravitational/teleport/lib/defaults"
)

// modifyFlags holds all flags for the "teleport configure-modify" command.
type modifyFlags struct {
	input          string
	output         string
	overwrite      bool
	enableService  []string
	disableService []string
	set            []string
	unset          []string
	roles          string

	// Named value flags for the fields a cluster migration / re-enrollment touches.
	// Anything not covered here can still be edited via --set/--unset.
	clusterName string
	token       string
	joinMethod  string
	authServer  string
	proxy       string
	dataDir     string
	caPin       string
	version     string
}

// flagPathMap maps CLI flag names to their YAML dot-paths.
var flagPathMap = map[string]string{
	"cluster-name": "teleport.cluster_name",
	"join-method":  "teleport.join_params.method",
	"auth-server":  "teleport.auth_server",
	"proxy":        "teleport.proxy_server",
	"data-dir":     "teleport.data_dir",
	"ca-pin":       "teleport.ca_pin",
	"version":      "version",
}

// modifyRoleServiceMap maps role names to their YAML service section keys.
var modifyRoleServiceMap = map[string]string{
	defaults.RoleNode:           "ssh_service",
	defaults.RoleProxy:          "proxy_service",
	defaults.RoleAuthService:    "auth_service",
	defaults.RoleApp:            "app_service",
	defaults.RoleDatabase:       "db_service",
	"kube":                      "kubernetes_service",
	defaults.RoleWindowsDesktop: "windows_desktop_service",
	defaults.RoleDiscovery:      "discovery_service",
}

// validServices is the set of service names accepted by --enable-service/--disable-service.
var validServices = map[string]bool{
	"ssh_service":             true,
	"proxy_service":           true,
	"auth_service":            true,
	"app_service":             true,
	"db_service":              true,
	"kubernetes_service":      true,
	"windows_desktop_service": true,
	"discovery_service":       true,
	"okta_service":            true,
}

// onConfigModify is the handler for "teleport configure-modify".
func onConfigModify(flags modifyFlags) error {
	if flags.output == "" {
		flags.output = teleport.SchemeStdout
	}
	flags.output = normalizeOutput(flags.output)

	data, err := os.ReadFile(flags.input)
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	doc, err := yamlmod.Parse(data)
	if err != nil {
		return trace.Wrap(err, "parsing input file %s", flags.input)
	}

	if flags.roles != "" {
		if err := applyRoles(doc, flags.roles); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := applyNamedFlags(doc, flags); err != nil {
		return trace.Wrap(err)
	}

	for _, s := range flags.set {
		path, value, ok := strings.Cut(s, "=")
		if !ok {
			return trace.BadParameter("--set value %q must be in format path=value", s)
		}
		if err := yamlmod.Set(doc, path, value); err != nil {
			return trace.Wrap(err, "setting %s", path)
		}
	}

	for _, svc := range flags.enableService {
		if !validServices[svc] {
			return trace.BadParameter("unknown service %q", svc)
		}
		if !yamlmod.Exists(doc, svc) {
			return trace.BadParameter("service %q does not exist in the config; use --roles to create it first", svc)
		}
		if err := yamlmod.SetBool(doc, svc+".enabled", true); err != nil {
			return trace.Wrap(err)
		}
	}

	for _, svc := range flags.disableService {
		if !validServices[svc] {
			return trace.BadParameter("unknown service %q", svc)
		}
		if !yamlmod.Exists(doc, svc) {
			return trace.BadParameter("service %q does not exist in the config", svc)
		}
		if err := yamlmod.SetBool(doc, svc+".enabled", false); err != nil {
			return trace.Wrap(err)
		}
	}

	for _, path := range flags.unset {
		if err := yamlmod.Delete(doc, path); err != nil {
			return trace.Wrap(err, "unsetting %s", path)
		}
	}

	output, err := yamlmod.Render(doc)
	if err != nil {
		return trace.Wrap(err)
	}

	return writeModifyOutput(flags, output)
}

func applyNamedFlags(doc *yaml.Node, flags modifyFlags) error {
	namedValues := map[string]string{
		"cluster-name": flags.clusterName,
		"join-method":  flags.joinMethod,
		"auth-server":  flags.authServer,
		"proxy":        flags.proxy,
		"data-dir":     flags.dataDir,
		"ca-pin":       flags.caPin,
		"version":      flags.version,
	}

	for flagName, value := range namedValues {
		if value == "" {
			continue
		}
		path, ok := flagPathMap[flagName]
		if !ok {
			return trace.BadParameter("no path mapping for flag %q", flagName)
		}
		if err := yamlmod.Set(doc, path, value); err != nil {
			return trace.Wrap(err, "setting %s via --%s", path, flagName)
		}
	}

	// For v3 configs, auth_server and proxy_server are mutually exclusive, and the
	// legacy plural auth_servers field is rejected entirely (see applyAuthOrProxyAddress).
	// When either endpoint is explicitly set, drop the opposing endpoint and the legacy
	// auth_servers field to avoid validation errors on the modified config.
	if flags.proxy != "" || flags.authServer != "" {
		if err := yamlmod.Delete(doc, "teleport.auth_servers"); err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err, "removing legacy teleport.auth_servers")
		}
	}
	if flags.proxy != "" {
		if err := yamlmod.Delete(doc, "teleport.auth_server"); err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err, "removing teleport.auth_server when setting proxy_server")
		}
	}
	if flags.authServer != "" {
		if err := yamlmod.Delete(doc, "teleport.proxy_server"); err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err, "removing teleport.proxy_server when setting auth_server")
		}
	}

	if flags.token != "" {
		if flags.joinMethod == "" && yamlmod.Exists(doc, "teleport.auth_token") {
			// Preserve legacy format: the config uses auth_token and no join-method
			// change was requested, so keep updating the existing field in place.
			if err := yamlmod.Set(doc, "teleport.auth_token", flags.token); err != nil {
				return trace.Wrap(err, "setting teleport.auth_token via --token")
			}
		} else {
			// Use the modern join_params.token_name field, and remove the legacy
			// auth_token if present so both fields don't coexist.
			if err := yamlmod.Set(doc, "teleport.join_params.token_name", flags.token); err != nil {
				return trace.Wrap(err, "setting teleport.join_params.token_name via --token")
			}
			if err := yamlmod.Delete(doc, "teleport.auth_token"); err != nil && !trace.IsNotFound(err) {
				return trace.Wrap(err, "removing teleport.auth_token")
			}
			// Teleport rejects a join_params block with no method set. Default to
			// "token" unless the user already specified --join-method or the config
			// already has a method.
			if flags.joinMethod == "" && !yamlmod.Exists(doc, "teleport.join_params.method") {
				if err := yamlmod.Set(doc, "teleport.join_params.method", "token"); err != nil {
					return trace.Wrap(err, "setting default teleport.join_params.method")
				}
			}
		}
	}

	return nil
}

func applyRoles(doc *yaml.Node, rolesStr string) error {
	roles := strings.Split(rolesStr, ",")
	for _, role := range roles {
		role = strings.TrimSpace(role)
		svcKey, ok := modifyRoleServiceMap[role]
		if !ok {
			return trace.BadParameter("unknown role %q", role)
		}

		if yamlmod.Exists(doc, svcKey) {
			if err := yamlmod.SetBool(doc, svcKey+".enabled", true); err != nil {
				return trace.Wrap(err)
			}
			continue
		}

		snippet, err := generateServiceDefaults(role)
		if err != nil {
			return trace.Wrap(err, "generating defaults for role %q", role)
		}

		srcDoc, err := yamlmod.Parse(snippet)
		if err != nil {
			return trace.Wrap(err, "parsing generated defaults for %q", role)
		}

		if err := yamlmod.Merge(doc, svcKey, srcDoc); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func generateServiceDefaults(role string) ([]byte, error) {
	switch role {
	case defaults.RoleNode:
		return []byte("enabled: \"yes\"\nlisten_addr: 0.0.0.0:3022\n"), nil
	case defaults.RoleProxy:
		return []byte("enabled: \"yes\"\nlisten_addr: 0.0.0.0:3023\nweb_listen_addr: 0.0.0.0:3080\n"), nil
	case defaults.RoleAuthService:
		return []byte("enabled: \"yes\"\nlisten_addr: 0.0.0.0:3025\n"), nil
	case defaults.RoleApp:
		return []byte("enabled: \"yes\"\n"), nil
	case defaults.RoleDatabase:
		return []byte("enabled: \"yes\"\n"), nil
	case "kube":
		return []byte("enabled: \"yes\"\nlisten_addr: 0.0.0.0:3026\n"), nil
	case defaults.RoleWindowsDesktop:
		return []byte("enabled: \"yes\"\n"), nil
	case defaults.RoleDiscovery:
		return []byte("enabled: \"yes\"\n"), nil
	default:
		return nil, trace.BadParameter("no defaults for role %q", role)
	}
}

func writeModifyOutput(flags modifyFlags, data []byte) error {
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
