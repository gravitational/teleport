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
	"os"
	"path/filepath"
	"sort"
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
	use *presetUseCommand
}

func newPresetCommand(app *kingpin.Application) presetCommands {
	preset := app.Command("preset", "View and select tsh configuration presets.")
	return presetCommands{
		ls:  newPresetLSCommand(preset),
		use: newPresetUseCommand(preset),
	}
}

func (c presetCommands) isManagementCommand(command string) bool {
	return command == c.ls.FullCommand() || command == c.use.FullCommand()
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
	found := false
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value != "default_preset" {
			continue
		}
		if found {
			return trace.BadParameter("tsh config contains duplicate default_preset fields")
		}
		found = true
		root.Content[i+1].Kind = yamlv3.ScalarNode
		root.Content[i+1].Tag = "!!str"
		root.Content[i+1].Value = name
	}
	if !found {
		root.Content = append(root.Content,
			&yamlv3.Node{Kind: yamlv3.ScalarNode, Tag: "!!str", Value: "default_preset"},
			&yamlv3.Node{Kind: yamlv3.ScalarNode, Tag: "!!str", Value: name},
		)
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
