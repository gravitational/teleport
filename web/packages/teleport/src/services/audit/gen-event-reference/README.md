# Audit event reference generator

Audit event reference generator imports events from `web/packages/teleport/src/Audit/fixtures` and
formatters from `web/packages/teleport/src/services/audit/makeEvent.ts`. This way the reference
generator and the codebase always stay in sync when it comes to the underlying types.

To generate the reference:

```
pnpm --filter @gravitational/teleport event-reference
```

This command first runs the generator code through Vite to strip TypeScript types and then runs the
built code with Node.js to generate the reference.
