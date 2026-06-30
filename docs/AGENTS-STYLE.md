# Reviewing docs for style (agent guide)

This file tells an AI agent **how to apply** Teleport's documentation
conventions when reviewing a docs PR. It deliberately does not restate the
conventions themselves: those live in
[`../contributing/documentation-style-guide.md`](../contributing/documentation-style-guide.md),
the single source of truth shared by human contributors and agents. When a
question isn't answered there, consistency within the page wins — do not flag
it.

For the review process, output format, and severity labels, see
[`AGENTS.md`](./AGENTS.md). This file covers the style/prose dimension
specifically.

## Before flagging a style issue

- **Check it against the style guide; don't assert from memory.** If a rule
  isn't in `documentation-style-guide.md`, it isn't a rule. Do not invent
  conventions or carry them in from other projects.
- **Don't duplicate the linters.** remark-lint (formatting, spacing, line
  length) and Vale (word choice, prose style, spelling, banned terms) already
  run in CI. Never raise a finding that one of those tools enforces — it's
  noise. Spend review effort on what a linter can't judge: structure, voice,
  internal consistency, and correctness.
- **Prefer no finding on judgment calls.** Voice and component choice need
  judgment (see below). Raise these only when the violation is clear, and as a
  Suggestion or Nit — never a Blocker.

## What an agent can check reliably vs. what needs a human

Check with confidence (flag when violated):

- Heading case (sentence case).
- Internal consistency within a page: a term capitalized or formatted two
  ways, or an acronym/keyword that switches forms ("two-factor" vs. "2FA").
- Product proper nouns wrapped in quotes, or not capitalized.
- Page-title length over the 55-character budget (before the
  "| Teleport Docs" suffix).
- List-item punctuation (period unless the item ends in a command).

Needs judgment (raise tentatively, or defer to a human):

- Whether the voice is appropriately technical for the page's audience.
- Whether a component (`Tabs`, `Details`, `Admonition`) earns its place or
  should be prose or subheadings.
- Whether the page is the right *type* (how-to vs. tutorial vs. reference) for
  its content.

## Severity defaults for style findings

Apply the severity labels from `AGENTS.md` as follows:

- **Blocker:** essentially never for pure style. Style issues don't block a
  merge.
- **Suggestion:** a clear convention violation with an unambiguous fix — a
  Title-Case heading, an inconsistent term, a command missing backticks.
- **Nit:** minor polish the author may ignore — a product noun not bolded on
  first use, a title near the length ceiling.

## Page-type shape

The required shape of each page type (how-to, tutorial, conceptual, reference)
is defined in `documentation-style-guide.md`. Use it as a checklist, but only
flag a *missing* structural element when the PR adds a new page or heavily
rewrites the introduction, per the diff-context rule in `AGENTS.md`. Don't
assume an element is absent just because it falls outside the diff.

## Frontmatter and tags (verification discipline)

- Every standalone page (not an `includes/` partial) needs `title` and
  `description`. The `description` is one sentence that starts with a verb and
  ends with a period.
- **Do not invent or "correct" tag names.** The canonical lists of valid
  frontmatter fields and tags live in the docs-website repo
  (`frontmatter_fields.yaml`, `tags.yml`), not here. If a tag looks
  unfamiliar, flag it for human verification against `tags.yml` rather than
  guessing a "right" value.
