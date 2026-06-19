# Teleport docs style reference

Condensed style rules for AI agents reviewing docs PRs. This file covers only
checkable conventions. Many style questions have equally justifiable
alternatives; where a rule is not listed here, consistency within the page
wins (do not flag it).

## Page types and their required shape

- **How-to guides** are task oriented. They include: introductory paragraphs
  stating the task, the scenario, and the expected outcome; a Prerequisites
  block before the first step; `Step N/M.` numbered step headings; and a
  `Next steps` section. Avoid links that break the reader's focus
  mid-procedure, and avoid admonitions or collapsed details unless necessary.
  External links must state why to follow them and what to take away
  ("Follow the installation instructions in the AWS documentation", not
  "Read the AWS documentation for more information").
- **Tutorials** are learning oriented for first-time users. They include: a
  "Before you begin" tool list, learning objectives, sequenced steps, and a
  guaranteed successful outcome with no unexplained errors.
- **Conceptual guides** explain what something is, why it matters, and how it
  works. Diagrams and links to related topics are encouraged when useful.
- **Reference manuals** are comprehensive (list all options, not a sample),
  prefer tables over prose, keep all content on one searchable page, and
  favor brevity.

## Voice and body text

- Technical precision over broad benefit statements. Good: "Teleport replaces
  shared secrets with short-lived X.509 and SSH certificates." Bad: "Teleport
  replaces insecure secrets with true identity."
- Prefer sparse one-to-two-sentence paragraph groupings over long blocks.
- List items end with a period, unless the item ends in a command.
- Prefer short paragraphs over lists; when listing, prefer bullets over
  numbers; use numbered lists only for step sequences.

## Code, commands, and values

- Inline mentions of `tsh`, `tctl`, `tbot`, and other commands go in
  backticks, as do ports and literal values (e.g. `443`).
- Prefer full-line code snippets for commands so the copy button renders.
- Commands and configuration examples should be copy-pastable with minimal
  modification.

## Headings, titles, and sidebar labels

- Section headings use sentence case ("Next steps", not "Next Steps");
  proper nouns and product names keep their capitals.
- Page titles use Teleport title-case conventions and should not exceed
  55 characters (the site appends "| Teleport Docs" toward a 70-character
  SEO ceiling).
- Sibling pages should not repeat a common prefix in sidebar labels
  ("AWS", "Azure" nested under a "Deployments" category, not "Deploy on
  AWS", "Deploy on Azure").

## Product and concept names

- Product proper nouns are capitalized ("Trusted Cluster", not "trusted
  cluster"), generally bolded on their first meaningful mention within a
  page, and never wrapped in quotes.
- Acronyms are introduced after the full concept keyword and used
  consistently thereafter. Use one variant per page (e.g. pick one of
  "two-factor" or "2FA"; do not mix).

## Frontmatter and tags

- Required on every standalone page (not `includes/` partials): `title` and
  `description`.
- `description` is one sentence that starts with a verb, summarizes the
  page, ends with a period, and uses common keywords for the subject.
- Common optional fields: `sidebar_label`, `sidebar_position`, `tags`
  (a YAML list), keywords, a video banner link, or an alternate first-level
  heading.
- Some repositories or page types may require additional frontmatter fields;
  follow repository-specific review guidance.
- The canonical definitions of valid frontmatter fields and tags live in the
  separate docs-website repo (`frontmatter_fields.yaml` and `tags.yml`), not
  in this repo. Reviewers must not invent tag names or "correct" a tag to a
  guessed value. If a tag looks unfamiliar, flag it for human verification
  against docs-website `tags.yml`.

## Additional conventions

- Footnotes appear in the order they are referenced.
- Prefer paragraphs, headings, and code snippets over additional UI
  components. Use components such as Tabs or Details only when they
  materially improve readability or reduce duplication.
