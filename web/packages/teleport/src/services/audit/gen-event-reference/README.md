# Audit event reference generator

Audit event reference generator imports events from `web/packages/teleport/src/Audit/fixtures` and
formatters from `web/packages/teleport/src/services/audit/makeEvent.ts`. This way the reference
generator and the codebase always stay in sync when it comes to the underlying types.

Build the event fixtures:

```bash
$ pnpm build
```

Generate the reference docs:

```bash
$ pnpm gen-docs
```
