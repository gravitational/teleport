# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Context

This directory (`content/18.x`) is a git submodule containing Teleport documentation source for version 18.x. It lives inside the [`docs-website`](https://github.com/gravitational/docs-website) repo (one level up), which is a Docusaurus site that builds and serves the docs. All build, lint, and preview commands are run from the `docs-website` root.

## Commands (run from `../..` — the docs-website root)

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

Pages are at `docs/pages/**/*.mdx`. The `includes/` directories are excluded from page rendering and are only pulled in via the `(!path/to/file.mdx!)` include syntax.

## MDX content conventions

### Frontmatter

Every page must begin with a YAML frontmatter block. Allowed fields are defined in `docs-website/frontmatter_fields.yaml`. Key fields:

- `title` (required) — page title
- `description` (required) — meta description
- `sidebar_label` — override the sidebar display name
- `tags` — list combining a type tag (`how-to`, `conceptual`, `get-started`, `reference`, `faq`, `other`) and product tags (`zero-trust`, `mwi`, `identity-governance`, `identity-security`, `platform-wide`)

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
