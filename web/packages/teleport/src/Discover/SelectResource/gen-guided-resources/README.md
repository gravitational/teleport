# Guided enrollment flow docs generator

The guided enrollment flow docs generator creates a list of guided enrollment
flows and writes it to `docs/pages` as a partial. This allows the docs to
include an up-to-date record of all guided enrollment flows to make it easier to
users to determine whether to follow step-by-step guides in the docs or use a
flow in the Web UI.

The generator imports resource definitions from
`web/packages/teleport/src/Discover/SelectResource/resources.ts`.

To generate the docs partial:

```
pnpm --filter @gravitational/teleport gen-docs
```

You can find the Vite config for this generator at
`web/packages/build/vite/gen-guided-resources-config.mts`.
