package kingpin

import (
	"fmt"
	"strings"
)

type cmdGroup struct {
	app          *Application
	parent       *CmdClause
	commands     map[string]*CmdClause
	commandOrder []*CmdClause
}

func (c *cmdGroup) defaultSubcommand() *CmdClause {
	for _, cmd := range c.commandOrder {
		if cmd.isDefault {
			return cmd
		}
	}
	return nil
}

// GetArg gets a command definition.
//
// This allows existing commands to be modified after definition but before parsing. Useful for
// modular applications.
func (c *cmdGroup) GetCommand(name string) *CmdClause {
	return c.commands[name]
}

func newCmdGroup(app *Application) *cmdGroup {
	return &cmdGroup{
		app:      app,
		commands: make(map[string]*CmdClause),
	}
}

func (c *cmdGroup) flattenedCommands() (out []*CmdClause) {
	for _, cmd := range c.commandOrder {
		if len(cmd.commands) == 0 {
			out = append(out, cmd)
		}
		out = append(out, cmd.flattenedCommands()...)
	}
	return
}

func (c *cmdGroup) addCommand(name, help string) *CmdClause {
	cmd := newCommand(c.app, name, help)
	c.commands[name] = cmd
	// replace the existing command (if exists)
	for i := range c.commandOrder {
		if c.commandOrder[i].Name() == name {
			c.commandOrder[i] = cmd
			return cmd
		}
	}
	c.commandOrder = append(c.commandOrder, cmd)
	return cmd
}

func (c *cmdGroup) init() error {
	seen := map[string]bool{}
	if c.defaultSubcommand() != nil && !c.have() {
		return fmt.Errorf("default subcommand %q provided but no subcommands defined", c.defaultSubcommand().name)
	}
	defaults := []string{}
	for _, cmd := range c.commandOrder {
		if cmd.isDefault {
			defaults = append(defaults, cmd.name)
		}
		if seen[cmd.name] {
			return fmt.Errorf("duplicate command %q", cmd.name)
		}
		seen[cmd.name] = true
		for _, alias := range cmd.aliases {
			if seen[alias] {
				return fmt.Errorf("alias duplicates existing command %q", alias)
			}
			c.commands[alias] = cmd
		}
		if err := cmd.init(); err != nil {
			return err
		}
	}
	if len(defaults) > 1 {
		return fmt.Errorf("more than one default subcommand exists: %s", strings.Join(defaults, ", "))
	}
	return nil
}

func (c *cmdGroup) have() bool {
	return len(c.commands) > 0
}

type CmdClauseValidator func(*CmdClause) error

// A CmdClause is a single top-level command. It encapsulates a set of flags
// and either subcommands or positional arguments.
type CmdClause struct {
	actionMixin
	*flagGroup
	*argGroup
	*cmdGroup
	app       *Application
	name      string
	aliases   []string
	help      string
	isDefault bool
	validator CmdClauseValidator
	hidden    bool
}

func newCommand(app *Application, name, help string) *CmdClause {
	c := &CmdClause{
		flagGroup: newFlagGroup(),
		argGroup:  newArgGroup(),
		cmdGroup:  newCmdGroup(app),
		app:       app,
		name:      name,
		help:      help,
	}
	return c
}

// Name returns the name of the clause
func (c *CmdClause) Name() string {
	return c.name
}

// Add an Alias for this command.
func (c *CmdClause) Alias(name string) *CmdClause {
	c.aliases = append(c.aliases, name)
	return c
}

// Validate sets a validation function to run when parsing.
func (c *CmdClause) Validate(validator CmdClauseValidator) *CmdClause {
	c.validator = validator
	return c
}

func (c *CmdClause) FullCommand() string {
	out := []string{c.name}
	for p := c.parent; p != nil; p = p.parent {
		out = append([]string{p.name}, out...)
	}
	return strings.Join(out, " ")
}

// Command adds a new sub-command.
func (c *CmdClause) Command(name, help string) *CmdClause {
	cmd := c.addCommand(name, help)
	cmd.parent = c
	return cmd
}

// Default makes this command the default if commands don't match.
func (c *CmdClause) Default() *CmdClause {
	c.isDefault = true
	return c
}

func (c *CmdClause) Action(action Action) *CmdClause {
	c.addAction(action)
	return c
}

func (c *CmdClause) PreAction(action Action) *CmdClause {
	c.addPreAction(action)
	return c
}

func (c *CmdClause) init() error {
	if err := c.flagGroup.init(c.app.defaultEnvarPrefix()); err != nil {
		return err
	}
	if c.argGroup.have() && c.cmdGroup.have() {
		return fmt.Errorf("can't mix Arg()s with Command()s")
	}
	if err := c.argGroup.init(); err != nil {
		return err
	}
	if err := c.cmdGroup.init(); err != nil {
		return err
	}
	return nil
}

func (c *CmdClause) Hidden() *CmdClause {
	c.hidden = true
	return c
}
