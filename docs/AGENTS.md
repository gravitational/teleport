# AGENTS.md

This file provides guidance to Claude Code (claude.ai/code) when working with
code in this repository.

## Structure of the Teleport documentation site source

The [docs-website](https://github.com/gravitational/docs-website) repo contains
source files for the Teleport docs site. In the `content` directory is a git
submodule that corresponds to each supported version of the docs site, such as
`content/18.x` (for v18 and minor versions) and `content/17.x` (for v17 and
minor versions). The highest-numbered subdirectory of `content` is for the edge
version of the docs.

Each submodule of `content` is a clone of
https://github.com/gravitational/teleport. This AGENTS.md file is in the `docs`
directory of a single `gravitational/teleport` clone within a subdirectory of
`content`.

All build, lint, and preview commands are run from the `docs-website` root.

### Managing the Teleport docs site

For a complete reference of `yarn` commands for managing the Teleport docs site
in a local clone, read `../../../package.json`. There are `package.json` files
in subdirectories of each `content/v[0-9]+.x` submodule, but those are for the
Teleport product instead of the Teleport docs site.

> **Note:** `yarn dev` uses `watchexec` to watch for file changes in `content/`
> and sync them live. Includes files trigger a full Docusaurus restart.

### Structure of a single docs content version

```
docs/
  config.json       # Version-scoped variables (used via (=variable.name=) syntax) and redirects
  pages/            # MDX source pages
    includes/       # Reusable partial files — NOT rendered as pages
    <section>/      # Content organized by product area
  sidebar.json      # Sidebar navigation config
```

Pages are at `docs/pages/**/*.mdx`. The `includes/` directories are excluded
from page rendering and are only pulled in via the `(!path/to/file.mdx!)`
include syntax.

### MDX Components

Components you can use with the docs site are in `../../../src`. The components
listed in `../../../src/theme/MDXComponents/index.tsx` can be imported without
using an `import` statement. Other components require an `import` statement. 

To import a component from this version of the docs site, use the `@version`
alias as per the example below:

```jsx
import agenticAiImg from '@version/docs/img/agentic-ai/agentic-ai-hero.png';
```

### Remark plugins

**Remark plugins** (`server/`) transform content:

- `remark-includes` — resolves `(!file!)` includes
- `remark-variables` — resolves `(=var=)` variable syntax using `docs/config.json`
- `remark-update-asset-paths` — rewrites asset paths to point to `content/{version}/`
- `remark-no-h1`, `remark-code-snippet`, `remark-version-alias` — additional transforms

### Building the docs site

To test the rendered HTML of one version of the docs site (the content in this
repository):

1. Run the following command to move content files into the directory structure
   Docusaurus expects:

   ```bash
   # Navigate to the docs-website root
   cd ../../..
   yarn prepare-files
   ```

   `scripts/prepare-files.mts` copies `.mdx` files from
   `content/{version}/docs/pages/` to `docs/` (current version) or
   `versioned_docs/version-{name}/` (other versions), generates `sidebars.json`
   and `versions.json`.

1. Build the docs site:

   ```bash
   yarn docusaurus build
   ```

### Linting

When a documentation author opens a pull request with a documentation change in
`gravitational/teleport`, the following GitHub Actions workflow checks the
change: `.github/workflows/doc-tests.yaml`.

When making changes to the docs, run the following linters locally to anticipate
the checks we run on GitHub actions:

1. **remark linters** check the structure of the MDX in each docs page:

   ```bash
   yarn markdown-lint
   ```

1. **cspell** checks the spelling of each docs page. The file `docs/cspell.json`
   in this repository configures exceptions to cspell. You can edit this file if
   a cspell violation is not actually a mispelling. To run cspell, navigate to
   the root of the docs-website repo (`../../..`) and execute the following
   commandl where SUBMODULE is the path to the current submodule:

   ```
   yarn spellcheck SUBMODULE
   ```

1. **Vale** checks prose style. Run the Vale linter using this command from the
   root of this `gravitational/teleport` submodule:

   ```
   vale --config docs/.vale.ini docs/pages
   ```

## MDX content conventions

The docs site uses [Docusaurus](https://docusaurus.io/). The `yarn prepare-files` command, when run from the `docs-website` root, copies docs page files into the directory structure that Docusaurus expects.

### Frontmatter

Every page must begin with a YAML frontmatter block. Allowed fields are defined in `docs-website/frontmatter_fields.yaml`. Key fields:

- `title` (required) — page title
- `description` (optional: generated from the first paragraph if empty) — meta description
- `sidebar_label` — override the sidebar display name
- `sidebar_position` — order in sidebar (lower = higher)
- `tags` — list combining a type tag (`how-to`, `conceptual`, `get-started`, `reference`, `faq`, `other`) and product tags (`zero-trust`, `mwi`, `identity-governance`, `identity-security`, `platform-wide`). For a full list of tags, see the `tags.yml` field at the root of the `docs-website` repo.
- `template` — use `"no-toc"` for index pages without a table of contents
- `enterprise` — shows an enterprise badge (e.g., `Identity Governance`)

Other allowed Docusaurus-native fields include: `custom_edit_url`, `displayed_sidebar`, `draft`, `hide_table_of_contents`, `hide_title`, `id`, `image`, `keywords`, `last_update`, `pagination_label`, `pagination_next`, `pagination_prev`, `parse_number_prefixes`, `sidebar_class_name`, `sidebar_custom_props`, `sidebar_key`, `slug`, `toc_max_heading_level`, `toc_min_heading_level`, `unlisted`, `videoBanner`, `videoBannerDescription`.

### Page structure rules (enforced by linter)

- No H1 headings — the `title` frontmatter field renders as H1.
- Every page must have at least one paragraph before the first H2 heading.
- For how-to guides (pages with numbered steps), the first H2 must be `## How it works`.
- Pages with step-by-step instructions must use `## Step X/N. Task Name` in H2 headings, where X is the current step and N is the total.
- Links to docs must be relative paths to `.mdx` files — never absolute `/docs/...` or `https://goteleport.com/docs/...` URLs.

### Special syntax

- **Variable substitution:** `(=variable.path=)` — variables are defined in `docs/config.json` under `"variables"` and resolved at build time. Common variables: `(=teleport.version=)`, `(=cloud.version=)`, `(=clusterDefaults.clusterName=)`.
- **Includes:** `(!docs/pages/includes/filename.mdx!)` — inlines another MDX file. Path is relative to the version repo root. Parameters can be passed: `(!file.mdx param="value"!)`. Partials define defaults at the top using `{{ param="default" }}` syntax.
- **Lint suppression:** `{/* lint disable page-structure remark-lint */}` — suppresses structural lint warnings for a section.

### Var component

Use `<Var>` for user-replaceable values. Each `<Var>` name must appear at least 2 times on a page:

```mdx
Replace <Var name="cluster-name" /> with your Teleport cluster address.

```code
$ tsh login --proxy=<Var name="cluster-name" />:443
```
```

### Markdown formatting

- Use hyphens (`-`) for unordered lists with 1-space indentation for nested items.
- Use asterisks (`*`) for emphasis.
- Always use fenced code blocks with language identifiers. Use `code` for shell commands (supports `$` prefixes and `#` comments), `yaml`, `json`, `go`, `python`, `js`/`ts`, `diff` as appropriate.

### Components

**Tabs** — use for alternative approaches:

```mdx
<Tabs>
<TabItem label="Linux">

```code
$ curl -O https://example.com/linux.tar.gz
```

</TabItem>
<TabItem label="macOS">

```code
$ curl -O https://example.com/macos.pkg
```

</TabItem>
</Tabs>
```

**Admonition** — use for notes, tips, warnings, and danger notices:

```mdx
<Admonition type="note">Note content.</Admonition>
<Admonition type="tip">Helpful tip.</Admonition>
<Admonition type="warning">Potential issue warning.</Admonition>
<Admonition type="danger">Destructive action warning.</Admonition>
<Admonition type="note" title="Custom Title">Custom title admonition.</Admonition>
```

**Collapsible sections** — use `<details>` for optional or advanced content:

```mdx
<details>
<summary>Click to expand optional configuration</summary>

Content here...

</details>
```

### File naming conventions

- Use **kebab-case** for files and folders: `getting-started.mdx`, `auto-user-provisioning/`
- Use `.mdx` extension for all documentation pages
- Index pages use the folder name: `agents/agents.mdx` for the `/agents/` route

## Page templates

Docs pages fall into several types, with different audiences and structures.
Each page type corresponds to a value of the `tags` frontmatter item.

|Type|Purpose|`tags` value|
|---|---|---|
|How-to|Includes steps for achieving a concrete goal.|`how-to`|
|Reference|Includes a comprehensive list of values, commands, arguments, fields, and so on.|`reference`|
|Conceptual|Explains relationships between concepts or how something works.|`conceptual`|

### How-to guide

```mdx
---
title: How to [Do Something]
description: Learn how to [accomplish specific task] with Teleport
tags:
  - how-to
---

Brief introduction explaining what this guide accomplishes and who it's for.

## Writing style

### Document types

Each page should have a clear purpose based on one of these types:

- **Tutorials** (`get-started` tag): Learning-oriented, hands-on for
newcomers. Must provide successful outcomes with no unexpected errors.
- **How-to guides** (`how-to` tag): Task-oriented with practical
steps. Assume readers know *what* they want to do, not *how*. Avoid
breaking focus—minimize inline links, notices, and admonishments.
- **Reference** (`reference` tag): Comprehensive technical
descriptions. List all options, use tables, avoid prose.
- **Conceptual** (`conceptual` tag): Explains how/why systems work.
Provides context for tutorials and how-to guides.

### Voice and tone

- Write for technical audiences (developers, SREs, security engineers)
- Emphasize specific technical capabilities over broad benefit
statements
- Use sparse 1-2 sentence paragraphs rather than dense blocks
- Prefer paragraphs over bulleted lists; use numbered lists only for
sequential steps

### Formatting conventions

- **Headings**: Sentence case ("Next steps" not "Next Steps")
- **Page titles**: Title case, max 55 characters (70 total with " |
Teleport Docs" suffix)
- **Products/features**: Capitalize proper nouns ("Trusted Cluster").
Bold on first use.
- **Acronyms**: Introduce after the full term, then use consistently
within the page
- **Lists**: End items with periods unless the item is a command

### Component philosophy

Prefer text over components. Before adding Tabs, Admonitions, or
Details, ask whether paragraphs and headings would suffice. Use Tabs
only when a single variation is relevant to the reader and others
would distract.

## How it works

Explain the high-level architecture in 1-3 paragraphs.

## Prerequisites

(!docs/pages/includes/edition-prereqs-tabs.mdx!)

- Additional prerequisite 1

## Step 1/N. First task

Instructions...

## Step N/N. Final task

Instructions...

## Next steps

- Link to related [guide](./related-guide.mdx)
```

### Reference page

```mdx
---
title: [Feature] Reference
description: Reference documentation for [feature]
tags:
  - reference
---

Brief overview of what this reference covers.

## Configuration

### Option One

Description.

```yaml
option_one: value
```

## Related topics

- [Related Guide](./related-guide.mdx)
```
