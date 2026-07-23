/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/skills"
)

// skillsCommandHelp is the top-level help for the `tsh skills` command. It is
// written to be legible to AI agents (Claude Code, Cursor, Codex, Gemini CLI,
// etc.) that run `tsh --help` or `tsh skills --help` while assisting a user:
// the phrasing tells the agent that installable, task-specific skills exist and
// how to install them, so the agent can proactively offer to set them up.
const skillsCommandHelp = "Discover and install Teleport agent skills for AI assistants."

// defaultSkillsDir is the per-user skills directory that Claude Code reads.
// Other agents use different locations; override with --dir.
const defaultSkillsDir = "~/.claude/skills"

// skill describes a single embedded Teleport agent skill.
type skill struct {
	// Name is the skill identifier, matching its directory under skills/.
	Name string `json:"name"`
	// ShortDescription is the one-line SKILL.md frontmatter short-description,
	// used for the ls table. Falls back to the first sentence of Description
	// when absent.
	ShortDescription string `json:"short_description"`
}

// installCommand returns the command a user (or agent) runs to install the skill.
func (s skill) installCommand() string {
	return "tsh skills install " + s.Name
}

// loadSkills reads the embedded skills catalog, parsing each skill's SKILL.md
// frontmatter for its description.
func loadSkills() ([]skill, error) {
	fsys := skills.FS()
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var out []skill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		data, err := fs.ReadFile(fsys, path.Join(name, "SKILL.md"))
		if err != nil {
			return nil, trace.Wrap(err, "reading SKILL.md for %q", name)
		}
		fm := parseSkillFrontmatter(data)
		out = append(out, skill{
			Name:             name,
			ShortDescription: fm.ShortDescription,
		})
	}
	slices.SortFunc(out, func(a, b skill) int {
		return strings.Compare(a.Name, b.Name)
	})
	return out, nil
}

// skillFrontmatter holds the SKILL.md frontmatter fields tsh cares about.
type skillFrontmatter struct {
	ShortDescription string `json:"short-description"`
}

// parseSkillFrontmatter parses the YAML frontmatter (the block delimited by
// "---") at the top of a SKILL.md file. Missing or malformed frontmatter yields
// a zero value rather than an error.
func parseSkillFrontmatter(content []byte) skillFrontmatter {
	text := string(content)
	if !strings.HasPrefix(text, "---") {
		return skillFrontmatter{}
	}
	rest := strings.TrimPrefix(text, "---")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return skillFrontmatter{}
	}

	var fm skillFrontmatter
	if err := yaml.Unmarshal([]byte(rest[:end]), &fm); err != nil {
		return skillFrontmatter{}
	}
	fm.ShortDescription = strings.TrimSpace(fm.ShortDescription)
	return fm
}

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

// skillsCommands groups the `tsh skills` subcommands.
type skillsCommands struct {
	ls      *skillsListCommand
	install *skillsInstallCommand
}

func newSkillsCommands(app *kingpin.Application) skillsCommands {
	skillsCmd := app.Command("skills", skillsCommandHelp)
	return skillsCommands{
		ls:      newSkillsListCommand(skillsCmd),
		install: newSkillsInstallCommand(skillsCmd),
	}
}

// skillsListCommand implements `tsh skills ls`.
type skillsListCommand struct {
	*kingpin.CmdClause
	format string
}

func newSkillsListCommand(parent *kingpin.CmdClause) *skillsListCommand {
	cmd := &skillsListCommand{
		CmdClause: parent.Command("ls", "List available Teleport agent skills.").Default(),
	}
	cmd.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).
		Short('f').
		Default(teleport.Text).
		EnumVar(&cmd.format, defaults.DefaultFormats...)
	return cmd
}

func (c *skillsListCommand) run(cf *CLIConf) error {
	catalog, err := loadSkills()
	if err != nil {
		return trace.Wrap(err)
	}

	format := strings.ToLower(c.format)
	switch format {
	case teleport.Text, "":
		return printSkillsAsText(cf, catalog)
	case teleport.JSON, teleport.YAML:
		out, err := serializeSkills(catalog, format)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprintln(cf.Stdout(), out)
		return nil
	default:
		return trace.BadParameter("unsupported format %q", format)
	}
}

func printSkillsAsText(cf *CLIConf, catalog []skill) error {
	var rows [][]string
	for _, s := range catalog {
		rows = append(rows, []string{s.Name, s.ShortDescription, s.installCommand()})
	}
	t := asciitable.MakeTable([]string{"Name", "Description", "Install"}, rows...)
	fmt.Fprintln(cf.Stdout(), t.AsBuffer().String())
	fmt.Fprint(cf.Stdout(), skillsListHint)
	return nil
}

const skillsListHint = "" +
	"hint: install a skill into your AI agent with 'tsh skills install <name>'\n" +
	"      run 'tsh skills ls --format json' for machine-readable output\n\n"

func serializeSkills(catalog []skill, format string) (string, error) {
	if catalog == nil {
		catalog = []skill{}
	}
	var out []byte
	var err error
	if format == teleport.JSON {
		out, err = utils.FastMarshalIndent(catalog, "", "  ")
	} else {
		out, err = yaml.Marshal(catalog)
	}
	return string(out), trace.Wrap(err)
}

// skillsInstallCommand implements `tsh skills install <name>`.
type skillsInstallCommand struct {
	*kingpin.CmdClause
	name  string
	dir   string
	force bool
}

func newSkillsInstallCommand(parent *kingpin.CmdClause) *skillsInstallCommand {
	cmd := &skillsInstallCommand{
		CmdClause: parent.Command("install", "Install an embedded Teleport agent skill into your AI agent."),
	}
	cmd.Arg("name", "Name of the skill to install (see 'tsh skills ls').").Required().StringVar(&cmd.name)
	cmd.Flag("dir", fmt.Sprintf("Skills directory to install into (default %q).", defaultSkillsDir)).
		StringVar(&cmd.dir)
	cmd.Flag("force", "Overwrite the skill if it is already installed.").BoolVar(&cmd.force)
	return cmd
}

func (c *skillsInstallCommand) run(cf *CLIConf) error {
	catalog, err := loadSkills()
	if err != nil {
		return trace.Wrap(err)
	}
	if !slices.ContainsFunc(catalog, func(s skill) bool { return s.Name == c.name }) {
		var names []string
		for _, s := range catalog {
			names = append(names, s.Name)
		}
		return trace.NotFound("unknown skill %q; available skills: %s",
			c.name, strings.Join(names, ", "))
	}

	baseDir, err := resolveSkillsDir(c.dir)
	if err != nil {
		return trace.Wrap(err)
	}
	dest := filepath.Join(baseDir, c.name)

	if _, err := os.Stat(dest); err == nil {
		if !c.force {
			return trace.AlreadyExists("skill %q is already installed at %s; use --force to overwrite",
				c.name, dest)
		}
		if err := os.RemoveAll(dest); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := extractSkill(c.name, baseDir); err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(cf.Stdout(), "Installed skill %q to %s\n", c.name, dest)
	return nil
}

// resolveSkillsDir determines the target skills directory, expanding a leading
// "~" and falling back to the per-user default.
func resolveSkillsDir(dir string) (string, error) {
	if dir == "" {
		dir = defaultSkillsDir
	}
	if after, ok := strings.CutPrefix(dir, "~"); ok {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", trace.Wrap(err)
		}
		dir = filepath.Join(home, after)
	}
	return dir, nil
}

// extractSkill copies the embedded files for the named skill into
// baseDir/<name>, recreating the directory structure.
func extractSkill(name, baseDir string) error {
	fsys := skills.FS()
	return fs.WalkDir(fsys, name, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return trace.Wrap(err)
		}
		target := filepath.Join(baseDir, filepath.FromSlash(p))
		if d.IsDir() {
			return trace.Wrap(os.MkdirAll(target, 0o755))
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(os.WriteFile(target, data, 0o644))
	})
}
