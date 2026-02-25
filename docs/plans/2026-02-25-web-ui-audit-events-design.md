# Web UI Audit Event Generation

## Problem

When `resource-gen` adds a new resource with audit events, the Go side is fully automated (event codes, types, dynamic factory, OneOf converters, TrimToMaxSize). But the web UI requires manual updates to 4 TypeScript files for the events to display in the audit log viewer. This is error-prone and easy to forget.

## Decision

Generate a single TypeScript file (`generatedResourceEvents.gen.ts`) containing all audit event metadata for resource-gen managed resources. Each of the 4 existing TS files gets a one-time import + spread to consume the generated data. After that wiring, new resources automatically appear in the web UI with zero manual TS work.

## Generated File

**Output path**: `web/packages/teleport/src/services/audit/generatedResourceEvents.gen.ts`

The file exports 5 things:

### 1. Event Codes

```typescript
export const generatedEventCodes = {
  ACCESS_POLICY_CREATE: 'AP001I',
  ACCESS_POLICY_UPDATE: 'AP002I',
  ACCESS_POLICY_DELETE: 'AP003I',
  ACCESS_POLICY_GET: 'AP004I',
  WEBHOOK_CREATE: 'WH001I',
  WEBHOOK_UPDATE: 'WH002I',
  WEBHOOK_DELETE: 'WH003I',
} as const;

export type GeneratedEventCode =
  (typeof generatedEventCodes)[keyof typeof generatedEventCodes];
```

Naming: KindPascal split to UPPER_SNAKE + operation name. "AccessPolicy" -> "ACCESS_POLICY", "Create" -> "CREATE".

### 2. RawEvent Types

```typescript
import type { RawEvent } from './types';
type HasName = { name: string };

export type GeneratedRawEvents = {
  [generatedEventCodes.ACCESS_POLICY_CREATE]: RawEvent<
    typeof generatedEventCodes.ACCESS_POLICY_CREATE, HasName>;
  // ... one entry per event code
};
```

All resource-gen events use `HasName` because they all include `ResourceMetadata.Name`.

### 3. Formatters

```typescript
export const generatedFormatters = {
  [generatedEventCodes.ACCESS_POLICY_CREATE]: {
    type: 'resource.access_policy.create',
    desc: 'Access Policy Created',
    format: ({ user, name }: { user: string; name: string }) =>
      `User [${user}] created an access policy [${name}]`,
  },
  // ... one entry per event code
};
```

- Event type: `resource.<kind>.<op>` (matches Go constants)
- Description: `<Display Name> <Past Tense>` — display name derived from KindPascal with spaces
- Format string: `User [${user}] <verb> a/an <display name> [${name}]`
- Article (a/an) derived from first letter of display name

### 4. Icon Mappings

```typescript
export const generatedEventIcons = {
  [generatedEventCodes.ACCESS_POLICY_CREATE]: 'Info',
  // ... all codes map to 'Info'
};
```

All resource CRUD events use `Icons.Info`, consistent with existing resources like CrownJewel, Plugin, UserTask.

### 5. Fixtures

```typescript
export const generatedFixtures = [
  {
    code: 'AP001I',
    event: 'resource.access_policy.create',
    name: 'example-policy',
    user: 'alice',
    time: '2026-01-01T00:00:00Z',
    uid: '00000000-0000-0000-0000-000000000001',
  },
  // ... one per event code
];
```

Minimal valid event objects with placeholder data for storybook rendering and documentation generation.

## One-Time Wiring

Each of the 4 existing files gets a small, permanent change:

### `types.ts`

```typescript
import { generatedEventCodes, type GeneratedRawEvents } from './generatedResourceEvents.gen';

export const eventCodes = {
  ...generatedEventCodes,
  // existing codes unchanged
};

export type RawEvents = GeneratedRawEvents & {
  // existing entries unchanged
};
```

### `makeEvent.ts`

```typescript
import { generatedFormatters } from './generatedResourceEvents.gen';

const formatters: Formatters = {
  ...generatedFormatters,
  // existing entries unchanged
};
```

### `EventTypeCell.tsx`

```typescript
import { generatedEventIcons } from './generatedResourceEvents.gen';

const generatedIconEntries = Object.fromEntries(
  Object.entries(generatedEventIcons).map(([code, name]) => [code, Icons[name]])
);

const EventIconMap = {
  ...generatedIconEntries,
  // existing entries unchanged
};
```

### `fixtures/index.ts`

```typescript
import { generatedFixtures } from '../generatedResourceEvents.gen';

export const events = [
  ...generatedFixtures,
  // existing fixtures unchanged
];
```

## Generator Implementation

### New file: `generators/web_events_gathering.go`

Reuses the existing `buildEventEntries()` helper from `events_gathering.go`. Computes additional per-entry fields:

- `UpperSnakeName`: "ACCESS_POLICY_CREATE" (for TS constant name)
- `DisplayName`: "Access Policy" (PascalCase split with spaces)
- `Article`: "a" or "an" (vowel check on display name)
- `PastTenseVerb`: "created", "updated", "deleted", "read"
- `EventType`: "resource.access_policy.create"
- `Code`: "AP001I"

### New template: `templates/web_events_ts.ts.tmpl`

Go text/template that produces the TypeScript file.

### CLI changes

New optional flag `--web-dir` (defaults to empty = skip TS generation). When set, the generator writes the `.gen.ts` file to `<web-dir>/packages/teleport/src/services/audit/generatedResourceEvents.gen.ts`.

### Make target

`make generate-resource-services/host` passes `--web-dir=web` to produce the TS file alongside Go files.

## Data Flow

```
resource service proto (resource_config option)
  → parser.ParseProtoDir() → []ResourceSpec
    → buildEventEntries() → []eventEntry (shared by Go + TS generators)
      → Go generators: codes.gen.go, api.gen.go, dynamic.gen.go, etc.
      → TS generator:  generatedResourceEvents.gen.ts
```

## What This Does NOT Do

- Does not generate snapshot test updates (those happen when tests run)
- Does not handle non-standard event fields (all generated events use HasName)
- Does not generate event group types (resource CRUD events don't need grouping)
- Does not customize icons per resource (all use Info; can be overridden manually)
