# AGENTS.md

Guidance for AI agents reviewing documentation PRs in this repo. Docs live in
`docs/pages/` and are written in MDX. These guidelines apply to changes under
`docs/`; for code changes, see the repo-root `AGENTS.md`.

## Review guidelines

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

## Review output

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

## What to check

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

## References

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

## Out of scope

Do not do the following in a docs review:

- Wholesale rewrites on style or tone grounds.
- Changes to product behavior described in the docs. If the docs appear to
  contradict product behavior, flag it for a human reviewer.
- Auto-resolving MDX includes (`(!...!)` partials) into inline content.
- Speculation on provider-specific facts (AWS, Azure, GCP, IdP vendors)
  without a source. Flag uncertainty for a human reviewer instead.
- Verifying commands against live clusters.
