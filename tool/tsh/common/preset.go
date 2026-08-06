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

package common

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	renameio "github.com/google/renameio/v2/maybe"
	"github.com/gravitational/trace"
	yamlv2 "gopkg.in/yaml.v2"
	yamlv3 "gopkg.in/yaml.v3"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

type presetCommands struct {
	ls  *presetLSCommand
	add *presetAddCommand
	use *presetUseCommand
}

func newPresetCommand(app *kingpin.Application) presetCommands {
	preset := app.Command("preset", "Manage tsh configuration presets.")
	return presetCommands{
		ls:  newPresetLSCommand(preset),
		add: newPresetAddCommand(preset),
		use: newPresetUseCommand(preset),
	}
}

func (c presetCommands) isManagementCommand(command string) bool {
	return command == c.ls.FullCommand() || command == c.add.FullCommand() || command == c.use.FullCommand()
}

type presetLSCommand struct {
	*kingpin.CmdClause
	format string
}

func newPresetLSCommand(parent *kingpin.CmdClause) *presetLSCommand {
	c := &presetLSCommand{
		CmdClause: parent.Command("ls", "List configured tsh presets."),
	}
	c.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).
		Short('f').
		Default(teleport.Text).
		EnumVar(&c.format, defaults.DefaultFormats...)
	return c
}

type presetInfo struct {
	Name    string `json:"name" yaml:"name"`
	Default bool   `json:"default" yaml:"default"`
	Proxy   string `json:"proxy,omitempty" yaml:"proxy,omitempty"`
	Cluster string `json:"cluster,omitempty" yaml:"cluster,omitempty"`
	User    string `json:"user,omitempty" yaml:"user,omitempty"`
	Home    string `json:"home,omitempty" yaml:"home,omitempty"`
}

func (c *presetLSCommand) run(cf *CLIConf) error {
	names := make([]string, 0, len(cf.TSHConfig.Presets))
	for name := range cf.TSHConfig.Presets {
		names = append(names, name)
	}
	sort.Strings(names)

	presets := make([]presetInfo, 0, len(names))
	for _, name := range names {
		preset := cf.TSHConfig.Presets[name]
		presets = append(presets, presetInfo{
			Name:    name,
			Default: name == cf.TSHConfig.DefaultPreset,
			Proxy:   preset.Proxy,
			Cluster: preset.Cluster,
			User:    preset.User,
			Home:    preset.Home,
		})
	}

	switch strings.ToLower(c.format) {
	case teleport.Text, "":
		table := asciitable.MakeTable([]string{"Preset", "Default", "Proxy", "Cluster", "User"})
		for _, preset := range presets {
			isDefault := ""
			if preset.Default {
				isDefault = "*"
			}
			table.AddRow([]string{preset.Name, isDefault, preset.Proxy, preset.Cluster, preset.User})
		}
		fmt.Fprint(cf.Stdout(), table.AsBuffer().String())
	case teleport.JSON:
		out, err := utils.FastMarshalIndent(presets, "", "  ")
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprintln(cf.Stdout(), string(out))
	case teleport.YAML:
		out, err := yamlv2.Marshal(presets)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprint(cf.Stdout(), string(out))
	default:
		return trace.BadParameter("unsupported format %q", c.format)
	}

	return nil
}

type presetAddCommand struct {
	*kingpin.CmdClause
	name     string
	proxyURL string
}

func newPresetAddCommand(parent *kingpin.CmdClause) *presetAddCommand {
	c := &presetAddCommand{
		CmdClause: parent.Command("add", "Add a tsh preset for a profile URL."),
	}
	c.Arg("name", "Name of the preset to add.").Required().StringVar(&c.name)
	c.Arg("profile URL", "Profile URL shown by tsh status.").Required().StringVar(&c.proxyURL)
	return c
}

func (c *presetAddCommand) run(cf *CLIConf) error {
	if strings.TrimSpace(c.name) == "" {
		return trace.BadParameter("preset name cannot be empty")
	}
	if _, ok := cf.TSHConfig.Presets[c.name]; ok {
		return trace.AlreadyExists("preset %q already exists", c.name)
	}

	proxy, err := normalizePresetProxy(c.proxyURL)
	if err != nil {
		return trace.Wrap(err, "invalid profile URL")
	}

	configPath := filepath.Join(profile.FullProfilePath(cf.HomePath), client.TSHConfigPath)
	if err := addPreset(configPath, c.name, proxy); err != nil {
		return trace.Wrap(err, "adding preset")
	}

	fmt.Fprintf(cf.Stdout(), "Preset %q added for proxy %q.\n", c.name, proxy)
	return nil
}

type presetUseCommand struct {
	*kingpin.CmdClause
	name string
}

func newPresetUseCommand(parent *kingpin.CmdClause) *presetUseCommand {
	c := &presetUseCommand{
		CmdClause: parent.Command("use", "Set the default tsh preset."),
	}
	c.Arg("name", "Name of the preset to use by default.").Required().StringVar(&c.name)
	return c
}

func (c *presetUseCommand) run(cf *CLIConf) error {
	if _, err := cf.TSHConfig.GetPreset(c.name); err != nil {
		return trace.Wrap(err)
	}

	configPath := filepath.Join(profile.FullProfilePath(cf.HomePath), client.TSHConfigPath)
	if err := setDefaultPreset(configPath, c.name); err != nil {
		return trace.Wrap(err, "setting default preset")
	}

	fmt.Fprintf(cf.Stdout(), "Default preset set to %q.\n", c.name)
	return nil
}

// setDefaultPreset updates only default_preset in a tsh YAML config. YAML v3
// nodes are used to retain unrelated fields and comments. A file lock prevents
// concurrent tsh processes from losing each other's config updates.
func setDefaultPreset(configPath, name string) (err error) {
	return updateTSHConfig(configPath, func(root *yamlv3.Node) error {
		value, found, err := yamlMappingValue(root, "default_preset")
		if err != nil {
			return trace.Wrap(err)
		}
		if found {
			value.Kind = yamlv3.ScalarNode
			value.Tag = "!!str"
			value.Value = name
			return nil
		}

		root.Content = append(root.Content,
			&yamlv3.Node{Kind: yamlv3.ScalarNode, Tag: "!!str", Value: "default_preset"},
			&yamlv3.Node{Kind: yamlv3.ScalarNode, Tag: "!!str", Value: name},
		)
		return nil
	})
}

func addPreset(configPath, name, proxy string) error {
	return updateTSHConfig(configPath, func(root *yamlv3.Node) error {
		presets, found, err := yamlMappingValue(root, "presets")
		if err != nil {
			return trace.Wrap(err)
		}
		if !found {
			presets = &yamlv3.Node{Kind: yamlv3.MappingNode, Tag: "!!map"}
			root.Content = append(root.Content,
				&yamlv3.Node{Kind: yamlv3.ScalarNode, Tag: "!!str", Value: "presets"},
				presets,
			)
		}
		if presets.Kind != yamlv3.MappingNode {
			return trace.BadParameter("tsh config presets field must contain a YAML mapping")
		}
		if _, found, err := yamlMappingValue(presets, name); err != nil {
			return trace.Wrap(err)
		} else if found {
			return trace.AlreadyExists("preset %q already exists", name)
		}

		preset := &yamlv3.Node{Kind: yamlv3.MappingNode, Tag: "!!map", Content: []*yamlv3.Node{
			{Kind: yamlv3.ScalarNode, Tag: "!!str", Value: "proxy"},
			{Kind: yamlv3.ScalarNode, Tag: "!!str", Value: proxy},
		}}
		presets.Content = append(presets.Content,
			&yamlv3.Node{Kind: yamlv3.ScalarNode, Tag: "!!str", Value: name},
			preset,
		)
		return nil
	})
}

func yamlMappingValue(mapping *yamlv3.Node, key string) (*yamlv3.Node, bool, error) {
	var value *yamlv3.Node
	for i := 0; i < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value != key {
			continue
		}
		if value != nil {
			return nil, false, trace.BadParameter("tsh config contains duplicate %q fields", key)
		}
		value = mapping.Content[i+1]
	}
	return value, value != nil, nil
}

// updateTSHConfig updates a tsh YAML config while retaining unrelated fields
// and comments. A file lock prevents concurrent tsh processes from losing each
// other's config updates.
func updateTSHConfig(configPath string, update func(*yamlv3.Node) error) (err error) {
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return trace.ConvertSystemError(err)
	}

	unlock, err := utils.FSWriteLock(configPath)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		err = trace.NewAggregate(err, unlock())
	}()

	data, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return trace.ConvertSystemError(err)
	}

	var document yamlv3.Node
	if len(strings.TrimSpace(string(data))) != 0 {
		if err := yamlv3.Unmarshal(data, &document); err != nil {
			return trace.Wrap(err)
		}
	}
	if len(document.Content) == 0 {
		document.Kind = yamlv3.DocumentNode
		document.Content = []*yamlv3.Node{{Kind: yamlv3.MappingNode, Tag: "!!map"}}
	}
	if document.Kind != yamlv3.DocumentNode || len(document.Content) != 1 || document.Content[0].Kind != yamlv3.MappingNode {
		return trace.BadParameter("tsh config must contain a YAML mapping")
	}

	root := document.Content[0]
	if err := update(root); err != nil {
		return trace.Wrap(err)
	}

	out, err := yamlv3.Marshal(&document)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := renameio.WriteFile(configPath, out, 0o600); err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

// normalizePresetProxy converts either a profile URL from tsh status or a tsh
// proxy address into a canonical web proxy host:port used for matching.
func normalizePresetProxy(raw string) (string, error) {
	proxy := strings.TrimSpace(raw)
	if proxy == "" {
		return "", trace.BadParameter("proxy cannot be empty")
	}

	if strings.Contains(proxy, "://") {
		parsedURL, err := url.Parse(proxy)
		if err != nil {
			return "", trace.BadParameter("invalid proxy URL %q", raw)
		}
		scheme := strings.ToLower(parsedURL.Scheme)
		if scheme != "https" && scheme != "http" {
			return "", trace.BadParameter("proxy URL must use http or https")
		}
		if parsedURL.Host == "" || parsedURL.User != nil || parsedURL.RawQuery != "" || parsedURL.Fragment != "" || (parsedURL.Path != "" && parsedURL.Path != "/") {
			return "", trace.BadParameter("proxy URL must contain only a scheme, host, and optional port")
		}

		host := parsedURL.Hostname()
		port := parsedURL.Port()
		if port == "" {
			if strings.HasSuffix(parsedURL.Host, ":") {
				return "", trace.BadParameter("invalid proxy URL %q", raw)
			}
			if scheme == "https" {
				port = "443"
			} else {
				port = "80"
			}
		}
		return canonicalProxyAddress(host, port)
	}

	if strings.ContainsAny(proxy, "/?#@") || strings.IndexFunc(proxy, func(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }) != -1 {
		return "", trace.BadParameter("invalid proxy address %q", raw)
	}
	parsedProxy, err := client.ParseProxyHost(proxy)
	if err != nil {
		return "", trace.Wrap(err)
	}
	host, port, err := net.SplitHostPort(parsedProxy.WebProxyAddr)
	if err != nil {
		return "", trace.BadParameter("invalid proxy address %q", raw)
	}
	return canonicalProxyAddress(host, port)
}

// presetProxyMatchesProfile reports whether a preset proxy selects a profile's
// web proxy endpoint. A preset without an explicit web port matches by host so
// existing shorthand configurations such as "proxy.example.com" match status
// URLs using either the cloud :443 port or the self-hosted :3080 default.
func presetProxyMatchesProfile(presetProxy, profileURL string) bool {
	profileAddress, err := normalizePresetProxy(profileURL)
	if err != nil {
		return false
	}
	presetAddress, err := normalizePresetProxy(presetProxy)
	if err != nil {
		return false
	}
	if presetAddress == profileAddress {
		return true
	}

	// URL forms have a scheme-defined port even when it is omitted from the
	// text, so only tsh's host-style proxy syntax can be port-agnostic.
	if strings.Contains(presetProxy, "://") {
		return false
	}
	parsedPreset, err := client.ParseProxyHost(strings.TrimSpace(presetProxy))
	if err != nil || !parsedPreset.UsingDefaultWebProxyPort {
		return false
	}
	presetHost, _, err := net.SplitHostPort(presetAddress)
	if err != nil {
		return false
	}
	profileHost, _, err := net.SplitHostPort(profileAddress)
	if err != nil {
		return false
	}
	return strings.EqualFold(presetHost, profileHost)
}

func canonicalProxyAddress(host, port string) (string, error) {
	if host == "" || strings.ContainsAny(host, "/,?#@") || strings.IndexFunc(host, func(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }) != -1 {
		return "", trace.BadParameter("proxy host cannot be empty or contain whitespace")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return "", trace.BadParameter("invalid proxy port %q", port)
	}
	return net.JoinHostPort(strings.ToLower(host), strconv.Itoa(portNumber)), nil
}
