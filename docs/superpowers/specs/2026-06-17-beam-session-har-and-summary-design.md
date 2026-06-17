# Beam session: combined HAR recording, summary, and end event

Date: 2026-06-17
Status: Design — approved decisions, pending spec review
Branch context: `feature/http-recording-events`

## 1. Goal

Make a beam's activity viewable as a single "beam session":

- **Combine** all of a beam's app-session recording chunks into one HAR (HTTP
  Archive) artifact and persist it ("the beams session").
- **Surface** the beam's AI security summary (already produced by
  `BeamSummarizer`) and the combined HAR in the web UI.
- **Emit a beam-level end event** modeled on `WindowsDesktopSessionEnd` when the
  beam ends, so the beam session is discoverable and the artifacts are
  referenceable.

A "beam" is a micro-VM running a coding agent that talks to LLM APIs through the
Teleport app proxy. Each app request/response is already recorded as HTTP
recording events in per-chunk session recordings, correlated to the beam via
`beam_id` on `AppSessionChunk` events.

## 2. Current state (what already exists)

- **HTTP recording (per chunk)**: the LLM app handler records request/response
  metadata and body chunks as `AppSessionHTTPRequest`,
  `AppSessionHTTPRequestBodyChunk`, `AppSessionHTTPResponse`,
  `AppSessionHTTPResponseBodyChunk`. These are **session-recording-only**
  (`recordEvent` excludes them from the global audit emitter —
  `lib/srv/app/common/audit.go`).
- **HAR generation**: `tool/tsh/common/har.go` (`writeHAR`, full HAR 1.2 structs)
  reads those four event types and writes a HAR; used by `tsh recording export`
  (CLI only). No web path.
- **Beam infra**: `Beam` resource + `BeamService`
  (`api/proto/teleport/beams/v1/`), `beam_id` flows cert → `UserMetadata.BeamID`
  → `AppSessionChunk` events. `e/lib/auth/summarizer/beam.go`'s `BeamSummarizer`
  already: finds all chunk session IDs for a `beam_id`, merges their event
  streams, **reconstructs HTTP exchanges** (`collectHTTPExchangesFromStream`),
  enriches with SSH/k8s/db session summaries, and calls the LLM to produce a
  `schema.SessionAnalysis`. It is wired up: `BeamMonitor` calls `SummarizeBeam`
  on rogue detection, and `e/lib/beams/v1/delete_beam.go` calls it on beam
  deletion (24h lookback).
- **Storage pattern**: `SummaryUploader.UploadSummary(ctx, sessionID, reader)` /
  `BeamSummaryDownloader.StreamSessionSummary(ctx, sessionID)` store and retrieve
  per-session artifacts (S3/GCS/etc).
- **Gaps**: `SummarizeBeam`'s analysis is consumed for real-time alerting only —
  it is **not persisted** for later retrieval. There is **no combined HAR
  artifact**, **no web API** to fetch a beam's HAR/summary, **no web HAR viewer**,
  and **no beam session-end event**. `tsh/har.go` and `beam.go` contain two
  separate exchange-reconstruction implementations.

## 3. Locked decisions

1. **End event is beam-level only** — a single `beam.session.end` event when the
   beam ends, modeled on `WindowsDesktopSessionEnd`. Per-chunk
   `AppSessionChunk` events already exist for reconstruction; no per-chunk
   metrics event is added.
2. **HAR is generate-and-store on beam end** — `BeamSummarizer` generates the
   combined HAR once when the beam ends and uploads it as a stored artifact; the
   web UI downloads the stored HAR.
3. **Both phases specced; Phase A (backend) built first.**

## 4. Architecture

```text
beam runs ──► app chunks recorded (HTTP events, per chunk, keyed by beam_id)
                                   │
beam ends (delete_beam.go) ──► SummarizeBeam(beamID, window)
        ├─ findAppChunkSessionIDs ─► merge streams ─► reconstruct exchanges  (existing)
        ├─ LLM analysis (schema.SessionAnalysis)                              (existing)
        ├─ [NEW] serialize exchanges → HAR (shared builder)  ─► UploadArtifact(beam-<id>, HAR)
        ├─ [NEW] persist analysis/summary                    ─► UploadSummary(beam-<id>)
        └─ [NEW] emit beam.session.end event (window, recorded, counts, risk, artifact refs)
                                   │
web UI ──► [NEW] GET beam summary + HAR ──► render summary panel + HAR viewer
```

Four components; **C1–C3 + retrieval API = Phase A (backend)**, **C4 = Phase B
(web UI)**. C4 depends on C2/C3 and the retrieval API.

## 5. Phase A — backend (build first)

### C1. Shared HAR builder

Extract HAR 1.2 serialization out of `tool/tsh/common/har.go` into a reusable,
non-CLI package: **`lib/events/har`** (new).

- Move the HAR structs (`harRoot`, `harLog`, `harEntry`, `harRequest`,
  `harResponse`, `harTimings`, `harNameValue`, `harPostData`, `harContent`) and
  the serialization logic there, exported.
- Define a single exchange-reconstruction type and function reused by both
  callers. `beam.go`'s `httpExchange` + `collectHTTPExchangesFromStream` and
  `tsh/har.go`'s reader currently duplicate this; consolidate into the new
  package: an exchange accumulator that consumes the four event types and a
  `BuildHAR(exchanges) ([]byte, error)` (or `io.Writer`) function.
- `tsh/har.go` is refactored to call the shared package (behavior preserved;
  `recording_export_test.go` continues to pass). `beam.go` switches its
  reconstruction to the shared accumulator.

Boundary: input is a stream/slice of the four HTTP recording events; output is
HAR bytes. No knowledge of beams or tsh. Independently unit-testable.

### C2. Beam HAR artifact ("beams session")

In `SummarizeBeam` (`e/lib/auth/summarizer/beam.go`), after merging chunk
streams and reconstructing exchanges:

- Serialize the **full** exchange set (not the anomaly-sampled
  `selectExchanges` subset used for the prompt — the HAR must be complete) to
  HAR bytes via C1.
- Upload via a stored-artifact uploader keyed by the synthetic beam session ID
  `session.ID("beam-" + beamID)` (already used in `beam.go` for tracing).
  Reuse the `SummaryUploader` pattern; introduce a sibling
  `UploadArtifact(ctx, sessionID, kind, reader)` (or a dedicated HAR uploader
  interface) so HAR and summary are distinct objects under the same key.
- Persist the beam analysis/summary so the web UI can read it: store the
  `schema.SessionAnalysis` (short description, risk level, cluster interactions,
  compromise indicators) as a `Summary` (or beam-specific) resource under the
  same `beam-<id>` key via `UploadSummary`. This closes the "analysis is not
  persisted today" gap.

Generation/upload happens on the existing beam-end path
(`delete_beam.go` → `SummarizeBeam`) and is best-effort/non-fatal: a HAR upload
failure logs and does not block alerting.

### C3. `beam.session.end` audit event

New event modeled on `WindowsDesktopSessionEnd`.

- **Proto** (`api/proto/teleport/legacy/types/events/events.proto`): new
  `BeamSessionEnd` message with: `Metadata`, `UserMetadata`, `StartTime`,
  `EndTime`, `BeamID`, `Recorded` (bool), `HTTPExchangeCount` (int64),
  `RiskLevel` (string), `ChunkSessionIDs` (repeated string — the merged
  recordings), and an artifact reference (the `beam-<id>` session ID under which
  HAR + summary are stored).
- **Codes/types**: `BeamSessionEndEvent = "beam.session.end"`
  (`lib/events/api.go`), `BeamSessionEndCode = "T2019I"` (`lib/events/codes.go`,
  T2019I is free).
- **Emission**: emitted from the beam-end path — beam deletion (`delete_beam.go`,
  alongside the `SummarizeBeam` call) and, if a separate beam-expiry cleanup path
  exists, from there too — via the **global audit emitter** (searchable; not
  recording-only) so it appears in the events list and can drive future triggers.
  Carries `beam_id` so it is filterable. Verify the expiry path during
  implementation; if none exists, deletion is the sole trigger.
- Register the new event type in the proto event oneof / `FromOneOf` decoding
  and any event registry needed for `SearchEvents` to return it.

### Retrieval API (backend half of C4)

New enterprise web handler (e.g. in `e/lib/web/beam.go`):

- `GET /v1/webapi/sites/:clusterId/beams/:beamId/summary` → returns the persisted
  beam analysis/summary (JSON).
- `GET /v1/webapi/sites/:clusterId/beams/:beamId/har` → streams the stored HAR
  artifact (downloads via `StreamSessionSummary`-style reader keyed by
  `beam-<id>`).
- RBAC: gated by the same permissions as viewing beams / session recordings.

### Phase A testing

- C1: unit tests for the exchange accumulator (request/response correlation by
  `request_id`, multi-chunk body reassembly, `IsLast` handling, binary bodies)
  and `BuildHAR` (valid HAR 1.2, timings, base64 binary). `tsh`
  `recording_export_test.go` must still pass after refactor.
- C2: unit test that `SummarizeBeam` uploads a complete HAR (all exchanges, not
  the sampled subset) and persists the analysis under `beam-<id>`; upload
  failures are non-fatal.
- C3: unit test that the beam-end path emits `beam.session.end` with correct
  window, counts, risk level, and artifact refs; event round-trips through
  proto decode and `SearchEvents`.
- Retrieval API: handler tests for summary + HAR endpoints incl. RBAC denial and
  missing-artifact (404) cases.

## 6. Phase B — web UI (build second; architecture-level)

- **API client**: `e/web/teleport/src/services/beams/beams.ts` — add
  `fetchBeamSummary(beamId)` and `fetchBeamHar(beamId)`.
- **Beam detail/recording route + view**: new route (e.g.
  `/web/beams/:beamId`) and a `BeamSession` view reached from `BeamsList`.
- **Summary panel**: render the persisted analysis — risk level, short
  description, compromise indicators, and the list of cluster interactions
  (SSH/k8s/db with their own summaries/risk).
- **HAR viewer component**: render the combined HAR as a list of HTTP exchanges
  (method, URL, status, headers, request/response bodies, timings). Evaluate a
  vetted HAR-viewer library vs. a focused in-house component; lean in-house for a
  bounded request/response inspector to avoid a heavy dependency. Decision
  deferred to Phase B's own brainstorm.
- **Audit formatters**: add `beam.session.end` (T2019I) to
  `services/audit/types.ts`, `makeEvent.ts` (formatter + message), and
  `EventTypeCell.tsx` (icon), plus formatters for the HTTP-recording events
  (currently render blank) if surfaced.

### Phase B testing

- API client unit tests; `BeamSession` view + `HARViewer` component tests with
  fixture HAR; audit formatter tests for the new event code.

## 7. Open questions / risks

- **Artifact storage object model**: whether to store HAR as a second object
  under the existing summary key or introduce a dedicated artifact uploader
  interface. Resolved during Phase A implementation; leaning toward a small
  `UploadArtifact(kind)` extension to keep one storage backend.
- **HAR size**: busy beams can produce large HARs. The stored HAR is complete
  (unlike the prompt's sampled subset). Consider gzip on upload and streamed
  download; the web viewer should paginate/virtualize entries.
- **Re-summarization**: `BeamMonitor` may call `SummarizeBeam` multiple times
  during a beam's life (rogue detection) before the final delete-time run.
  Artifact upload should be idempotent/overwrite under the `beam-<id>` key, with
  the delete-time run authoritative.
- **HAR viewer library choice**: deferred to Phase B brainstorm.

## 8. Out of scope

- Per-chunk metrics end event (explicitly dropped in favor of beam-level).
- A new `BeamSessionKind` recording type / generic recordings-list integration
  (beams are surfaced via the beams pages, not the SSH/k8s/db recordings list).
- Changes to how `beam_id` is minted or how chunks are recorded.
- OpenAI/non-Anthropic provider transcript specifics (the HAR is provider-
  agnostic; it captures raw HTTP).
