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
	"net"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
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
	Document         *Document
	FieldsRemoved    []string
	ServicesDisabled []string
	LogPathsChanged  []PathChange
	Notices          []string
}

// PathChange describes a value changed at a YAML path.
type PathChange struct {
	Path string
	Old  string
	New  string
}

var disableServiceSections = map[string]string{
	"ssh":             "ssh_service",
	"kube":            "kube_service",
	"app":             "app_service",
	"db":              "db_service",
	"discovery":       "discovery_service",
	"windows_desktop": "windows_desktop_service",
	"okta":            "okta_service",
	"jamf":            "jamf_service",
	"debug":           "debug_service",
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
	out := doc.Clone()
	result := &MigrationResult{Document: out}

	oldDataDir := defaults.DataDir
	if dataDirNode, ok := out.Get("teleport", "data_dir"); ok && dataDirNode.Value != "" {
		oldDataDir = dataDirNode.Value
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
	} {
		if out.Delete(path...) {
			result.FieldsRemoved = append(result.FieldsRemoved, strings.Join(path, "."))
		}
	}

	if out.pruneStorage(oldDataDir) {
		result.FieldsRemoved = append(result.FieldsRemoved, "teleport.storage")
	}

	switch {
	case p.ProxyServer != "" && p.AuthServer != "":
		return nil, trace.BadParameter("only one of proxy server or auth server can be set")
	case p.ProxyServer != "":
		if err := out.Set(p.ProxyServer, "teleport", "proxy_server"); err != nil {
			return nil, trace.Wrap(err)
		}
	case p.AuthServer != "":
		if err := out.Set(p.AuthServer, "teleport", "auth_server"); err != nil {
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
		{false, []string{"auth_service", "enabled"}},
		{false, []string{"proxy_service", "enabled"}},
	} {
		if err := out.Set(set.value, set.path...); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if p.JoinMethod == types.JoinMethodToken {
		if err := out.Set(p.TokenSecretPath, "teleport", "join_params", "token_secret"); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	for _, service := range p.DisableServices {
		section, ok := disableServiceSections[service]
		if !ok {
			return nil, trace.BadParameter("unsupported service %q in disable services", service)
		}
		if err := out.Set(false, section, "enabled"); err != nil {
			return nil, trace.Wrap(err)
		}
		result.ServicesDisabled = append(result.ServicesDisabled, section)
	}

	if err := out.mergeSSHLabels(p.ExtraSSHLabels); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := out.resolveConflicts(p.InstallSuffix, result); err != nil {
		return nil, trace.Wrap(err)
	}
	return result, nil
}

func (d *Document) mergeSSHLabels(labels map[string]string) error {
	if len(labels) == 0 {
		return nil
	}
	labelsNode, ok := d.Get("ssh_service", "labels")
	if !ok {
		labelsNode = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		if err := d.Set(labelsNode, "ssh_service", "labels"); err != nil {
			return trace.Wrap(err)
		}
	}
	if labelsNode.Kind != yaml.MappingNode {
		return trace.BadParameter("ssh_service.labels must be a mapping")
	}
	for key, value := range labels {
		if existing, ok := mappingValue(labelsNode, key); ok && existing.Value != value {
			return trace.BadParameter("ssh_service.labels.%s already has value %q", key, existing.Value)
		}
		setMappingValue(labelsNode, key, scalarString(value))
	}
	return nil
}

func (d *Document) pruneStorage(oldDataDir string) bool {
	storage, ok := d.Get("teleport", "storage")
	if !ok {
		return false
	}
	pruned := pruneNodeReferences(storage, oldDataDir)
	if isEmptyCollection(storage) {
		d.Delete("teleport", "storage")
		return true
	}
	return pruned
}

func pruneNodeReferences(node *yaml.Node, needle string) bool {
	if needle == "" {
		return false
	}
	switch node.Kind {
	case yaml.MappingNode:
		pruned := false
		for i := 0; i+1 < len(node.Content); {
			if nodeReferences(node.Content[i+1], needle) {
				node.Content = append(node.Content[:i], node.Content[i+2:]...)
				pruned = true
				continue
			}
			if pruneNodeReferences(node.Content[i+1], needle) {
				pruned = true
			}
			i += 2
		}
		return pruned
	case yaml.SequenceNode:
		pruned := false
		for i := 0; i < len(node.Content); {
			if nodeReferences(node.Content[i], needle) {
				node.Content = append(node.Content[:i], node.Content[i+1:]...)
				pruned = true
				continue
			}
			if pruneNodeReferences(node.Content[i], needle) {
				pruned = true
			}
			i++
		}
		return pruned
	default:
		return false
	}
}

func nodeReferences(node *yaml.Node, needle string) bool {
	switch node.Kind {
	case yaml.ScalarNode:
		return strings.Contains(node.Value, needle)
	case yaml.MappingNode, yaml.SequenceNode:
		for _, child := range node.Content {
			if nodeReferences(child, needle) {
				return true
			}
		}
	}
	return false
}

func isEmptyCollection(node *yaml.Node) bool {
	switch node.Kind {
	case yaml.MappingNode, yaml.SequenceNode:
		return len(node.Content) == 0
	default:
		return false
	}
}

func (d *Document) resolveConflicts(installSuffix string, result *MigrationResult) error {
	if diagAddr, ok := d.Get("teleport", "diag_addr"); ok && strings.TrimSpace(diagAddr.Value) != "" {
		return trace.BadParameter("teleport.diag_addr %q would be inherited by both agents; pass a different address or remove it%s", diagAddr.Value, diagAddrSuggestion(diagAddr.Value))
	}
	if err := d.suffixLogPath(installSuffix, result); err != nil {
		return trace.Wrap(err)
	}
	for section := range disableServiceSections {
		if err := d.checkSectionListeners(disableServiceSections[section]); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (d *Document) suffixLogPath(installSuffix string, result *MigrationResult) error {
	logOutput, ok := d.Get("teleport", "log", "output")
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
	result.Notices = append(result.Notices, "rewrote teleport.log.output to avoid two agents writing the same log file")
	return nil
}

func (d *Document) checkSectionListeners(section string) error {
	sectionNode, ok := d.Get(section)
	if !ok || sectionNode.Kind != yaml.MappingNode {
		return nil
	}
	for _, field := range listenerFields {
		listenNode, ok := mappingValue(sectionNode, field)
		if !ok || strings.TrimSpace(listenNode.Value) == "" {
			continue
		}
		if sectionEnabled(sectionNode) {
			return trace.BadParameter("%s.%s %q would be bound by both agents; pass a different address or remove it", section, field, listenNode.Value)
		}
	}
	return nil
}

func sectionEnabled(sectionNode *yaml.Node) bool {
	enabled, ok := mappingValue(sectionNode, "enabled")
	if !ok {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(enabled.Value)) {
	case "no", "false", "off", "0":
		return false
	case "yes", "true", "on", "1":
		return true
	default:
		parsed, err := strconv.ParseBool(enabled.Value)
		if err != nil {
			return true
		}
		return parsed
	}
}

func diagAddrSuggestion(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return ""
	}
	return "; for example, use " + net.JoinHostPort(host, strconv.Itoa(portNum+1))
}
