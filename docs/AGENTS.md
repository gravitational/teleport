# AGENTS.md

Guidance for AI agents working with documentation in this repo. Docs live in
`docs/pages/` and are written in MDX. These guidelines apply to work under
`docs/`; for code changes, see the repo-root `AGENTS.md`.

This file covers two distinct jobs. Determine which one applies to your
current task before reading further, and only follow the rules for that
mode — the two are not interchangeable and some rules conflict by design.

- **Review Mode:** You are reading a documentation PR as a critic. You do
  not run commands against live infrastructure and you do not edit files;
  you produce findings. Covered by "Review guidelines" through "Out of
  scope" below.
- **Execution Mode:** You are following a guide's numbered steps against
  real infrastructure, acting as a reader would. You run commands, resolve
  `<Var>` placeholders, and check prerequisites. Covered by "Resolving
  variables and placeholders in guides" and "Prerequisites and permission
  failures" below.

A third section, "Authoring guidance," is written for the human (or agent)
drafting a guide, not for a reviewing or executing agent. Review Mode checks
against it; see "What to check" below.

If it's unclear which mode applies, stop and ask rather than guessing —
the two modes call for opposite behavior around running commands.

## Authoring guidance

This section is for whoever is writing or editing a guide. Its purpose is
narrow: guides get followed by AI agents in Execution Mode (an internal
Beams test run, or a customer's own agent working through the guide against
their cluster), and two specific failure patterns show up repeatedly when
guides don't follow it.

**Failure pattern 1: prerequisites get skipped even when they're present.**
A Prerequisites section written as narrative description ("You'll need
appropriate permissions to configure this resource") reads fine to a human
but doesn't give an executing agent anything to check against — it can
convince itself the description is satisfied without verifying it. To
prevent this:

- Name the exact role or permission required, not a paraphrase of it (say
  `editor`, not "appropriate permissions").
- Where a step depends on a specific role, add a `requires` attribute to
  the relevant `<Var>` or step (e.g. `requires="role:editor"`) instead of
  relying on prose alone. Execution Mode is instructed to check `requires`
  attributes against the agent's own roles before proceeding; prose alone
  has no equivalent enforcement.
- If a prerequisite depends on infrastructure the agent can't assume exists
  (a target host, a network path, an external system), say so explicitly
  rather than implying the reader's own workstation satisfies it.

**Failure pattern 2: `<Var>` placeholders get filled with the example value
instead of a real one.** If a `<Var>` shows a plausible-looking default
(e.g. `teleport.example.com`), an executing agent may treat it as usable
rather than a placeholder to replace. To prevent this:

- Add `source="user-supplied"` to any `<Var>` whose value cannot be
  derived from a prior command, a discovery command, or an environment
  variable — this is what tells an executing agent to stop and ask instead
  of guessing or reusing the example. See "Resolving variables and
  placeholders in guides" below for the full rule set this maps to.
- Avoid giving a user-supplied `<Var>` a `default` that looks like a
  complete, valid value; if an example is necessary, make it obviously a
  placeholder (e.g. `<my-cluster-name>`) rather than something that could
  pass for real.
- If a value can be derived (proxy address, cluster name, existing role),
  mark it with `source="command:<...>"` rather than leaving it ambiguous —
  don't make every `<Var>` "user-supplied" by default, since that pushes
  unnecessary questions onto a customer running the guide.

A guide that follows both of these should need no other agent-specific
authoring effort — the burden shifts from every guide re-explaining itself
to the shared `<Var>` and Prerequisites conventions doing the work.

## Review Mode

### Review guidelines

- Focus on accuracy, structure, and conventions. Flag issues; do not rewrite.
- Keep suggestions targeted. Quote the specific line and propose a minimal fix.
- Flag only issues that affect correctness, conventions, or reader success.
  Do not report stylistic preferences beyond the rules listed here and in
  `AGENTS-STYLE.md`.
- **Diff context limitations:** Only flag missing structural elements (like
  Prerequisites or "How it works" sections) if the PR is adding a completely
  new page or heavily rewriting the introduction. Do not flag missing elements
  if they might exist outside the provided diff context.
- When uncertain, prefer no finding over a speculative finding.
- Do not infer repo conventions that are not documented in this file or
  `AGENTS-STYLE.md`.
- Docs changes on `master` are backported to release branches. Raise all
  issues on the original PR; do not defer findings to the backport phase. A
  finding that only surfaces during backport is a review miss.

### Review output

- Group findings by file, in the order files appear in the diff.
- Label each finding with a severity:
  - **Blocker**: factual error, broken link, command that will not run as
    written, or missing required frontmatter.
  - **Suggestion**: convention violation, structural issue, or missing
    recommended pattern with a clear fix.
  - **Nit**: minor wording or formatting; authors may ignore these.
- For each finding, quote the line, state the issue in one sentence, and
  propose the minimal fix. Use a GitHub suggestion block when the fix is a
  one-line change.
- If the diff is clean, say so briefly. Do not invent findings.

Example of a well-formed finding:

> **Suggestion** (`docs/pages/admin-guides/example.mdx`, line 42):
> `## Configure The Agent` - headings use sentence case.
> Proposed fix:
> ```suggestion
> ## Configure the agent
> ```

### What to check

- **Style and conventions**
  - The conventions themselves (voice, headings, naming, components, page-type
    shape, and so on) live in
    [`contributing/documentation-style-guide.md`](contributing/documentation-style-guide.md),
    the single source of truth shared with human contributors. Do not restate
    or re-derive those rules — consult the guide.
  - For how to *apply* them in review — what an agent can check reliably, what
    the linters already cover, and how to rate severity — see
    [`AGENTS-STYLE.md`](./AGENTS-STYLE.md).
  - Beyond the guide, flag internal inconsistency within a page: the same term
    capitalized or formatted two ways, or an acronym/concept keyword that
    switches forms (e.g. "two-factor" in one place, "2FA" in another).
  - Code fences declare a language and follow repo conventions (e.g., `code`
    for commands, `yaml` for config).
  - Admonitions (`<Admonition>`) are used for warnings and notes, not for
    content that belongs in body text.
- **Structural patterns**
  - How-to guides include a `How it works` section after the introduction.
  - A Prerequisites block appears before the first step.
  - Steps use `Step N/M.` numbered headings. *Note: Verify that step numbers
    are sequential within the diff. Flag obvious numbering errors, but defer to
    human review for the total step count (the `M` value) if the whole file is
    not visible.*
  - Checkpoint blocks appear at genuine verification gates. A Checkpoint
    belongs after a step whose silent failure would break later steps. Do not
    require checkpoints after every step; use them only where verification
    materially reduces troubleshooting effort. Titles state a positive success
    condition; bodies contain only troubleshooting guidance.
- **Version scoping**
  - Commands, config fields, and flags exist in the Teleport version the page
    targets. The target version is the release line of the branch the PR is
    based on (`master` targets the next release; `branch/v*` targets that
    major version). *(Note: The target branch will be provided in your system
    prompt context.)*
  - To verify a command or flag exists, check the CLI reference pages in this
    repo: `docs/pages/reference/cli/tctl.mdx`,
    `docs/pages/reference/cli/tsh.mdx`, and
    `docs/pages/reference/cli/tbot.mdx` (published under
    `https://goteleport.com/docs/reference/cli/`). If you cannot verify, flag
    uncertainty for a human reviewer; do not assert that a flag exists or does
    not exist from memory.
  - Version-specific behavior is called out explicitly.
- **LLM-readability**
  - Commands are copy-pastable as written.
  - User-supplied values are marked with `<Var>` components (e.g.,
    `<Var name="username" />`). Reused values should generally use the same
    `<Var>` component consistently throughout the page.
  - Partials/includes are used where shared content exists. Partials live in
    `docs/pages/includes/`; before flagging duplication, confirm a matching
    partial actually exists there.
  - Prerequisites name the exact role or permission required (not a
    paraphrase like "appropriate permissions"), per "Authoring guidance"
    above. Flag vague prerequisite language as a Suggestion.
  - A `<Var>` whose value can't reasonably be derived (a new resource name,
    an external credential, anything only the reader would know) carries
    `source="user-supplied"`. Flag a missing `source` attribute on such a
    `<Var>` as a Suggestion — without it, an executing agent may reuse the
    shown example value instead of asking. Don't flag `<Var>` tags for
    values that are plausibly derivable (proxy address, cluster name);
    only flag the ones that clearly cannot be.
- **Cross-references**
  - Links to related docs are present and resolve (correct relative paths, no
    broken anchors).
  - Links to pages within this repo use relative `.mdx` paths rather than
    published `https://goteleport.com/docs/...` URLs.
  - `Next steps` sections link to canonical follow-on guides.
- **Frontmatter completeness**
  - `title` and `description` are present and accurate. The `description` is
    one sentence that starts with an active verb (e.g., "Explains how to...",
    "Configures...") and ends with a period.
  - For newly added standalone pages (files outside `includes/`), verify that
    `tags` and `sidebar_label` are present.
  - Canonical tag definitions live in the docs-website repo (`tags.yml`). Do
    not invent or "correct" tag names; flag unfamiliar tags for human
    verification. See the "Frontmatter and tags" section of `AGENTS-STYLE.md`.

### References

- Documentation conventions (source of truth):
  [`contributing/documentation-style-guide.md`](contributing/documentation-style-guide.md),
  shared with human contributors.
- Applying those conventions in review: [`AGENTS-STYLE.md`](./AGENTS-STYLE.md)
  (next to this file) — what's checkable, what the linters cover, and severity
  for style findings.
- CLI references: `docs/pages/reference/cli/{tctl,tsh,tbot}.mdx`.
- Automated linting: docs content is checked by remark-lint in the
  docs-website repo (`.remarkrc.mjs`, run via `yarn markdown-lint` in CI). Do
  not spend review effort on mechanical issues these rules catch automatically
  (e.g. formatting, line length); focus on accuracy, structure, and
  conventions the linter cannot verify. Do not attempt to build or lint docs
  from this repo.

### Out of scope

Do not do the following in a docs review. (This list applies to Review Mode
only — Execution Mode's whole job is running commands against a live
cluster; see below.)

- Wholesale rewrites on style or tone grounds.
- Changes to product behavior described in the docs. If the docs appear to
  contradict product behavior, flag it for a human reviewer.
- Auto-resolving MDX includes (`(!...!)` partials) into inline content.
- Speculation on provider-specific facts (AWS, Azure, GCP, IdP vendors)
  without a source. Flag uncertainty for a human reviewer instead.
- Verifying commands against live clusters. A reviewer checks a command
  for correctness by reading it, not by running it.

## Execution Mode

You are following a guide's steps for real, against live infrastructure, to
confirm it works as written. This means running commands, resolving
placeholders, and checking your own permissions before acting — the opposite
posture from Review Mode above.

### Resolving variables and placeholders in guides

Guides contain `<Var name="..." />` placeholders standing in for values such
as addresses, tokens, and resource names. Never invent or guess a value for
a placeholder. Resolve each one using the first rule that applies:

1. **Output of a prior step.** If a command earlier in the guide (or one
   you already ran in this session) produced the value (a join token, a
   resource ID, an address printed in command output), reuse that exact
   value. Example: `<Var name="token" />` after a `tctl tokens add` step
   refers to the token that command printed.
2. **Discoverable by command.** If the value describes the current cluster
   or session: proxy address, cluster name, your roles, existing
   resources - obtain it with a read-only command and parse the output.
   Useful commands: `tsh status --format=json`, `tctl status`,
   `tctl get <resource>`. Example: `<Var name="proxy-address" />` comes
   from `tsh status`.
3. **Environment.** If a conventional environment variable clearly
   provides the value (for example `TELEPORT_PROXY`), use it.
4. **Otherwise, ask the user and stop until they answer.** Values only the
   user can choose or know, such as names for new resources, endpoints and
   credentials of external systems, cloud account identifiers cannot be
   derived. Ask before proceeding. A wrong guess wastes the entire run;
   asking costs one exchange.

The same placeholder name refers to the same value everywhere on a page:
once resolved, reuse it; do not re-derive or re-prompt.

If you resolved a value by inference (rules 1–3) and later output
contradicts it (errors referencing a different address, a resource not
found) stop and re-confirm the value with the user instead of continuing.

### Values that aren't wrapped in `<Var>`

Many guides have not yet been updated to mark every substitutable value
with `<Var>` (see "Authoring guidance"). Absence of a `<Var>` tag is not a
signal that a literal value is safe to copy as-is. Before reusing any
literal value from a guide's text or code fences, ask: **would this value
be the same for every reader, or does it depend on the reader's own
account, cluster, or environment?** If it depends on the reader, treat it
as if it carried `source="user-supplied"` and apply rules 1–4 above,
even though no `<Var>` tag is present.

Treat the following as reliable signals that an unwrapped literal is a
stand-in, not a real value:

- Known placeholder domains and hosts: `example.com`, `example.org`,
  `*.example.teleport.sh`, or any host containing the literal word
  `example`.
- Placeholder-shaped identifiers: repeated or sequential digits in an
  account ID or ARN (`111111111111`, `123456789012`), UUIDs that appear
  earlier in the same guide as a different example, or resource names
  that don't match anything you've actually created or queried.
- Generic human names or roles used as usernames: `joe`, `myuser`,
  `contractor`, `foo@example.com`, and similar.
- Names prefixed `my-`/`your-` (`my-kubernetes-cluster`,
  `your-github-username`), or file paths written as a description rather
  than a real path (`/path/to/token.file`).
- Values introduced in prose as "replace X with your own Y" — this is the
  same instruction a `source="user-supplied"` `<Var>` would carry, just
  stated in text instead of markup.

When genuinely unsure whether a literal is a placeholder or a fixed value, 
ask rather than guess; the cost asymmetry is the same as for `<Var>` resolution.

### Explicit sourcing hints override the rules above

Some `<Var>` tags may carry a `source` attribute (in rendered pages,
`data-agent-source`). When present, it overrides rules 1–4:

- `source="user-supplied"` - ask the user; the value cannot be derived.
- `source="command:<cli command>"` - run that command and parse its output.
- `source="env:<VAR_NAME>"` - read that environment variable; ask if unset.
- `source="computed"` - reuse a value resolved earlier on the same page.

Most guides do not carry these hints; the numbered rules are the default.

### Prerequisites and permission failures

Before executing a guide's numbered steps, verify its Prerequisites
section. Confirm you are logged in (`tsh status`) and that each listed
requirement is met; do not assume a prerequisite is satisfied because it
is probably true.

Not every prerequisite is a login or a role. Some require
infrastructure, such as a host to enroll, a network path, or an external
system. Do not assume such a resource exists, and do not substitute the
machine you are running on for it. Many guides act on a **separate target
host**, distinct from the workstation where you run `tsh` and `tctl`; that
host's requirements (operating system, open ports, network access) may
differ from your own environment. For example, a guide that enrolls a
Linux server still expects that server to exist even though you run the
client commands from macOS or Windows. Before starting the steps, identify
which machine or resource each step acts on, confirm it exists and meets
the stated requirements, and stop to ask the user if it is missing or the
guide is ambiguous about which machine is meant. A wrong assumption here
wastes the entire run; asking costs one exchange.

If any command fails with an authorization error, such as "access denied",
"permission denied", a missing verb on a resource, or any RBAC error,
treat it as a privilege gap, not a configuration problem, a product bug,
or a documentation error. Do not retry with invented workarounds, do not
skip the step, and never report the step complete. Instead:

1. Identify the role or permission the step requires. Guides usually name
   required roles (such as `editor`) in their Prerequisites section.
2. File an Access Request: `tsh request create --roles=<role>`.
3. Tell the user what you requested and why, and pause the run until the
   request is approved or denied.

If a prerequisite or `<Var>` carries an explicit `requires` attribute
(for example `requires="role:editor"`), check your roles against it with
`tsh status` before attempting the step, and follow the same
Access Request procedure if you lack the role.
