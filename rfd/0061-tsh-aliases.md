---
authors: Roman Tkachenko (roman@goteleport.com)
state: implemented
---

# RFD 61 - tsh command aliases

## What

Proposes a way for users to define custom tsh commands.

## Why

Defining custom commands will allow users to extend `tsh` CLI to fit their
use-case without having to modify Teleport's source code.

Examples of what users might want to do include:

- Define shorthands for the existing `tsh` commands.

- Tweak the default behavior of existing `tsh` commands, for example to have
  `tsh login` always default to a particular auth connector or username.

- Define new subcommands combining multiple actions in the same command, for
  example logging into a cluster, a database and connecting to the database.

## Prior art

### bash aliases

The simplest analogue that comes to mind is the bash alias:

```bash
alias ll="ls -l"
```

Bash aliases are quite simplistic as they only allow to define new commands
that expand to the strings they alias and don't provide a way to refer to the
command's arguments.

### git aliases

Git aliases are defined in the repo-specific `.gitconfig` (or `.git/config`)
or global `/etc/gitconfig` file, and are much closer conceptually to what we
want to accomplish as they allow to define custom `git` subcommands:

```ini
[alias]
co = checkout
ls = log --oneline
```

Git aliases also support calling non-`git` commands (using `!` notation) and
refer to the command's arguments (via `$@`, `$1`, etc.) which allows to build
powerful new commands.

For example, the following alias will make `git top` to show 15 most recently
updated local branches:

```ini
[alias]
top = "!bash -c 'git branch --sort=-committerdate | head -15'"
```

## Configuration and examples

Users will define aliases in their `tsh` configuration file which is kept
in `$TELEPORT_HOME/config/config.yaml` using the following syntax:

```yaml
aliases:
    "<alias>": "<command>"
```

The `<alias>` can only be a top-level subcommand. In other words, users can
define a new `tsh mycommand` alias but cannot define `tsh my command` command.

A few notes regarding the `<command>`:

- Unlike `git` aliases explored above, proposal is to always require the binary
  name instead of defaulting to `tsh` and requiring special syntax otherwise.
- The `<command>` will be fed to `exec.Command` for execution. To chain multiple
  commands, use pipes, etc. users will explicitly use `bash -c`.
- The `<command>` will require `$@`, `$1`, etc. to reference the arguments
  provided to the alias command.

### Examples

Starting with a very simple example, suppose you're tired of typing `tsh login`
every morning and want to alias it to something shorter e.g. `tsh l`:

```yaml
aliases:
    "l": "tsh login $@"
```

As a more practical example, the following alias will make `tsh login` default
to the specific auth connector and username:

```yaml
aliases:
    "login": "tsh login --auth=local --user=alice $@"
```

Similarly, as discussed in the search-based access requests RFD, some users
may want `tsh ssh` to default to the mode in which it will auto-request access
to the node upon encountering an access denied error:

```yaml
aliases:
    "ssh": "tsh ssh -P $@"
```

Note: command arguments will be resolved prior to invoking the alias. So in the
example above, when a user runs `tsh ssh root@node1`, the alias command executed
will be `tsh ssh -P root@node1`.

### Environment variables

Each alias invocation will set a few environment variables that can be consumed
within the alias.

To prevent infinite recursion, `TSH_ALIAS` variable will be set indicating the
command is invoked as an alias. If it's detected, `tsh` won't attempt to expand
the same command again. For example, `TSH_ALIAS=login`. This will prevent other
`login` commands from being expanded but will allow using aliases in other
aliases (see below for an example).

Each alias will also set `TSH` environment variable to the path of the `tsh`
binary that invoked the alias and `SHELL` environment variable to the default
shell of the user invoking the command.

### More examples

An alias can also define a custom subcommand that combines multiple commands.
The following alias will connect to a node within a specific leaf cluster
using `tsh connect leaf ubuntu@node-1` command:

```yaml
aliases:
    "connect": "bash -c '$TSH login $1 && $TSH ssh $2'"
```

The following alias will list nodes in all clusters:

```yaml
aliases:
    "lsall": "bash -c 'for cluster in $($TSH clusters | tail -3 | head -2 | cut -d \' \' -f1); $TSH ls --cluster=$cluster; done'"
```

Multiple aliases can be defined in the config and can reference each other:

```yaml
aliases:
    "login": "$TSH login --auth=local --user=alice $@"
    "ssh": "$TSH ssh -P $@"
    "connect": "bash -c '$TSH login $1 && $TSH ssh $2'"
```

In this example, `tsh login` and `tsh ssh` will use the aliases when invoked
as a part of the "connect" command.

## Global tsh config

Currently tsh config is read from `$TELEPORT_HOME/config/config.yaml` which by
default is user-specific since `$TELEPORT_HOME` defaults to `~/.tsh`.

As a part of this feature, introduce global tsh config `/etc/tsh.yaml`. Settings
from the global config will merge with the user-local config, with the local
config taking precedence.

## Troubleshooting

For easier troubleshooting of aliases, make sure that tsh logs the actual
command being executed when invoking an alias.

## Future work

When we have `bash`/`zsh` auto-completion, it should take aliased commands into
account as well. For reference, `git` autocompletion supports it.
