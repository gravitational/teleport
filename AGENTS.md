# AGENTS Guide

**Build / Lint / Test**
- Build: `make all` (dev), `make full` (prod), `make release`
- Clean: `make clean`
- Go lint/format: `golangci-lint run ./...`, `go fmt ./... && goimports -w .`
- JS/TS lint: `pnpm lint`, `pnpm lint-fix`, Prettier via `pnpm prettier-check`
- Test all: `make test` (Go), `pnpm test` (Jest)
- Single Go test: `go test ./... -run ^TestName$`
- Single JS test: `jest path/to/file -t 'TestName'`
- TS type check: `pnpm run type-check`

**Code Style**
- Go: camelCase vars, PascalCase types, errors end with `Err`, wrap with `fmt.Errorf`/`trace.Wrap`
- JS/TS: singleQuote, semi, tabWidth=2, import order via `.prettierrc.js`, strict types
- Naming: PascalCase components, camelCase props/functions
- Error handling: return errors (Go), React error boundaries (TS)

**Configs**
- Go: see `.golangci.yml`, `go.mod`
- JS/TS: see `.prettierrc.js`, `eslint.config.mjs`, `tsconfig.json`

**Rules**
- No Cursor rules found
- No Copilot rules found

**Suggest Manual Tests**
- Existing manual test steps found in `./.github/ISSUE_TEMPLATE/testplan.md`.
- Feature matrix with relationships found in `./feature_matrix.yml`
- Use PR description and files changed to determine features potentially impacted.
- Write a comment suggesting features that may be impacted and manual test steps to take.
