# SHARED

This package contains shared code used across Gravitational packages.

## How to use it

Add `@gravitational/shared` to your package.json file.

```
"devDependencies": {
    "@gravitational/shared": "^1.0.0",
  },
```

### WASM

This package includes a WASM module built from a Rust codebase located in `packages/shared/libs/ironrdp`.

Running `pnpm build-wasm` builds the WASM binary as well as the appropriate Javascript/Typescript
bindings and types in `web/packages/shared/libs/ironrdp/pkg`.