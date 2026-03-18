---
authors: Roman Tkachenko (roman@goteleport.com)
state: draft
---

# RFD 0037e - Skills

## Required Approvers

- Engineering: @greedy52
- Product: @klizhentas

## What

Defines how we keep custom [Agent Skills](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/overview)
we create for Teleport in our codebase and how Teleport users consume them.

## Why

As our focus is on making Teleport integrate better with AI/LLM ecosystem by
improving CLI/API, the team will be creating custom Skills that will allow
AI agents to interact with Teleport to simplify and improve user experience
of routine operations, for instance reviewing access requests, doing periodic
access list recertifications and so on.

With the Skills being developed by different teams and team members, we need
to agree on a common way to keep them in our codebase and letting Teleport
users discover them easily while also ensuring compatibility with existing tools
that can discover and install Skills.

## Details

### Placement

We will keep Teleport Skills in the Teleport GitHub repository under the top-level
skills/ directory:

```
➜  teleport git:(master) tree skills
skills
└── teleport-acl-review
    └── SKILL.md
```

There's a certain amount of prior art of existing projects that keep their Skills
under the top-level skills/ directory, for example:

- Anthropic's own Skills repository: https://github.com/anthropics/skills
- Vercel's agent Skills repository: https://github.com/vercel-labs/agent-skills
- Google Workspace CLI repository: https://github.com/googleworkspace/cli

Keeping the Skills in the same repo as Teleport allows us to version them in
tandem with Teleport, avoid drift and compatibility issues, and potentially
publish them as part of our regular publishing pipeline (similar to Teleport
access plugins residing in Teleport repository) should there be a central Skills
registy in future.

### Best practices

When writing a Skill, follow Agent Skills specification which is widely adopted
by agents: https://agentskills.io/specification.

At a minimum, a Skill directory should include a `SKILL.md` file that includes
metadata such as name, description and instructions.

### Documentation and testing

A Skill should have a corresponding guide in the relevant section of Teleport
public documentation explaining what it's for and showing usage examples. For
example, a Skill that helps perform access list reviews, has a corresponding
guide showing how to install and use it.

Since Skills committed to the repository become a part of our public API, ensure
their quality:

- Use evals to test your Skills: https://agentskills.io/skill-creation/evaluating-skills
- Include Skills test cases into relevant manual test plan sections
- Use pre-commit hook for Skill validation: https://github.com/agent-ecosystem/skill-validator

### Installation

Keeping skills in the repo's skills/ directory enables interoperability with
Vercel's "skills" command-line tool that is able to discover and install Skills
from GitHub repositories:

```
➜  npx skills add https://github.com/gravitational/teleport/tree/roman-skill-acl/skills/teleport-acl-review

███████╗██╗  ██╗██╗██╗     ██╗     ███████╗
██╔════╝██║ ██╔╝██║██║     ██║     ██╔════╝
███████╗█████╔╝ ██║██║     ██║     ███████╗
╚════██║██╔═██╗ ██║██║     ██║     ╚════██║
███████║██║  ██╗██║███████╗███████╗███████║
╚══════╝╚═╝  ╚═╝╚═╝╚══════╝╚══════╝╚══════╝

┌   skills
│
◇  Source: https://github.com/gravitational/teleport.git @ roman-skill-acl (skills/teleport-acl-review)
│
◇  Repository cloned
│
◇  Found 1 skill
│
●  Skill: teleport-acl-review
│
│  Review Teleport access lists that are due for audit. Use when the user asks to review access lists, audit Teleport ACLs, check which access lists need attention, perform periodic access list reviews, recertify access, or manage Teleport access list compliance. Trigger on phrases like "review access lists", "which access lists need review", "audit my ACLs", "recertify access lists", or any mention of Teleport access list reviews. Also trigger when the user follows up on access list findings from a previous command.
│
◇  42 agents
◇  Which agents do you want to install to?
│  Amp, Cline, Codex, Cursor, Gemini CLI, GitHub Copilot, Kimi Code CLI, OpenCode, Warp, Claude Code
│
◇  Installation scope
│  Global
│
◇  Installation method
│  Symlink (Recommended)

...

│
└  Done!  Review skills before use; they run with full agent permissions.
```

## Future work

### GitHub Actions validation

Set up GitHub Action that runs Skill validation:

https://github.com/marketplace/actions/validate-skill

### Teleport CLI for Skills

In future, we can consider embedding the Skills into Teleport so users can use
its CLI tooling to easily discover and install Skills into their agent without
having to use 3rd party dependencies like npx skills.

```
➜  tsh skills ls
| Name     | Description |
|----------|-------------|
| teleport-acl-review | Review Teleport access lists that are due for audit. Use when the user asks to review access list... |

➜  tsh skills install --skills=teleport-acl-review --agents=claudecode
Installed 1 skill to ~/.claude/skills
```
