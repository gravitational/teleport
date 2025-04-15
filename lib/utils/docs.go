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

package utils

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/alecthomas/kingpin/v2"
)

// formatThreeColMarkdownTable formats the provided row data into a three-column
// Markdown table, minus the header.
func formatThreeColMarkdownTable(rows [][3]string) string {
	buf := bytes.NewBuffer(nil)
	for _, r := range rows {
		buf.WriteString("\n|" + r[0] + "|" + r[1] + "|" + r[2] + "|")
	}
	return buf.String()
}

// flagsToColumns outputs data for a table that lists flags, their default
// values, and help texts.
func flagsToColumns(f []*kingpin.FlagModel) [][3]string {
	rows := [][3]string{}
	haveShort := false
	for _, flag := range f {
		if flag.Short != 0 {
			haveShort = true
			break
		}
	}
	for _, flag := range f {
		if flag.Hidden {
			continue
		}
		rows = append(rows, [3]string{
			formatFlagForTable(haveShort, flag),
			formatDefaultFlagValue(flag),
			formatHelp(flag.Help),
		})
	}
	return rows
}

// anyVisibleFlags returns whether any flags in f are visible, i.e., should be
// included in a table of flags.
func anyVisibleFlags(f []*kingpin.FlagModel) bool {
	for _, l := range f {
		if !l.Hidden {
			return true
		}
	}
	return false
}

// anyEnvVarsForCmd indicates whether at least one of the arguments and flags
// provided exposes an environment variable for configuration.
func anyEnvVarsForCmd(args []*kingpin.ArgModel, flags []*kingpin.FlagModel) bool {
	for _, a := range args {
		if a.Envar != "" {
			return true
		}
	}
	for _, f := range flags {
		if f.Envar != "" {
			return true
		}
	}
	return false
}

// argsToColumns outputs data for a table that lists arguments, their default
// values, and help texts.
func argsToColumns(a []*kingpin.ArgModel) [][3]string {
	rows := [][3]string{}
	for _, arg := range a {
		if arg.Hidden {
			continue
		}

		// Some commands declare empty argument names and help texts as
		// a hack to allow arbitrary values. Indicate this in the table
		// as a special case.
		var argName string
		if arg.Name != "" {
			argName = arg.Name
		} else {
			argName = "args"
		}

		var help string
		if arg.Help != "" {
			help = formatHelp(arg.Help)
		} else {
			help = "Arbitrary arguments"
		}

		rows = append(rows, [3]string{
			argName,
			formatDefaultArgValue(arg),
			help,
		})
	}
	return rows
}

// envVarsToColumns prints table data for a list of environment variables, their
// default values, and help texts.
func envVarsToColumns(args []*kingpin.ArgModel, flags []*kingpin.FlagModel) [][3]string {
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

// formatFlagForTable includes the names of flags so we can display them in a
// table of flag information.
func formatFlagForTable(haveShort bool, flag *kingpin.FlagModel) string {
	flagString := ""
	flagName := flag.Name
	if flag.IsBoolFlag() {
		flagName = "[no-]" + flagName
	}
	if flag.Short != 0 {
		flagString += fmt.Sprintf("`-%c`, `--%s`", flag.Short, flagName)
	} else {
		if haveShort {
			flagString += fmt.Sprintf("    `--%s`", flagName)
		} else {
			flagString += fmt.Sprintf("`--%s`", flagName)
		}
	}
	return flagString
}

// cmdModelCollection is used to sort CmdModels for display in the CLI reference
// docs page.
type cmdModelCollection []*kingpin.CmdModel

// Len returns the length of c. Required to implement sort.Interface.
func (c cmdModelCollection) Len() int {
	return len(c)
}

// Less returns whether command name i is lexicographically ordered before
// command name j. Required to implement sort.Interface.
func (c cmdModelCollection) Less(i, j int) bool {
	return c[i].FullCommand < c[j].FullCommand
}

// Swap reverses the ordering of i and j within c. Required to implement
// sort.Interface.
func (c cmdModelCollection) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

// sortcommandsByName sorts the commands in cmds by their full command names,
// including all subcommands.
func sortCommandsByName(cmds []*kingpin.CmdModel) []*kingpin.CmdModel {
	col := cmdModelCollection(cmds)
	sort.Stable(&col)
	return []*kingpin.CmdModel(col)
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
// curly braces to avoid breaking the MDX parser.
func formatHelp(help string) string {
	return strings.NewReplacer("{", `\{`, "}", `\}`).Replace(help)
}

// docsUsageTemplate is a help text template for CLI reference documentation.
// Intended to be used as the argument to *kingpin.Application.UsageTemplate.
//
//go:embed docs-usage.md.tmpl
var docsUsageTemplate string

// PrintCLIDocs updates app's kingpin usage template to print a docs page, then
// prints the usage string to usageWriter.
func PrintCLIDocs(usageWriter io.Writer, app *kingpin.Application) {
	app.UsageWriter(usageWriter)
	app.UsageFuncs(map[string]any{
		"AnyEnvVarsForCmd":            anyEnvVarsForCmd,
		"AnyVisibleFlags":             anyVisibleFlags,
		"ArgsToColumns":               argsToColumns,
		"EnvVarsToColumns":            envVarsToColumns,
		"FlagsToColumns":              flagsToColumns,
		"FormatThreeColMarkdownTable": formatThreeColMarkdownTable,
		"FormatUsageArg":              formatUsageArg,
		"SortCommandsByName":          sortCommandsByName,
	})
	app.UsageTemplate(docsUsageTemplate)
	app.Usage(nil)
}
