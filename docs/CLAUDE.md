# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Context

The [docs-website](https://github.com/gravitational/docs-website) repo contains
source files for the Teleport docs site. In the `content` directory is a git
submodule that corresponds to each supported version of the docs site, such as
`content/18.x` (for v18 and minor versions) and `content/17.x` (for v17 and
minor versions). The highest-numbered subdirectory of `content` is for the edge
version of the docs.

Each submodule of `content` is a clone of
https://github.com/gravitational/teleport. This CLAUDE.md file is in the `docs`
directory of a single `gravitational/teleport` clone within a subdirectory of
`content`.

All build, lint, and preview commands are run from the `docs-website` root.

## Commands

Run the following commands to manage the docs website from `../../..`, the root
of the `docs-website` repo clone:

```bash
# Local development (macOS only; requires rsync, watchexec, jq via Homebrew)
yarn dev

# Full build (updates submodules, prepares files, builds static site)
yarn build

# Lint MDX content (runs remark linter on content/**/docs/pages/**/*.mdx)
yarn markdown-lint

# Run unit tests for server/ and src/ code
yarn test

# Run a single test file
yarn test -- --testPathPattern=server/remark-variables

# Type-check TypeScript
yarn typecheck

# Lint/format TypeScript and JS
yarn lint

# Prepare versioned file layout (copies MDX from content/ to docs/ and versioned_docs/)
yarn prepare-files
```

For a complete reference of `yarn` commands, read `../../../package.json`. There
are `package.json` files in subdirectories of each `content/v[0-9]+.x`
submodule, but those are for the Teleport product instead of the Teleport docs
site.

> **Note:** `yarn dev` uses `watchexec` to watch for file changes in `content/` and sync them live. Includes files trigger a full Docusaurus restart.

## Content structure (this repo)

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

## MDX content conventions

The docs site uses [Docusaurus](https://docusaurus.io/). The `yarn
prepare-files` command, when run from the `docs-website` root, copies docs page
files into the directory structure that Docusaurus expects.

### Frontmatter

Every page must begin with a YAML frontmatter block. Allowed fields are defined in `docs-website/frontmatter_fields.yaml`. Key fields:

- `title` (required) — page title
- `description` (optional: generated from the first paragraph if empty) — meta description
- `sidebar_label` — override the sidebar display name
- `tags` — list combining a type tag (`how-to`, `conceptual`, `get-started`,
  `reference`, `faq`, `other`) and product tags (`zero-trust`, `mwi`,
  `identity-governance`, `identity-security`, `platform-wide`). For a full list
  of tags, see the `tags.yml` field at the root of the `docs-website` repo.

### Special syntax

- **Variable substitution:** `(=variable.path=)` — variables are defined in `docs/config.json` under `"variables"` and resolved at build time. Example: `(=teleport.version=)`.
- **Includes:** `(!docs/pages/includes/filename.mdx!)` — inlines another MDX file. Path is relative to the version repo root. Parameters can be passed: `(!file.mdx param="value"!)`.
- **Lint suppression:** `{/* lint disable page-structure remark-lint */}` — suppresses structural lint warnings for a section.

### Page structure rules (enforced by linter)

- No H1 headings (the `title` frontmatter field renders as H1).
- Pages with step-by-step instructions should use `Step N/N` in H2 headings.
- Links to docs must be relative paths to `.mdx` files — never absolute `/docs/...` or `https://goteleport.com/docs/...` URLs.

## Architecture overview (docs-website)

The docs-website (`../..`) converts the MDX content at build time:

1. **`scripts/prepare-files.mts`** — copies `.mdx` files from `content/{version}/docs/pages/` to `docs/` (current version) or `versioned_docs/version-{name}/` (other versions), generates `sidebars.json` and `versions.json`.
2. **`config.json`** (docs-website root) — declares versions with `name`, `branch`, `isDefault`. The last version is treated as current/next.
3. **Remark plugins** (`server/`) transform content:
   - `remark-includes` — resolves `(!file!)` includes
   - `remark-variables` — resolves `(=var=)` variable syntax using `docs/config.json`
   - `remark-update-asset-paths` — rewrites asset paths to point to `content/{version}/`
   - `remark-no-h1`, `remark-code-snippet`, `remark-version-alias` — additional transforms
4. **Linters** (`server/lint-*.ts`) — run via `yarn markdown-lint` using `remark-cli`
