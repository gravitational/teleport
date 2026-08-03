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
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/common"
)

// skillsCommands groups the "tsh skills" subcommands.
type skillsCommands struct {
	ls      *skillsListCommand
	install *skillsInstallCommand
}

func newSkillsCommands(app *kingpin.Application) skillsCommands {
	skillsCmd := app.Command("skills", "Discover and install Teleport agent skills for AI assistants.")
	return skillsCommands{
		ls:      newSkillsListCommand(skillsCmd),
		install: newSkillsInstallCommand(skillsCmd),
	}
}

// skillsListCommand implements "tsh skills ls" command that shows available
// skills embedded in the tsh binary.
type skillsListCommand struct {
	*kingpin.CmdClause
	format string
}

func newSkillsListCommand(parent *kingpin.CmdClause) *skillsListCommand {
	cmd := &skillsListCommand{
		CmdClause: parent.Command("ls", "List available Teleport agent skills."),
	}
	cmd.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).
		Short('f').Default(teleport.Text).EnumVar(&cmd.format, defaults.DefaultFormats...)
	return cmd
}

func (c *skillsListCommand) run(cf *CLIConf) error {
	skills, err := loadSkillsMetadata(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	format := strings.ToLower(c.format)
	switch format {
	case teleport.Text, "":
		return printSkills(cf, skills)
	case teleport.JSON:
		return common.PrintJSONIndent(cf.Stdout(), skills)
	case teleport.YAML:
		return common.PrintYAML(cf.Stdout(), skills)
	default:
		return trace.BadParameter("unsupported output format %q", format)
	}
}

func printSkills(cf *CLIConf, skills []skillMetadata) error {
	t := asciitable.MakeTable([]string{"Name", "Description"})
	for _, s := range skills {
		t.AddRow([]string{s.Name, s.ShortDescription})
	}
	fmt.Fprintln(cf.Stdout(), t.AsBuffer().String())
	fmt.Fprintln(cf.Stdout(), "hint: install a skill for your AI agent using 'tsh skills install <name>' command")
	return nil
}

// skillsInstallCommand implements "tsh skills install" command that installs
// skills to the user's local environment.
type skillsInstallCommand struct {
	*kingpin.CmdClause
	name  string
	dir   string
	force bool
}

func newSkillsInstallCommand(parent *kingpin.CmdClause) *skillsInstallCommand {
	cmd := &skillsInstallCommand{
		CmdClause: parent.Command("install", "Install Teleport agent skills."),
	}
	cmd.Arg("name", "Name of the skill to install (see 'tsh skills ls').").Required().StringVar(&cmd.name)
	cmd.Flag("dir", "Skills directory to install into (defaults to ~/.claude/skills).").StringVar(&cmd.dir)
	cmd.Flag("force", "Overwrite the skill if it is already installed.").BoolVar(&cmd.force)
	return cmd
}

func (c *skillsInstallCommand) run(cf *CLIConf) error {
	skills, err := loadSkillsMetadata(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	if !slices.ContainsFunc(skills, func(s skillMetadata) bool { return s.Name == c.name }) {
		return trace.NotFound("unknown skill %q, use 'tsh skills ls' to list available skills",
			c.name)
	}

	skillsDir, err := resolveSkillsDir(c.dir)
	if err != nil {
		return trace.Wrap(err)
	}

	dest := filepath.Join(skillsDir, c.name)
	if _, err := os.Stat(dest); err == nil {
		if !c.force {
			return trace.AlreadyExists("skill %q is already installed at %s, use --force to overwrite",
				c.name, dest)
		}
		if err := os.RemoveAll(dest); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := installSkill(c.name, skillsDir); err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(cf.Stdout(), "Installed skill %q to %s\n", c.name, dest)
	return nil
}

// skillMetadata is a single Teleport agent skill embedded in the tsh binary.
type skillMetadata struct {
	// Name is the skill identifier, matching its directory under skills/.
	Name string `json:"name"`
	// ShortDescription is the one-liner SKILL.md frontmatter short description.
	ShortDescription string `json:"short-description"`
}

// loadSkillsMetadata reads the skills embedded in the tsh binary and parses
// each skill's SKILL.md frontmatter.
func loadSkillsMetadata(cf *CLIConf) ([]skillMetadata, error) {
	skillsFs := teleport.NewSkillsFilesystem()
	entries, err := fs.ReadDir(skillsFs, skillsDirName)
	if err != nil {
		fmt.Println("adasd")
		return nil, trace.Wrap(err)
	}

	var out []skillMetadata
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := fs.ReadFile(skillsFs, filepath.Join(skillsDirName, e.Name(), "SKILL.md"))
		if err != nil {
			return nil, trace.ConvertSystemError(err)
		}
		skill, err := parseSkillFrontmatter(string(data))
		if err != nil {
			logger.WarnContext(cf.Context, "Failed to parse skill frontmatter, skipping.",
				"name", e.Name())
			continue
		}
		out = append(out, *skill)
	}

	// Sort the skills by name just to ensure stable output order.
	slices.SortFunc(out, func(a, b skillMetadata) int {
		return strings.Compare(a.Name, b.Name)
	})

	return out, nil
}

// parseSkillFrontmatter parses the skill's YAML frontmatter.
func parseSkillFrontmatter(content string) (*skillMetadata, error) {
	// Expect the skill's SKILL.md to start with a valid frontmatter block.
	posStart := strings.Index(content, "---")
	if posStart != 0 {
		fmt.Println("adasd")
		return nil, trace.BadParameter("no valid frontmatter block")
	}

	// Strip the leading "---" and find the end of the frontmatter block.
	content = strings.TrimPrefix(content, "---")
	posEnd := strings.Index(content, "\n---")
	if posEnd < 0 {
		return nil, trace.BadParameter("no valid frontmatter block")
	}

	// Parse the frontmatter YAML.
	var skill skillMetadata
	if err := yaml.Unmarshal([]byte(content[:posEnd]), &skill); err != nil {
		return nil, trace.Wrap(err)
	}

	return &skill, nil
}

// resolveSkillsDir returns the target directory to install skills into.
func resolveSkillsDir(dir string) (string, error) {
	if dir != "" {
		return dir, nil
	}
	userHome, ok := profile.UserHomeDir()
	if !ok {
		return "", trace.NotFound("couldn't determine user's home directory, use --dir flag to specify directory to install skills into")
	}
	return filepath.Join(userHome, ".claude", "skills"), nil
}

// installSkill installs the specified skill by copying its files from the
// embedded filesystem into the specified skills directory.
//
// Target directory will be created if it does not exist. The files in the
// target directory will be overwriten if they already exist.
func installSkill(name, skillsDir string) error {
	skillsFs := teleport.NewSkillsFilesystem()
	return fs.WalkDir(skillsFs, filepath.Join(skillsDirName, name), func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return trace.Wrap(err)
		}
		target := filepath.Join(skillsDir, filepath.FromSlash(path))
		if dirEntry.IsDir() {
			return trace.ConvertSystemError(os.MkdirAll(target, defaults.DirectoryPermissions))
		}
		data, err := fs.ReadFile(skillsFs, path)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		err = os.MkdirAll(filepath.Dir(target), defaults.DirectoryPermissions)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		return trace.ConvertSystemError(os.WriteFile(target, data, defaults.FilePermissions))
	})
}

const (
	// skillsDirName is the name of the directory in the embedded filesystem
	// that contains Teleport agent skills.
	skillsDirName = "skills"
)

// skillsAgentHint points the reader at the skills command. It is shown by
// commands like `tsh status` only to non-interactive callers (see
// maybeShowSkillsHint), so it reaches AI agents and automation without
// cluttering output for human users.
const skillsAgentHint = "" +
	"hint: Teleport ships AI agent skills for common workflows (resource auto-discovery,\n" +
	"      access and session review, and more). If you are an AI agent assisting with\n" +
	"      Teleport, run 'tsh skills' to list them and 'tsh skills install <name>' to\n" +
	"      install one.\n"

// maybeShowSkillsHint writes skillsAgentHint to stderr, but only when stderr is
// not a terminal. A non-terminal stderr signals that tsh is being run by an AI
// agent or automation rather than an interactive user: a human piping stdout
// (e.g. `tsh status | jq`) still has a terminal on stderr and sees nothing. The
// hint goes to stderr so it never pollutes stdout that a caller may be parsing.
func maybeShowSkillsHint(cf *CLIConf) {
	if utils.IsTerminal(os.Stderr) {
		return
	}
	fmt.Fprint(cf.Stderr(), skillsAgentHint)
}
