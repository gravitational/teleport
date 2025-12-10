// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

//go:build docs

package utils

import (
	"bytes"
	"cmp"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"gopkg.in/yaml.v3"
)

// formatThreeColMarkdownTable formats the provided row data into a three-column
// Markdown table, minus the header.
func formatThreeColMarkdownTable(rows [][3]string) string {
	var buf bytes.Buffer

	for _, r := range rows {
		fmt.Fprintf(&buf, "\n|%v|%v|%v|", r[0], r[1], r[2])
	}
	return buf.String()
}

// flagsToRows outputs data for a table that lists flags, their default
// values, and help texts.
func flagsToRows(f []*kingpin.FlagModel) [][3]string {
	rows := [][3]string{}

	for _, flag := range f {
		// Skip hidden flags and flags whose only purpose is to expose
		// YAML-based default env variables.
		if flag.Hidden || flag.Name == flag.Envar {
			continue
		}
		flagString := ""
		flagName := flag.Name
		if flag.IsBoolFlag() {
			flagName = "[no-]" + flagName
		}
		if flag.Short != 0 {
			flagString += fmt.Sprintf("`-%c`, `--%s`", flag.Short, flagName)
		} else {
			flagString += fmt.Sprintf("`--%s`", flagName)
		}

		rows = append(rows, [3]string{
			flagString,
			formatDefaultFlagValue(flag),
			formatHelp(flag.Help),
		})
	}
	return rows
}

// anyVisibleFlags returns whether any flags in f are visible, i.e., should be
// included in a table of flags.
func anyVisibleFlags(f []*kingpin.FlagModel) bool {
	return slices.ContainsFunc(f, func(m *kingpin.FlagModel) bool {
		return !m.Hidden
	})
}

// anyEnvVarsForCmd indicates whether at least one of the arguments and flags
// provided exposes an environment variable for configuration.
func anyEnvVarsForCmd(args []*kingpin.ArgModel, flags []*kingpin.FlagModel) bool {
	return slices.ContainsFunc(args, func(arg *kingpin.ArgModel) bool {
		return arg.Envar != ""
	}) || slices.ContainsFunc(flags, func(flag *kingpin.FlagModel) bool {
		return flag.Envar != ""
	})
}

// argsToRows outputs data for a table that lists arguments, their default
// values, and help texts.
func argsToRows(a []*kingpin.ArgModel) [][3]string {
	rows := [][3]string{}
	for _, arg := range a {
		if arg.Hidden {
			continue
		}

		// Some commands declare empty argument names and help texts as
		// a hack to allow arbitrary values. Indicate this in the table
		// as a special case.
		argName := cmp.Or(arg.Name, "args")

		help := "Arbitrary arguments"
		if arg.Help != "" {
			help = formatHelp(arg.Help)
		}

		rows = append(rows, [3]string{
			argName,
			formatDefaultArgValue(arg),
			help,
		})
	}
	return rows
}

// envVarsToRows prints table data for a list of environment variables, their
// default values, and help texts.
func envVarsToRows(args []*kingpin.ArgModel, flags []*kingpin.FlagModel) [][3]string {
	rows := [][3]string{}
	for _, arg := range args {
		if arg.Hidden || arg.Envar == "" {
			continue
		}

		rows = append(rows, [3]string{
			fmt.Sprintf("`%v`", arg.Envar),
			formatDefaultArgValue(arg),
			arg.Help,
		})
	}
	for _, flg := range flags {
		if flg.Hidden || flg.Envar == "" {
			continue
		}

		rows = append(rows, [3]string{
			fmt.Sprintf("`%v`", flg.Envar),
			formatDefaultFlagValue(flg),
			flg.Help,
		})
	}
	return rows
}

// sortcommandsByName sorts the commands in cmds by their full command names,
// including all subcommands.
func sortCommandsByName(cmds []*kingpin.CmdModel) []*kingpin.CmdModel {
	slices.SortStableFunc(cmds, func(a, b *kingpin.CmdModel) int {
		switch {
		case a.FullCommand < b.FullCommand:
			return -1
		case a.FullCommand > b.FullCommand:
			return 1
		default:
			return 0
		}
	})
	return cmds
}

// formatDefaultFlagValue returns the default value of flag to display in a
// table of flags. Assumes that a Boolean flag is false unless it is true by
// default.
func formatDefaultFlagValue(flag *kingpin.FlagModel) string {
	switch {
	case len(flag.Default) == 0 && flag.IsBoolFlag():
		return "`false`"
	case len(flag.Default) > 0:
		ret := make([]string, len(flag.Default))
		for i, v := range flag.Default {
			ret[i] = fmt.Sprintf("`%v`", v)
		}
		return strings.Join(ret, ",")
	default:
		return "none"
	}
}

// formatDefaultArgValue returns the default value of arg to display in a table
// of flags. It also indicates whether the value is optional or required.
func formatDefaultArgValue(arg *kingpin.ArgModel) string {
	var ret string
	if len(arg.Default) > 0 {
		def := make([]string, len(arg.Default))
		for i, v := range arg.Default {
			def[i] = fmt.Sprintf("`%v`", v)
		}
		ret = strings.Join(def, ",")
	} else {
		ret = "none"
	}
	if arg.Required {
		ret += " (required)"
	} else {
		ret += " (optional)"
	}

	return ret
}

// repeatableFlag is an interface for flags that can be repeated. Unexported
// type in github.com/alecthomas/kingpin/v2.
type repeatableFlag interface {
	IsCumulative() bool
}

// formatUsageArg prints a command argument to include in a usage snippet.
func formatUsageArg(arg *kingpin.ArgModel) string {
	var ret string
	switch {
	case arg.PlaceHolder != "":
		ret = arg.PlaceHolder
	// Some special cases have empty arg names
	case arg.Name == "":
		ret = "args"
	default:
		ret = "<" + arg.Name + ">"
	}
	if v, ok := arg.Value.(repeatableFlag); ok && v.IsCumulative() {
		ret += "..."
	}
	if !arg.Required {
		ret = "[" + ret + "]"
	}
	return ret
}

// formatHelp prints help text to include in a Markdown table cell. It escapes
// curly braces to avoid breaking the MDX parser, and it escapes pipes to
// avoid breaking the cell.
func formatHelp(help string) string {
	return strings.NewReplacer("{", `\{`, "}", `\}`, "|", `\|`).Replace(help)
}

// docsUsageTemplatePath points to a help text template for CLI reference
// documentation. Intended to be used as the argument to
// *kingpin.Application.UsageTemplate.
var docsUsageTemplatePath = filepath.Join("lib", "utils", "docs-usage.md.tmpl")

// updateAppUsageTemplatePath updates the app usage template to print a reference
// guide for the CLI application. It reads the template from r.
func updateAppUsageTemplate(r io.Reader, app *kingpin.Application) {
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		panic(fmt.Sprintf("unable to read from the docs usage template: %v", err))
	}

	app.UsageFuncs(map[string]any{
		"AnyEnvVarsForCmd":            anyEnvVarsForCmd,
		"AnyVisibleFlags":             anyVisibleFlags,
		"ArgsToRows":                  argsToRows,
		"EnvVarsToRows":               envVarsToRows,
		"FlagsToRows":                 flagsToRows,
		"FormatThreeColMarkdownTable": formatThreeColMarkdownTable,
		"FormatUsageArg":              formatUsageArg,
		"SortCommandsByName":          sortCommandsByName,
	})
	app.UsageTemplate(buf.String())
}

// envVarDefault represents the structure of environment variable defaults in YAML files.
type envVarDefault struct {
	Description string `yaml:"description"`
	Default     string `yaml:"default"`
	Type        string `yaml:"type"`
}

// loadDefaultEnvVars loads possible default environment variables defined in a YAML file
// that matches the application name.
func loadDefaultEnvVars(appName string) ([][4]string, error) {
	pathname := filepath.Join("lib", "utils", "docenvdefaults", appName+".yaml")
	data, err := os.ReadFile(pathname)
	if err != nil || len(data) == 0 {
		return nil, nil
	}

	envDefaults := make(map[string]envVarDefault)
	if err := yaml.Unmarshal(data, &envDefaults); err != nil {
		return nil, fmt.Errorf("unable to parse YAML from %s: %w", pathname, err)
	}

	if len(envDefaults) == 0 {
		return nil, nil
	}

	rows := make([][4]string, 0, len(envDefaults))
	for envVar, def := range envDefaults {
		if def.Description == "" || def.Default == "" || def.Type == "" {
			return nil, fmt.Errorf("invalid YAML structure in %s: entry %q is missing one of required fields 'description', 'default' or 'type'", pathname, envVar)
		}
		rows = append(rows, [4]string{
			envVar,
			strings.Trim(def.Default, "`"),
			def.Description,
			def.Type,
		})
	}

	slices.SortFunc(rows, func(a, b [4]string) int {
		return cmp.Compare(a[0], b[0])
	})

	return rows, nil
}

// UpdateAppUsageTemplate updates the app usage template to print a reference
// guide for the CLI application.
func UpdateAppUsageTemplate(app *kingpin.Application, _ []string) {
	defaultEnvVars, err := loadDefaultEnvVars(app.Name)
	if err != nil {
		panic(err)
	}

	existingEnvVars := make(map[string]struct{})

	for _, flag := range app.Model().Flags {
		if flag.Envar != "" {
			existingEnvVars[flag.Envar] = struct{}{}
		}
	}

	for _, envRow := range defaultEnvVars {
		envVarName := envRow[0]
		defaultVal := envRow[1]
		description := envRow[2]
		envVarType := envRow[3]

		// Check if the flag already exists in the app model to avoid
		// duplicate flag errors.
		if _, flagExists := existingEnvVars[envVarName]; flagExists {
			continue
		}

		// If the flag does not exist, create it with the default value
		// and description from the YAML file.
		flag := app.Flag(envVarName, description).
			Envar(envVarName).
			Default(defaultVal)
		if envVarType == "bool" {
			flag.Bool()
		} else {
			flag.String()
		}
	}

	// Panic when failing to open or read from the docs usage template since
	// we need to keep the signature of UpdateAppUsageTemplate consistent
	// with the one included without build tags, i.e., with no return value.
	f, err := os.Open(docsUsageTemplatePath)
	if err != nil {
		panic(fmt.Sprintf("unable to open the docs usage template at %v: %v", docsUsageTemplatePath, err))
	}

	updateAppUsageTemplate(f, app)
}
