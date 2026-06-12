/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package transform

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
)

// MigrateParams controls ApplyMigration.
type MigrateParams struct {
	InstallSuffix   string
	ProxyServer     string
	AuthServer      string
	JoinMethod      types.JoinMethod
	TokenName       string
	TokenSecretPath string
	DataDir         string
	DisableServices []string
	ExtraSSHLabels  map[string]string
}

// MigrationResult describes a migrated config and the edits made to it.
type MigrationResult struct {
	Document                *Document
	ServicesDisabled        []string
	DisableServicesNotFound []string
	LogPathsChanged         []PathChange
	PIDFileChanged          *PathChange
	ListenerWarnings        []string
	Notices                 []string
}

// PathChange describes a value changed at a YAML path.
type PathChange struct {
	Path string
	Old  string
	New  string
}

var disableServiceSections = map[string]string{
	"ssh":             "ssh_service",
	"kube":            "kubernetes_service",
	"app":             "app_service",
	"db":              "db_service",
	"discovery":       "discovery_service",
	"windows_desktop": "windows_desktop_service",
	"okta":            "okta_service",
	"jamf":            "jamf_service",
	"debug":           "debug_service",
}

var defaultEnabledServiceSections = map[string]struct{}{
	"auth_service":  {},
	"proxy_service": {},
	"ssh_service":   {},
	"debug_service": {},
}

var listenerFields = []string{
	"listen_addr",
	"web_listen_addr",
	"tunnel_listen_addr",
	"peer_listen_addr",
	"kube_listen_addr",
	"mysql_listen_addr",
	"postgres_listen_addr",
	"mongo_listen_addr",
	"transport_listen_addr",
}

// ApplyMigration returns a config transformed for a suffixed migrated agent.
func ApplyMigration(doc *Document, p MigrateParams) (*MigrationResult, error) {
	if doc == nil {
		return nil, trace.BadParameter("missing input document")
	}
	out := doc.clone()
	result := &MigrationResult{Document: out}

	var oldPIDFile string
	if pidFile, ok := out.get("teleport", "pid_file"); ok {
		oldPIDFile = strings.TrimSpace(pidFile.Value)
	}

	for _, path := range [][]string{
		{"teleport", "auth_server"},
		{"teleport", "auth_servers"},
		{"teleport", "proxy_server"},
		{"teleport", "auth_token"},
		{"teleport", "token"},
		{"teleport", "join_params"},
		{"teleport", "ca_pin"},
		{"teleport", "data_dir"},
		{"teleport", "pid_file"},
	} {
		out.delete(path...)
	}

	switch {
	case p.ProxyServer != "" && p.AuthServer != "":
		return nil, trace.BadParameter("only one of proxy server or auth server can be set")
	case p.ProxyServer != "":
		if err := out.set(p.ProxyServer, "teleport", "proxy_server"); err != nil {
			return nil, trace.Wrap(err)
		}
	case p.AuthServer != "":
		if err := out.set(p.AuthServer, "teleport", "auth_server"); err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		return nil, trace.BadParameter("one of proxy server or auth server must be set")
	}

	for _, set := range []struct {
		value any
		path  []string
	}{
		{p.DataDir, []string{"teleport", "data_dir"}},
		{string(p.JoinMethod), []string{"teleport", "join_params", "method"}},
		{p.TokenName, []string{"teleport", "join_params", "token_name"}},
		{"no", []string{"auth_service", "enabled"}},
		{"no", []string{"proxy_service", "enabled"}},
	} {
		if err := out.set(set.value, set.path...); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if p.JoinMethod == types.JoinMethodToken && p.TokenSecretPath != "" {
		if err := out.set(p.TokenSecretPath, "teleport", "join_params", "token_secret"); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	for _, service := range p.DisableServices {
		section, ok := disableServiceSections[service]
		if !ok {
			return nil, trace.BadParameter("unsupported service %q in disable services", service)
		}
		if _, ok := out.get(section); !ok {
			if _, defaultEnabled := defaultEnabledServiceSections[section]; !defaultEnabled {
				result.DisableServicesNotFound = append(result.DisableServicesNotFound, service)
				continue
			}
		}
		if err := out.set("no", section, "enabled"); err != nil {
			return nil, trace.Wrap(err)
		}
		result.ServicesDisabled = append(result.ServicesDisabled, section)
	}

	if err := out.mergeSSHLabels(p.ExtraSSHLabels); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := out.resolveConflicts(p.InstallSuffix, oldPIDFile, result); err != nil {
		return nil, trace.Wrap(err)
	}
	return result, nil
}

func (d *Document) mergeSSHLabels(labels map[string]string) error {
	if len(labels) == 0 {
		return nil
	}
	sshService, ok := d.get("ssh_service")
	if !ok {
		return trace.BadParameter("cannot place labels: ssh_service is not configured; marker labels are required for verify/decommission")
	}
	if sshService.Kind != yaml.MappingNode {
		return trace.BadParameter("ssh_service must be a mapping")
	}
	if !sectionEnabled(sshService) {
		return trace.BadParameter("cannot place labels: ssh_service is disabled; marker labels are required for verify/decommission")
	}
	labelsNode, ok := d.get("ssh_service", "labels")
	if !ok {
		labelsNode = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		if err := d.set(labelsNode, "ssh_service", "labels"); err != nil {
			return trace.Wrap(err)
		}
		var inserted bool
		labelsNode, inserted = d.get("ssh_service", "labels")
		if !inserted {
			return trace.BadParameter("failed to create ssh_service.labels")
		}
	}
	if labelsNode.Kind != yaml.MappingNode {
		return trace.BadParameter("ssh_service.labels must be a mapping")
	}
	for _, key := range slices.Sorted(maps.Keys(labels)) {
		value := labels[key]
		if existing, ok := mappingValue(labelsNode, key); ok {
			if existing.Value == value {
				continue
			}
			return trace.BadParameter("ssh_service.labels.%s already has value %q", key, existing.Value)
		}
		setMappingValue(labelsNode, key, scalarString(value))
	}
	return nil
}

func (d *Document) resolveConflicts(installSuffix, oldPIDFile string, result *MigrationResult) error {
	if diagAddr, ok := d.get("teleport", "diag_addr"); ok && strings.TrimSpace(diagAddr.Value) != "" {
		old := diagAddr.Value
		d.delete("teleport", "diag_addr")
		result.Notices = append(result.Notices, fmt.Sprintf("diag_addr removed from migrated config: would conflict with the original agent (%s).", old))
	}
	if err := d.suffixLogPath(installSuffix, result); err != nil {
		return trace.Wrap(err)
	}
	if err := d.resolvePIDFile(installSuffix, oldPIDFile, result); err != nil {
		return trace.Wrap(err)
	}
	for _, section := range slices.Sorted(maps.Keys(disableServiceSections)) {
		d.checkSectionListeners(disableServiceSections[section], result)
	}
	return nil
}

func (d *Document) suffixLogPath(installSuffix string, result *MigrationResult) error {
	logOutput, ok := d.get("teleport", "log", "output")
	if !ok || logOutput.Kind != yaml.ScalarNode {
		return nil
	}
	oldPath := strings.TrimSpace(logOutput.Value)
	switch oldPath {
	case "", "stderr", "stdout", "syslog":
		return nil
	}
	if installSuffix == "" {
		return trace.BadParameter("teleport.log.output %q must be changed, but --install-suffix was not provided", oldPath)
	}
	ext := filepath.Ext(oldPath)
	newPath := strings.TrimSuffix(oldPath, ext) + "_" + installSuffix + ext
	logOutput.Value = newPath
	result.LogPathsChanged = append(result.LogPathsChanged, PathChange{
		Path: "teleport.log.output",
		Old:  oldPath,
		New:  newPath,
	})
	return nil
}

func (d *Document) resolvePIDFile(installSuffix, oldPath string, result *MigrationResult) error {
	if oldPath == "" {
		return nil
	}
	if installSuffix == "" {
		return trace.BadParameter("teleport.pid_file %q must be changed, but --install-suffix was not provided", oldPath)
	}
	ext := filepath.Ext(oldPath)
	newPath := strings.TrimSuffix(oldPath, ext) + "_" + installSuffix + ext
	if err := d.set(newPath, "teleport", "pid_file"); err != nil {
		result.Notices = append(result.Notices, "removed teleport.pid_file to avoid two agents sharing the same PID file.")
		return nil
	}
	result.PIDFileChanged = &PathChange{
		Path: "teleport.pid_file",
		Old:  oldPath,
		New:  newPath,
	}
	return nil
}

func (d *Document) checkSectionListeners(section string, result *MigrationResult) {
	sectionNode, ok := d.get(section)
	if !ok || sectionNode.Kind != yaml.MappingNode {
		return
	}
	for _, field := range listenerFields {
		listenNode, ok := mappingValue(sectionNode, field)
		if !ok || strings.TrimSpace(listenNode.Value) == "" {
			continue
		}
		if sectionEnabled(sectionNode) {
			result.ListenerWarnings = append(result.ListenerWarnings, fmt.Sprintf("%s.%s %q may be bound by both agents.", section, field, listenNode.Value))
		}
	}
}

func sectionEnabled(sectionNode *yaml.Node) bool {
	enabled, ok := mappingValue(sectionNode, "enabled")
	if !ok || strings.TrimSpace(enabled.Value) == "" {
		return true
	}
	// Match fileconf.Service.Enabled: apiutils.ParseBool semantics,
	// unparseable values mean disabled.
	parsed, err := apiutils.ParseBool(strings.TrimSpace(enabled.Value))
	return err == nil && parsed
}
