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

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/config/yamlmod"
	"github.com/gravitational/teleport/lib/defaults"
)

// modifyFlags holds all flags for the "teleport configure modify" command.
type modifyFlags struct {
	input          string
	output         string
	overwrite      bool
	enableService  []string
	disableService []string
	set            []string
	unset          []string
	roles          string

	// Named value flags (mirror configure flags)
	clusterName   string
	token         string
	joinMethod    string
	authServer    string
	proxy         string
	publicAddr    string
	dataDir       string
	nodeName      string
	nodeLabels    string
	caPin         string
	certFile      string
	keyFile       string
	acmeEnabled   bool
	acmeEmail     string
	version       string
	appName       string
	appURI        string
	mcpDemoServer bool
}

// flagPathMap maps CLI flag names to their YAML dot-paths.
var flagPathMap = map[string]string{
	"cluster-name": "teleport.cluster_name",
	"token":        "teleport.auth_token",
	"join-method":  "teleport.join_params.method",
	"auth-server":  "teleport.auth_server",
	"proxy":        "teleport.proxy_server",
	"public-addr":  "proxy_service.public_addr",
	"data-dir":     "teleport.data_dir",
	"node-name":    "teleport.nodename",
	"ca-pin":       "teleport.ca_pin",
	"cert-file":    "proxy_service.https_cert_file",
	"key-file":     "proxy_service.https_key_file",
	"acme-email":   "proxy_service.acme.email",
	"version":      "version",
	"app-name":     "app_service.apps[0].name",
	"app-uri":      "app_service.apps[0].uri",
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

// onConfigModify is the handler for "teleport configure modify".
func onConfigModify(flags modifyFlags) error {
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
		"token":        flags.token,
		"join-method":  flags.joinMethod,
		"auth-server":  flags.authServer,
		"proxy":        flags.proxy,
		"public-addr":  flags.publicAddr,
		"data-dir":     flags.dataDir,
		"node-name":    flags.nodeName,
		"ca-pin":       flags.caPin,
		"cert-file":    flags.certFile,
		"key-file":     flags.keyFile,
		"acme-email":   flags.acmeEmail,
		"version":      flags.version,
		"app-name":     flags.appName,
		"app-uri":      flags.appURI,
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

	if flags.nodeLabels != "" {
		labels, err := parseLabels(flags.nodeLabels)
		if err != nil {
			return trace.Wrap(err, "parsing --node-labels")
		}
		if err := yamlmod.SetMap(doc, "ssh_service.labels", labels); err != nil {
			return trace.Wrap(err, "setting ssh_service.labels via --node-labels")
		}
	}

	if flags.acmeEnabled {
		if err := yamlmod.SetBool(doc, "proxy_service.acme.enabled", true); err != nil {
			return trace.Wrap(err)
		}
	}
	if flags.mcpDemoServer {
		if err := yamlmod.SetBool(doc, "app_service.apps[0].mcp_demo_server", true); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// parseLabels parses a comma-separated list of key=value pairs into a map.
func parseLabels(s string) (map[string]string, error) {
	labels := make(map[string]string)
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		k, v, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, trace.BadParameter("label %q must be in format key=value", pair)
		}
		labels[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	if len(labels) == 0 {
		return nil, trace.BadParameter("no valid labels found in %q", s)
	}
	return labels, nil
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

	if !flags.overwrite {
		if _, err := os.Stat(path); err == nil {
			return trace.AlreadyExists("output file %s already exists; use --overwrite to replace it", path)
		}
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return trace.ConvertSystemError(err)
	}

	if err := os.WriteFile(path, data, teleport.FileMaskOwnerOnly); err != nil {
		return trace.ConvertSystemError(err)
	}

	return nil
}
