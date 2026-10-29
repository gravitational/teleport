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

package migrate

import (
	"errors"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gravitational/trace"
	"go.yaml.in/yaml/v3"

	"github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/parse"
)

// ClusterConfig identifies one cluster: its proxy address and the identity file
// that authenticates to it.
type ClusterConfig struct {
	// ProxyServer is the proxy's public address, host:port.
	ProxyServer string `yaml:"proxy_server"`
	// IdentityFile is the path to an identity file (tctl auth sign) for the
	// cluster; LoadConfig resolves a relative one.
	IdentityFile string `yaml:"identity_file"`
}

// HostMapping adds the hosts selected by SourceLabels into TargetScope.
type HostMapping struct {
	// SourceLabels is "*" or a comma-separated k=v list; every listed key
	// must match exactly. LoadConfig rewrites it in sorted form.
	SourceLabels string `yaml:"source_labels" json:"source_labels"`
	// TargetScope is the scope the selected resources join on the target.
	TargetScope string `yaml:"target_scope" json:"target_scope"`

	// labels and wildcard are set by parse. An unparsed HostMapping matches
	// nothing.
	labels   map[string]string
	wildcard bool
}

// Config describes one migration: two clusters and the mappings between them.
type Config struct {
	// Source is the cluster the hosts leave.
	Source ClusterConfig `yaml:"source"`
	// Target is the cluster they join.
	Target ClusterConfig `yaml:"target"`
	// Optional: empty means ~/.teleport-migrate. Otherwise, it is the directory
	// that holds the state file.
	StateDir string `yaml:"state_dir"`
	// Migrations are the mappings; a source host belongs to at most one.
	Migrations []HostMapping `yaml:"migrations"`
}

// LoadConfig reads, validates and defaults the migration config file at
// path. Unknown keys are an error. Relative source.identity_file,
// target.identity_file and state_dir paths resolve against the file's
// directory; an unset state_dir defaults to ~/.teleport-migrate. Every
// mapping is parsed and its source_labels rewritten with sorted keys and no
// spaces.
func LoadConfig(path string) (*Config, error) {
	f, err := utils.OpenFileAllowingUnsafeLinks(path)
	if err != nil {
		return nil, trace.Wrap(err, "opening %q", path)
	}
	defer f.Close()
	var cfg Config
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, trace.BadParameter("config file %q is empty", path)
		}
		return nil, trace.Wrap(err, "parsing %q", path)
	}
	dir := filepath.Dir(path)
	cfg.Source.IdentityFile = resolvePath(dir, cfg.Source.IdentityFile)
	cfg.Target.IdentityFile = resolvePath(dir, cfg.Target.IdentityFile)
	cfg.StateDir = resolvePath(dir, cfg.StateDir)
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err, "validating %q", path)
	}
	return &cfg, nil
}

func resolvePath(dir, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(dir, p)
}

// checkAndSetDefaults validates the config, defaults state_dir, and parses
// every mapping.
func (c *Config) checkAndSetDefaults() error {
	switch {
	case c.Source.ProxyServer == "":
		return trace.BadParameter("source proxy_server is required")
	case c.Source.IdentityFile == "":
		return trace.BadParameter("source identity_file is required")
	case c.Target.ProxyServer == "":
		return trace.BadParameter("target proxy_server is required")
	case c.Target.IdentityFile == "":
		return trace.BadParameter("target identity_file is required")
	case len(c.Migrations) == 0:
		return trace.BadParameter("at least one migration is required")
	}
	if c.StateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return trace.Wrap(err, "defaulting state_dir")
		}
		c.StateDir = filepath.Join(home, ".teleport-migrate")
	}
	for i := range c.Migrations {
		mapping := &c.Migrations[i]
		if err := mapping.parse(); err != nil {
			return trace.Wrap(err, "validating migrations[%d]", i)
		}
		// Two mappings that are guaranteed to select the same host cannot
		// both be right. Selectors that only overlap on some hosts (team=a
		// and env=prod) are decided per host by plan.
		for j := range i {
			switch other := &c.Migrations[j]; {
			case other.covers(mapping):
				return trace.BadParameter("migrations[%d] (%s) selects every host that migrations[%d] (%s) selects; a host can only exist in a single scope", j, other, i, mapping)
			case mapping.covers(other):
				return trace.BadParameter("migrations[%d] (%s) selects every host that migrations[%d] (%s) selects; a host can only exist in a single scope", i, mapping, j, other)
			}
		}
	}
	return nil
}

// parse validates the mapping and rewrites the selector with sorted keys, so
// ordering and spacing do not change the state file name.
func (m *HostMapping) parse() error {
	if err := scopes.StrongValidate(m.TargetScope); err != nil {
		return trace.Wrap(err, "validating target_scope")
	}
	s := strings.TrimSpace(m.SourceLabels)
	switch s {
	case types.Wildcard:
		m.labels, m.wildcard, m.SourceLabels = nil, true, types.Wildcard
		return nil
	case "":
		return trace.BadParameter("source_labels is required; use %q to select every host", types.Wildcard)
	}
	// The multi-value parser keeps a repeated key's values, where
	// LabelSelectorSpec would silently keep the last one.
	multi, err := parse.MultiValueLabelSelectorSpec(s)
	if err != nil {
		return trace.Wrap(err, "parsing source_labels")
	}
	labels := make(map[string]string, len(multi))
	pairs := make([]string, 0, len(multi))
	for _, k := range slices.Sorted(maps.Keys(multi)) {
		if len(multi[k]) != 1 {
			return trace.BadParameter("source label %q is given more than once with different values", k)
		}
		v := multi[k][0]
		if k == "" || v == "" || strings.ContainsAny(k+v, ",=\n") {
			return trace.BadParameter("source label %q=%q: key and value must be non-empty and free of ',', '=' and newlines", k, v)
		}
		labels[k] = v
		pairs = append(pairs, k+"="+v)
	}
	m.labels, m.wildcard, m.SourceLabels = labels, false, strings.Join(pairs, ",")
	return nil
}

// matches reports whether labels select a resource for this mapping.
func (m HostMapping) matches(labels map[string]string) bool {
	if m.wildcard {
		return true
	}
	if m.labels == nil {
		return false
	}
	for k, v := range m.labels {
		if labels[k] != v {
			return false
		}
	}
	return true
}

// covers reports whether every host o selects, m selects too: m is the
// wildcard, or every label m requires o requires as well. Both must be
// parsed.
func (m HostMapping) covers(other *HostMapping) bool {
	return m.wildcard || (!other.wildcard && m.matches(other.labels))
}

// String renders the mapping for messages.
func (m HostMapping) String() string {
	return m.SourceLabels + " -> " + m.TargetScope
}
