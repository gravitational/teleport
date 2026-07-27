/**
 * DedicatedWorker variant of `sharedSessionWorker.ts` for the codec-test
 * harness. Owns the WebSocket, the IronRDP `WorkerDecoder`, and a
 * `VideoDecoder` per AVC surface. Main hands over its visible canvas via
 * `transferControlToOffscreen` so wasm renders directly into pixels the
 * user sees — every paint path (fast-path bitmap PDU, EGFX ClearCodec
 * via `EgfxBitmap`, decoded AVC via `blitAvcRgba`) converges on the
 * wasm framebuffer.
 *
 * Differences from `sharedSessionWorker.ts`:
 *  - One client (the main window), so no port management, no broadcast.
 *  - The H.264 `VideoDecoder` per surface lives here (not in main); the
 *    decoded RGBA is fed straight into the wasm fb via `blitAvcRgba`.
 *  - No SPS replay cache: only one decoder exists, and it sees every chunk
 *    in order from the moment the WS opens.
 *  - No layout computation: main owns the multi-monitor layout (it opens
 *    the popups) and encodes the ScreenSpec itself.
 */

import init, {
  initWasmLog,
  MainCodec,
  setSimdIdwt,
  WorkerDecoder,
} from './pkg/session';
import { installWorkerLogSink, setLogPerfProvider } from './logsink';
import {
  runGpuPlumbingBench,
  formatGpuBench,
  type GpuBenchResult,
} from './gpuPlumbingBench';
// Step 2: the dedicated RFX/ClearCodec decode worker (Vite `?worker` — a nested
// DedicatedWorker). Spawned in start(); EGFX decode is offloaded to it so it no
// longer blocks this worker's WS pump / outbound input.
import RfxDecodeWorker from './rfxDecodeWorker?worker';

// Install the log sink first so it captures every wasm `console::warn` probe
// (and our own logs) from the very first line. See logsink.ts.
installWorkerLogSink();

// DIAGNOSTIC (revert): muted worker lifecycle log.
// console.log(
//   '[codec-test-worker] module loaded at',
//   new Date().toISOString()
// );

type State = 'closed' | 'authing' | 'open' | 'error';

interface HostMessage {
  type: 'host';
  wsUrl: string;
  bearerToken: string;
  /** Diagnostic A/B (RFX_POOL_CORRUPTION_HANDOFF "First diagnostic"): when
   * false, skip the decode-worker pool and decode RFX inline (the original
   * wire-ordered path) so the offload-vs-inline corruption can be isolated.
   * Defaults to true (offload on). Driven by `?offload=0` on the page URL. */
  egfxOffload?: boolean;
  /** Override the decode-worker pool size (`?workers=N`); else hardwareConcurrency-derived. */
  workers?: number;
}

interface OutboundMessage {
  type: 'outbound';
  buffer: ArrayBuffer;
}

interface UnregisterMessage {
  type: 'unregister';
}

/// Main hands the worker the controlling `OffscreenCanvas` for its visible
/// canvas (via `transferControlToOffscreen`). The worker registers it with
/// the wasm framebuffer so every paint path (fast-path PDU, EgfxBitmap,
/// AVC `blitAvcRgba`) renders directly into pixels the user sees.
interface RegisterCanvasMessage {
  type: 'register-canvas';
  canvasId: number;
  canvas: OffscreenCanvas;
  /** Viewport into the virtual desktop this canvas displays. For codec-test
   * single-monitor this is `(0, 0, window.innerWidth, window.innerHeight)`. */
  viewportX: number;
  viewportY: number;
  viewportWidth: number;
  viewportHeight: number;
}

/// Drop a previously-registered canvas (its monitor's window closed). The wasm
/// frees the per-canvas GL painter / CanvasView.
interface UnregisterCanvasMessage {
  type: 'unregister-canvas';
  canvasId: number;
}

/// Re-point a registered canvas at a new viewport without re-transferring the
/// OffscreenCanvas (used when a monitor closes and survivors reflow).
interface RepositionCanvasMessage {
  type: 'reposition-canvas';
  canvasId: number;
  viewportX: number;
  viewportY: number;
  viewportWidth: number;
  viewportHeight: number;
}

/** Run the GPU plumbing microbench in the worker (its own OffscreenCanvas, so it
 * never touches the live framebuffer) and reply with `gpuBenchResult`. Triggered
 * from the main console via `gpuBench()`. */
interface GpuBenchMessage {
  type: 'gpuBench';
  numTiles?: number;
  passesPerComponent?: number;
  iters?: number;
  bits?: 16 | 32;
}

/** Flip the SIMD inverse-DWT on or off in every pool worker + this worker's
 * inline decoder (main → codecTestWorker → broadcast to pool). */
interface SetSimdIdwtMessage {
  type: 'set-simd-idwt';
  on: boolean;
}

/** Reset the rolling perf accumulators (rfxStages + pacing histograms). */
interface ResetPerfMessage {
  type: 'reset-perf';
}

/** Ask the worker for structured perf data; it replies with `perf-data`. */
interface PerfDataMessage {
  type: 'perf-data';
}

type ClientToWorker =
  | HostMessage
  | OutboundMessage
  | UnregisterMessage
  | RegisterCanvasMessage
  | UnregisterCanvasMessage
  | RepositionCanvasMessage
  | GpuBenchMessage
  | SetSimdIdwtMessage
  | ResetPerfMessage
  | PerfDataMessage;

interface StateMessage {
  type: 'state';
  state: State;
  message?: string;
}

type DecoderEvent =
  | { type: 'log'; text: string }
  | { type: 'decoded'; text: string }
  | { type: 'resolution'; width: number; height: number }
  | { type: 'tdpbUpgrade' }
  | {
      type: 'cursorBitmap';
      imageData: ImageData;
      width: number;
      height: number;
      hotspotX: number;
      hotspotY: number;
    }
  | { type: 'cursorHidden' }
  | { type: 'cursorDefault' }
  | {
      type: 'avcChunk';
      surface: number;
      desktopX: number;
      desktopY: number;
      destWidth: number;
      destHeight: number;
      codecId: number;
      encoding: number;
      luma: Uint8Array;
      chroma?: Uint8Array;
    }
  // Step 2 (offload): emitted by the wasm WorkerDecoder when EGFX decode is
  // offloaded. Handled internally (forwarded to the decode worker / barrier),
  // never posted to main — see WorkerToClient's Exclude below.
  | {
      type: 'rfxChunk';
      // Wire-order seq assigned by the wasm paint queue; echoed back on each
      // partition's reply and used to signal the queue when all replies are in.
      seq: number;
      surfaceId: number;
      ctxId: number;
      originX: number;
      originY: number;
      surfaceWidth: number;
      surfaceHeight: number;
      payload: Uint8Array;
    }
  | { type: 'egfxDeleteContext'; surfaceId: number; ctxId: number }
  | { type: 'egfxEndFrame' }
  // Posted by wasm when its wire-order drain flushed a frame to GL (present is
  // wasm-driven now); used only for the `[pacing]` present-cadence diagnostic.
  | { type: 'egfxPresented' };

/** Decoder events handled inside the worker, never forwarded to main. */
type InternalDecoderEvent = {
  type:
    | 'avcChunk'
    | 'rfxChunk'
    | 'egfxDeleteContext'
    | 'egfxEndFrame'
    | 'egfxPresented';
};

interface GpuBenchResultMessage {
  type: 'gpuBenchResult';
  result: GpuBenchResult;
}

/** Structured perf snapshot; reply to `perf-data` queries from the console. */
export interface PerfDataSnapshot {
  stages: { tiles: number; entropy: number; idwt: number; color: number };
  present: { p50: number; p95: number };
  arrival: { p50: number };
  inflight: number;
}

interface PerfDataResultMessage {
  type: 'perf-data';
  data: PerfDataSnapshot;
}

type WorkerToClient =
  | StateMessage
  | Exclude<DecoderEvent, InternalDecoderEvent>
  | GpuBenchResultMessage
  | PerfDataResultMessage;

let ws: WebSocket | null = null;
let state: State = 'closed';
let lastErrorMessage: string | undefined;

let wasmReady: Promise<void> | null = null;
let decoder: WorkerDecoder | null = null;
let responseCodec: MainCodec | null = null;

interface AvcDecoderEntry {
  decoder: VideoDecoder;
  configured: boolean;
  // DIAGNOSTIC (revert): one-time NAL-format probe flag + per-surface
  // decode/output tallies so we can tell (a) whether the wire H.264 is
  // Annex-B or length-prefixed, and (b) whether the decoder desyncs
  // (outputs < decodes => dropped frames => pending FIFO drift).
  loggedInit: boolean;
  counters: { decodes: number; outputs: number };
  pending: Array<{
    desktopX: number;
    desktopY: number;
    destWidth: number;
    destHeight: number;
  }>;
  // Pooled copyTo buffers (reused across frames to avoid a per-frame alloc;
  // full-screen AVC is ~8 MB/frame). copyTo is async so this is a small pool,
  // not a single buffer.
  rgbaPool: Uint8Array[];
}

const avcDecoders = new Map<number, AvcDecoderEntry>();

// ── EGFX decode offload to a POOL of N decode workers ───────────────────────
// When offload is on, the wasm WorkerDecoder emits rfxChunk/egfxDeleteContext/
// egfxEndFrame events instead of decoding RFX inline. RFX PDUs are BROADCAST to
// all N workers; each decodes only the tiles it owns (stable position partition)
// and returns a partial blob. We forward each returned blob to the wasm paint
// queue (`stageRfxBlob`) and, once all N partitions for a seq have replied, mark
// it ready (`rfxSeqReady`). Wire ORDER + present now live in wasm: the paint
// queue applies every EGFX op (RFX + the inline SurfaceToSurface/SolidFill/
// ClearCodec/… decoded on the main thread) in seq order and flushes at the
// EndFrame Present marker — so an inline op never reads the framebuffer ahead of
// a prior in-flight RFX blit (the window-drag corruption fix). JS keeps no
// present barrier; it only counts replies per seq. See RFX_POOL_STEP3_PLAN.md
// and RFX_POOL_CORRUPTION_HANDOFF.md.

const decodeWorkers: Worker[] = [];
// Replies still expected per RFX seq (one per position-partition worker). The
// seq is assigned by the wasm WorkerDecoder in wire order and echoed on each
// reply; when the count hits 0 we tell wasm the seq's tiles are all in
// (`rfxSeqReady`) and the wasm wire-order drain composites + presents. Present
// ordering lives in wasm now (the paint queue) — JS keeps no present barrier.
const remaining = new Map<number, number>();

// Pacing diagnostics (smoothness): per ~1s, the distribution of
//   arrival = EndFrame inter-arrival (server/network delivery cadence)
//   present = present inter-arrival (actual display cadence)
//   barrier = EndFrame→present wait (decode-pool latency contribution)
// Jittery arrival => network/server bound; steady arrival + jittery barrier =>
// decode-pool bound (SIMD/more workers help); steady both but still janky =>
// present/vsync pacing (jitter buffer helps).
const pacing = {
  lastArrival: 0,
  lastPresent: 0,
  arrival: [] as number[],
  present: [] as number[],
  barrier: [] as number[],
  lastLog: 0,
};

function pacingPct(xs: number[]): string {
  if (xs.length === 0) return 'n/a';
  const s = [...xs].sort((a, b) => a - b);
  const at = (p: number) =>
    s[Math.min(s.length - 1, Math.floor((p / 100) * s.length))];
  return `p50=${at(50).toFixed(1)} p95=${at(95).toFixed(1)} p99=${at(99).toFixed(1)}`;
}

function pacingMaybeLog(now: number) {
  if (pacing.lastLog === 0) {
    pacing.lastLog = now;
    return;
  }
  if (now - pacing.lastLog < 1000) return;
  // eslint-disable-next-line no-console
  console.log(
    `[pacing] frames=${pacing.present.length} ` +
      `arrival{${pacingPct(pacing.arrival)}} ` +
      `present{${pacingPct(pacing.present)}} ` +
      `barrier{${pacingPct(pacing.barrier)}} ms`
  );
  pacing.arrival.length = 0;
  pacing.present.length = 0;
  pacing.barrier.length = 0;
  pacing.lastLog = now;
}
/** Replies from a decode worker (rfxDecodeWorker.ts). */
type DecodeReply =
  | { type: 'ready' }
  | { type: 'fatal'; error: string }
  | { type: 'rfxDone'; seq: number; blob?: Uint8Array }
  | {
      type: 'rfxStats';
      entropyMs: number;
      idwtMs: number;
      colorMs: number;
      tiles: number;
    };

// [perf-probe] Pool-aggregated RFX per-stage decode time (summed across all
// decode workers since start). Surfaced in perfSnapshot()/L.perf() to read the
// entropy:IDWT:color split on the offload path — the input to the GPU-offload
// sizing (IDWT is what moves to the GPU; entropy stays on the CPU).
const rfxStages = { entropyMs: 0, idwtMs: 0, colorMs: 0, tiles: 0 };

function dispatchRfx(msg: Extract<DecoderEvent, { type: 'rfxChunk' }>) {
  const n = decodeWorkers.length;
  if (n === 0) return;
  const seq = msg.seq; // wire-order seq assigned by the wasm paint queue
  remaining.set(seq, n); // every worker replies (possibly an empty blob)
  // Broadcast the encoded PDU to every worker. NO transfer: one buffer can't be
  // transferred to N recipients, so each postMessage structured-clones it (the
  // encoded PDU is KiB — cheap). Each worker decodes only its (workerIndex,n)
  // partition.
  const buf = msg.payload.buffer as ArrayBuffer;
  for (let k = 0; k < n; k++) {
    decodeWorkers[k].postMessage({
      type: 'rfx',
      seq,
      surfaceId: msg.surfaceId,
      ctxId: msg.ctxId,
      workerIndex: k,
      numWorkers: n,
      surfaceWidth: msg.surfaceWidth,
      surfaceHeight: msg.surfaceHeight,
      payload: buf,
    });
  }
}

function dispatchDeleteContext(surfaceId: number, ctxId: number) {
  // Broadcast: any worker may own this surface's tiles.
  for (const w of decodeWorkers) {
    w.postMessage({ type: 'delete', surfaceId, ctxId });
  }
}

/**
 * Record an EndFrame boundary for the `[pacing]` arrival-cadence diagnostic.
 * The actual present is driven by the wasm wire-order drain (it enqueues a
 * Present marker at this boundary and flushes once the frame's ops have all
 * applied), reported back via the `egfxPresented` event (`notePresented`).
 */
function noteEndFrame() {
  const now = performance.now();
  if (pacing.lastArrival > 0) pacing.arrival.push(now - pacing.lastArrival);
  pacing.lastArrival = now;
  pacingMaybeLog(now);
}

/** wasm flushed a frame to GL: record present cadence. `barrier` is the most
 * recent EndFrame→present wait (approximate now that present is wasm-driven). */
function notePresented() {
  const now = performance.now();
  if (pacing.lastArrival > 0) pacing.barrier.push(now - pacing.lastArrival);
  if (pacing.lastPresent > 0) pacing.present.push(now - pacing.lastPresent);
  pacing.lastPresent = now;
}

/** One expected reply for `seq` arrived. When every partition has replied, tell
 * the wasm paint queue the seq is complete so its wire-order drain composites
 * the tiles (and any inline ops / EndFrame present waiting behind it). */
function onReply(seq: number) {
  const left = (remaining.get(seq) ?? 0) - 1;
  if (left <= 0) {
    remaining.delete(seq);
    if (decoder) decoder.rfxSeqReady(seq);
  } else {
    remaining.set(seq, left);
  }
}

/**
 * A decode worker failed or died — stop offloading and recover. Without this the
 * dead worker's replies never arrive, so its in-flight RFX seqs never reach
 * `rfxSeqReady`, the wasm wire-order queue head stays pending, and the drain
 * (hence ALL later ops + presents) wedges permanently — a screen freeze.
 *
 * `setEgfxOffload(false)` drains the queue's ready prefix and clears the rest
 * (so the never-arriving replies can't wedge it), then `resetEgfxOffloadState`
 * tears the pool down. Subsequent EGFX decodes run inline on the main thread.
 *
 * Caveat (pre-existing, unavoidable for any mid-stream fallback): the inline
 * RFX-Progressive decoders start from an empty difference chain, so RFX may show
 * artifacts until the next server keyframe/refresh — but that beats a freeze.
 * Idempotent: safe if several workers report failure.
 */
function fallbackToInline(reason: string) {
  if (decodeWorkers.length === 0 && !decoder) return;
  // eslint-disable-next-line no-console
  console.warn(`[codec-test-worker] EGFX offload → inline fallback: ${reason}`);
  if (decoder) decoder.setEgfxOffload(false);
  resetEgfxOffloadState();
}

function handleDecodeWorkerMessage(e: MessageEvent<DecodeReply>) {
  const msg = e.data;
  switch (msg.type) {
    case 'ready':
      // Workers buffer requests until their wasm is ready; offload was enabled
      // at spawn, so nothing to flush here.
      return;
    case 'fatal':
      fallbackToInline(`worker init failed: ${msg.error}`);
      return;
    case 'rfxDone': {
      // Stage this partition's tiles into the wasm paint queue (the blit origin
      // is held wasm-side). Composited in wire order once all partitions are in.
      if (msg.blob && decoder) decoder.stageRfxBlob(msg.seq, msg.blob);
      onReply(msg.seq);
      return;
    }
    case 'rfxStats':
      rfxStages.entropyMs += msg.entropyMs;
      rfxStages.idwtMs += msg.idwtMs;
      rfxStages.colorMs += msg.colorMs;
      rfxStages.tiles += msg.tiles;
      return;
  }
}

function resetEgfxOffloadState() {
  for (const w of decodeWorkers) {
    try {
      w.terminate();
    } catch {
      /* ignore */
    }
  }
  decodeWorkers.length = 0;
  remaining.clear();
  pacing.lastArrival = 0;
  pacing.lastPresent = 0;
  pacing.arrival.length = 0;
  pacing.present.length = 0;
  pacing.barrier.length = 0;
}

// ── AVC444v2 chroma path (increment 1: plumbing + validation) ───────────────
//
// For AVC444/v2 each avcChunk carries BOTH the main (luma) H.264 view and the
// auxiliary (chroma) view: encoding 0=LUMA_AND_CHROMA, 1=LUMA-only, 2=CHROMA-only
// (the aux stream is always in `m.chroma`). The main view alone decodes as a
// 4:2:0 frame → soft text; full 4:4:4 needs decoding the aux view too and
// recombining (YUV420→YUV444 per MS-RDPEGFX §3.3.8.3.3 / FreeRDP
// general_ChromaV2ToYUV444 — increment 2).
//
// This increment is ADDITIVE and OFF the display path: it decodes the aux
// stream into a second per-surface VideoDecoder purely to (a) prove the aux
// stream decodes and (b) PROBE whether the browser exposes planar I420 for it
// (the prerequisite for recombination — the single biggest unknown). It never
// touches pixels, so the existing luma display is unchanged. Replace with the
// real recombine in increment 2.
interface ChromaDecoderEntry {
  decoder: VideoDecoder;
  configured: boolean;
  loggedProbe: boolean;
}
const chromaDecoders = new Map<number, ChromaDecoderEntry>();
// LC/encoding distribution: tells us what the host actually sends per frame.
const avc444EncCounts = { both: 0, lumaOnly: 0, chromaOnly: 0 };
// Per-frame codecId tally. 11=AVC420 (arrived as EgfxAvc420 → chroma is ALWAYS
// nulled by apply_egfx_avc420, lib.rs), 14=AVC444, 15=AVC444v2 (EgfxAvcFrame →
// real chroma forwarded). If this shows 11, the host is sending 420 and there is
// NO chroma to recombine no matter how much motion there is.
const avc444ByCodecId: Record<number, number> = {};

// [perf-probe] DIAGNOSTIC (remove after root-causing the YouTube perf/memory
// blowup): tally GPU-fast-path vs CPU-copyTo-fallback presents. The snapshot is
// exposed ON DEMAND via `L.perf()` (run in the main DevTools console; it
// round-trips to this worker through the logsink) — no periodic console spam.
// Run it a few times while video plays to read the trend. It discriminates the
// candidate causes:
//   cpu >> gpu            → CPU readback storm (GPU fast path failing every frame)
//   queueSize climbing    → decoder backpressure (decode outruns present)
//   gap / heapMB climbing → a VideoFrame/buffer leak
//   chroma queueSize > 0  → the AVC444 double-decode (feedChroma) is live load
const perfProbe = { gpu: 0, cpu: 0 };
function perfStructured(): PerfDataSnapshot {
  const tiles = rfxStages.tiles || 1;
  const usPerTile = (ms: number) => (ms / tiles) * 1000;
  const pctAt = (xs: number[], p: number) => {
    if (xs.length === 0) return 0;
    const s = [...xs].sort((a, b) => a - b);
    return s[Math.min(s.length - 1, Math.floor((p / 100) * s.length))];
  };
  return {
    stages: {
      tiles: rfxStages.tiles,
      entropy: usPerTile(rfxStages.entropyMs),
      idwt: usPerTile(rfxStages.idwtMs),
      color: usPerTile(rfxStages.colorMs),
    },
    present: { p50: pctAt(pacing.present, 50), p95: pctAt(pacing.present, 95) },
    arrival: { p50: pctAt(pacing.arrival, 50) },
    inflight: remaining.size,
  };
}

function perfSnapshot(): string {
  let mainQ = 0;
  let mainDec = 0;
  let mainOut = 0;
  let mainPending = 0;
  for (const e of avcDecoders.values()) {
    mainQ += e.decoder.decodeQueueSize;
    mainDec += e.counters.decodes;
    mainOut += e.counters.outputs;
    mainPending += e.pending.length;
  }
  let chromaQ = 0;
  for (const e of chromaDecoders.values()) chromaQ += e.decoder.decodeQueueSize;
  // performance.memory is Chrome-only and may be absent in a worker; guard it.
  const mem = (performance as unknown as { memory?: { usedJSHeapSize: number } })
    .memory;
  const heapMB = mem ? Math.round(mem.usedJSHeapSize / 1048576) : -1;
  // RFX section first — it's the active path on AVC-disabled hosts. `inflight`
  // is the pool backlog (in-flight RFX seqs awaiting all-N replies); a climbing
  // value = the pool can't keep up. `present` percentiles are the real on-screen
  // cadence (ms between frames); `arrival` is the server delivery cadence.
  // (pacing arrays cover ~1s — they reset each [pacing] log.)
  // Per-tile µs for each decode stage (cumulative across the pool) — the ratio
  // entropy:idwt:color sizes the GPU-offload win (idwt+color move to GPU; entropy
  // stays CPU). Needs `rfx-stage-timing` in the wasm build (release has it).
  const snap = perfStructured();
  const f = (n: number) => n.toFixed(1);
  return (
    `[perf-probe] rfx{workers=${decodeWorkers.length} inflight=${snap.inflight} ` +
    `arrival[p50=${f(snap.arrival.p50)}] present[p50=${f(snap.present.p50)} p95=${f(snap.present.p95)}] barrier[${pacingPct(pacing.barrier)}] ` +
    `stages{tiles=${snap.stages.tiles} us/tile: entropy=${f(snap.stages.entropy)} ` +
    `idwt=${f(snap.stages.idwt)} color=${f(snap.stages.color)}}} ` +
    `avc{decoders=${avcDecoders.size} decodes=${mainDec} outputs=${mainOut} gap=${mainDec - mainOut} ` +
    `queueSize=${mainQ} present{gpu=${perfProbe.gpu} cpu=${perfProbe.cpu}} chromaDecoders=${chromaDecoders.size}} ` +
    `heapMB=${heapMB}`
  );
}
// Expose as `L.perf()` (replaces the periodic dump). Hoisted fn decl, so this
// registration is safe even though `installWorkerLogSink` ran at module top.
setLogPerfProvider(perfSnapshot);

function feedChroma(m: Extract<DecoderEvent, { type: 'avcChunk' }>) {
  if (m.encoding === 0) avc444EncCounts.both++;
  else if (m.encoding === 1) avc444EncCounts.lumaOnly++;
  else if (m.encoding === 2) avc444EncCounts.chromaOnly++;
  avc444ByCodecId[m.codecId] = (avc444ByCodecId[m.codecId] ?? 0) + 1;
  const total =
    avc444EncCounts.both + avc444EncCounts.lumaOnly + avc444EncCounts.chromaOnly;
  if (total === 1 || total % 120 === 0) {
    // eslint-disable-next-line no-console
    console.log(
      `[avc444] enc distribution: both(0)=${avc444EncCounts.both} ` +
        `lumaOnly(1)=${avc444EncCounts.lumaOnly} chromaOnly(2)=${avc444EncCounts.chromaOnly} ` +
        `| codecId counts=${JSON.stringify(avc444ByCodecId)} (11=AVC420/no-chroma, 14/15=AVC444/v2)`
    );
  }

  const chroma = m.chroma;
  if (!chroma || chroma.length === 0) return; // luma-only frame: no aux to decode

  let entry = chromaDecoders.get(m.surface);
  if (!entry) {
    const surface = m.surface;
    const vd = new VideoDecoder({
      output: frame => {
        const e = chromaDecoders.get(surface);
        if (e && !e.loggedProbe) {
          e.loggedProbe = true;
          // THE validation: can we read planar I420 (the planes we'd recombine)?
          let planar: string;
          try {
            const sz = frame.allocationSize({ format: 'I420' });
            planar = `I420 copyTo OK (allocationSize=${sz}B, nativeFormat=${frame.format})`;
          } catch (err) {
            planar = `I420 copyTo UNSUPPORTED (nativeFormat=${frame.format}): ${
              err instanceof Error ? err.message : String(err)
            }`;
          }
          // eslint-disable-next-line no-console
          console.log(
            `[avc444] chroma surface=${surface} decoded coded=${frame.codedWidth}x${frame.codedHeight} ` +
              `display=${frame.displayWidth}x${frame.displayHeight} — planar probe: ${planar}`
          );
        }
        // Increment 1: instrumentation only — do not composite. Release it.
        frame.close();
      },
      error: e => {
        // eslint-disable-next-line no-console
        console.warn(`[avc444] chroma decoder error surface=${surface}:`, e);
      },
    });
    entry = { decoder: vd, configured: false, loggedProbe: false };
    chromaDecoders.set(m.surface, entry);
  }

  if (!entry.configured) {
    const codecString = codecStringFromAvc(chroma);
    if (!codecString) return; // wait for a chroma keyframe carrying SPS
    try {
      entry.decoder.configure({
        codec: codecString,
        hardwareAcceleration: 'prefer-hardware',
        optimizeForLatency: true,
      });
      entry.configured = true;
    } catch (e) {
      // eslint-disable-next-line no-console
      console.warn('[avc444] chroma configure failed:', e);
      return;
    }
  }

  const chunkType: EncodedVideoChunkType = avcContainsIdr(chroma)
    ? 'key'
    : 'delta';
  let chunk: EncodedVideoChunk;
  try {
    chunk = new EncodedVideoChunk({
      type: chunkType,
      timestamp: performance.now() * 1000,
      data: chroma,
    });
  } catch (e) {
    // eslint-disable-next-line no-console
    console.warn('[avc444] chroma EncodedVideoChunk failed:', e);
    return;
  }
  try {
    entry.decoder.decode(chunk);
  } catch (e) {
    // eslint-disable-next-line no-console
    console.warn('[avc444] chroma decode failed:', e);
  }
}

declare const self: {
  addEventListener(
    type: 'message',
    listener: (event: MessageEvent<ClientToWorker>) => void
  ): void;
  postMessage(msg: WorkerToClient, transfer?: Transferable[]): void;
};

self.addEventListener('message', (e: MessageEvent<ClientToWorker>) => {
  handleClientMessage(e.data);
});

function handleClientMessage(msg: ClientToWorker) {
  switch (msg.type) {
    case 'host':
      if (state === 'closed') {
        void start(msg.wsUrl, msg.bearerToken, msg.egfxOffload ?? true, msg.workers);
      }
      break;
    case 'outbound':
      if (ws && state === 'open') {
        ws.send(msg.buffer);
      }
      break;
    case 'unregister':
      closeSession();
      break;
    case 'gpuBench': {
      // GPU plumbing throughput microbench on its own OffscreenCanvas (doesn't
      // touch the live framebuffer). Logs the result (readable via L) AND replies.
      const result = runGpuPlumbingBench(
        msg.numTiles,
        msg.passesPerComponent,
        msg.iters,
        msg.bits
      );
      // eslint-disable-next-line no-console
      console.log(formatGpuBench(result));
      post({ type: 'gpuBenchResult', result });
      break;
    }
    case 'register-canvas':
      registerCanvas(msg);
      break;
    case 'unregister-canvas':
      if (decoder) {
        try {
          decoder.removeCanvas(msg.canvasId);
        } catch (e) {
          // eslint-disable-next-line no-console
          console.warn('[codec-test-worker] removeCanvas failed', e);
        }
      }
      break;
    case 'reposition-canvas':
      if (decoder) {
        try {
          decoder.repositionViewport(
            msg.canvasId,
            msg.viewportX,
            msg.viewportY,
            msg.viewportWidth,
            msg.viewportHeight
          );
        } catch (e) {
          // eslint-disable-next-line no-console
          console.warn('[codec-test-worker] repositionViewport failed', e);
        }
      }
      break;
    case 'set-simd-idwt':
      // Broadcast to every pool worker so their per-worker wasm instances
      // pick up the toggle.
      for (const w of decodeWorkers) {
        w.postMessage({ type: 'set-simd-idwt', on: msg.on });
      }
      // Also toggle this worker's own inline decoder path (used when the
      // decode-worker pool is off or as the main-thread EGFX inline path).
      try {
        // setSimdIdwt is a statically-imported free wasm-bindgen export (same as
        // rfxDecodeWorker). Calling before init() would throw — caught below.
        setSimdIdwt(msg.on);
      } catch {
        // ignore — the inline path / wasm may not be initialized yet
      }
      break;
    case 'reset-perf':
      // Clear the rolling accumulators used by perfSnapshot()/perfStructured().
      rfxStages.entropyMs = 0;
      rfxStages.idwtMs = 0;
      rfxStages.colorMs = 0;
      rfxStages.tiles = 0;
      pacing.arrival.length = 0;
      pacing.present.length = 0;
      pacing.barrier.length = 0;
      pacing.lastArrival = 0;
      pacing.lastPresent = 0;
      pacing.lastLog = 0;
      // Also reset the per-worker stage-timing accumulators.
      for (const w of decodeWorkers) {
        w.postMessage({ type: 'reset-perf' });
      }
      break;
    case 'perf-data':
      post({ type: 'perf-data', data: perfStructured() });
      break;
  }
}

function registerCanvas(msg: RegisterCanvasMessage) {
  // The wasm decoder accepts canvas registrations before ServerHello — it
  // queues them and replays once the framebuffer is created.
  void ensureDecoderReady().then(() => {
    if (!decoder) return;
    try {
      decoder.addCanvas(
        msg.canvasId,
        msg.canvas,
        msg.viewportX,
        msg.viewportY,
        msg.viewportWidth,
        msg.viewportHeight
      );
      // DIAGNOSTIC (revert): muted addCanvas log.
      // console.log(
      //   `[codec-test-worker] addCanvas id=${msg.canvasId} viewport=(${msg.viewportX},${msg.viewportY}) ${msg.viewportWidth}x${msg.viewportHeight}`
      // );
    } catch (e) {
      // eslint-disable-next-line no-console
      console.warn('[codec-test-worker] addCanvas failed', e);
    }
  });
}

/// Resolve once the wasm module + decoder instance are both ready. Pre-host
/// canvas registrations land here and wait for `start()` to set `decoder`.
function ensureDecoderReady(): Promise<void> {
  return new Promise(resolve => {
    const tick = () => {
      if (decoder) {
        resolve();
        return;
      }
      setTimeout(tick, 16);
    };
    tick();
  });
}

async function start(url: string, bearerToken: string, egfxOffload = true, workersOverride?: number) {
  // DIAGNOSTIC (revert): muted.
  // console.log('[codec-test-worker] start() invoked, url=', url);
  if (!wasmReady) {
    wasmReady = init().then(() => initWasmLog('debug'));
  }
  try {
    await wasmReady;
  } catch (e) {
    setError(`wasm init failed: ${e}`);
    return;
  }
  // DIAGNOSTIC (revert): muted.
  // console.log('[codec-test-worker] wasm ready; opening WS');

  responseCodec = new MainCodec();
  decoder = new WorkerDecoder(handleDecoderEvent);

  // Step 3: spawn a POOL of N EGFX decode workers and offload decode to them.
  // Workers buffer requests until their wasm initializes, so enabling offload now
  // (before any EGFX PDU arrives) guarantees they see every PDU in order — no
  // inline→offload switch mid-stream that would desync the RFX difference chain.
  // N = `?workers=` override, else hardwareConcurrency − 2 (reserve cores for this
  // WS/input/render worker), clamped to [1, 8]. MEASURED (2026-06-17, 16-logical
  // Apple Silicon, 5K@2x video): raising the cap to 16 BACKFIRED — 14 workers
  // oversubscribed (per-tile 147µs vs 73µs, present ~5fps vs ~15fps). hardwareConcurrency
  // counts E-cores + hyperthreads that HURT decode, so it's a poor proxy; the 8 ceiling
  // is validated and the real sweet spot here is ~6 — more lanes cost dispatch + CPU
  // contention with the render thread. `?workers=N` stays for per-host tuning.
  // If any spawn throws, tear down and stay on the proven inline path.
  if (egfxOffload) {
    try {
      const hc = navigator.hardwareConcurrency || 4;
      const n =
        workersOverride && workersOverride > 0
          ? Math.max(1, Math.min(64, Math.floor(workersOverride)))
          : Math.max(1, Math.min(8, hc - 2));
      for (let k = 0; k < n; k++) {
        const w = new RfxDecodeWorker();
        w.addEventListener('message', handleDecodeWorkerMessage);
        // A worker that dies AFTER init (OOM, or a wasm panic/abort inside
        // decode — not a catchable JS error, so the worker's own try/catch can't
        // echo a reply) would otherwise stop replying and wedge the wire-order
        // queue forever. Detect it and fall back to inline.
        w.addEventListener('error', ev =>
          fallbackToInline(`worker error: ${ev.message || 'unknown'}`)
        );
        w.addEventListener('messageerror', () =>
          fallbackToInline('worker messageerror (uncloneable reply)')
        );
        decodeWorkers.push(w);
      }
      decoder.setEgfxOffload(true);
    } catch (e) {
      // eslint-disable-next-line no-console
      console.warn(
        '[codec-test-worker] decode worker pool spawn failed; inline EGFX decode',
        e
      );
      resetEgfxOffloadState();
    }
  } else {
    // eslint-disable-next-line no-console
    console.log(
      '[codec-test-worker] EGFX offload DISABLED (?offload=0): inline RFX decode'
    );
  }

  openWebSocket(url, bearerToken);
}

function handleDecoderEvent(
  msg: DecoderEvent | { type: 'rdpResponse'; buffer: ArrayBuffer }
) {
  switch (msg.type) {
    case 'rdpResponse': {
      if (!ws || state !== 'open' || !responseCodec) return;
      try {
        const wrapped = responseCodec.encodeRdpResponse(
          new Uint8Array(msg.buffer)
        );
        ws.send(wrapped.buffer as ArrayBuffer);
      } catch (e) {
        // eslint-disable-next-line no-console
        console.warn('[codec-test-worker] encodeRdpResponse failed', e);
      }
      return;
    }
    case 'tdpbUpgrade':
      if (responseCodec) responseCodec.upgradeToTdpb();
      post(msg);
      return;
    case 'avcChunk':
      handleAvcChunk(msg);
      return;
    case 'rfxChunk':
      dispatchRfx(msg);
      return;
    case 'egfxDeleteContext':
      dispatchDeleteContext(msg.surfaceId, msg.ctxId);
      return;
    case 'egfxEndFrame':
      noteEndFrame();
      return;
    case 'egfxPresented':
      notePresented();
      return;
    case 'log':
    case 'decoded':
    case 'resolution':
    case 'cursorBitmap':
    case 'cursorHidden':
    case 'cursorDefault':
      post(msg);
      return;
  }
}

function* walkAnnexBNals(
  data: Uint8Array
): Generator<{ offset: number; length: number }> {
  const len = data.length;
  let i = 0;
  while (
    i + 3 < len &&
    !(data[i] === 0 && data[i + 1] === 0 && data[i + 2] === 1) &&
    !(
      i + 4 <= len &&
      data[i] === 0 &&
      data[i + 1] === 0 &&
      data[i + 2] === 0 &&
      data[i + 3] === 1
    )
  ) {
    i++;
  }
  while (i + 3 <= len) {
    let start: number;
    if (
      i + 4 <= len &&
      data[i] === 0 &&
      data[i + 1] === 0 &&
      data[i + 2] === 0 &&
      data[i + 3] === 1
    ) {
      start = i + 4;
    } else if (data[i] === 0 && data[i + 1] === 0 && data[i + 2] === 1) {
      start = i + 3;
    } else {
      i++;
      continue;
    }
    let end = len;
    for (let j = start; j + 2 < len; j++) {
      if (data[j] === 0 && data[j + 1] === 0 && data[j + 2] === 1) {
        end = j > 0 && data[j - 1] === 0 ? j - 1 : j;
        break;
      }
    }
    yield { offset: start, length: end - start };
    i = end;
  }
}

function codecStringFromAvc(data: Uint8Array): string | null {
  for (const { offset, length } of walkAnnexBNals(data)) {
    if (length < 4) continue;
    if ((data[offset] & 0x1f) === 7) {
      const profile = data[offset + 1];
      const constraints = data[offset + 2];
      const level = data[offset + 3];
      const hex = (n: number) =>
        n.toString(16).padStart(2, '0').toUpperCase();
      return `avc1.${hex(profile)}${hex(constraints)}${hex(level)}`;
    }
  }
  return null;
}

function avcContainsIdr(data: Uint8Array): boolean {
  for (const { offset, length } of walkAnnexBNals(data)) {
    if (length >= 1 && (data[offset] & 0x1f) === 5) return true;
  }
  return false;
}

function handleAvcChunk(m: Extract<DecoderEvent, { type: 'avcChunk' }>) {
  // AVC444 increment 1: decode + probe the chroma auxiliary stream alongside the
  // main view (additive, off the display path). Must run BEFORE the luma-only
  // early return so CHROMA-only (enc=2) frames still reach the chroma decoder.
  feedChroma(m);

  // The MAIN/luma display path: a CHROMA-only frame has no luma to show.
  if (m.encoding === 2 || m.luma.length === 0) return;

  let entry = avcDecoders.get(m.surface);
  if (!entry) {
    const pending: AvcDecoderEntry['pending'] = [];
    // DIAGNOSTIC (revert): stable per-surface captures for the closures —
    // do NOT reference the reassigned `entry`/`m` from here, they change per
    // call and would attribute counters to the wrong surface.
    const surface = m.surface;
    // Stable captures for the async output/copyTo closures (m is reassigned per
    // call). codecId/encoding are the values at decoder-creation (first frame) —
    // representative for the one-shot [avc-green] diagnostic below.
    const codecId = m.codecId;
    const encoding = m.encoding;
    const counters = { decodes: 0, outputs: 0 };
    const rgbaPool: Uint8Array[] = [];
    const vd = new VideoDecoder({
      output: frame => {
        counters.outputs++;
        const paint = pending.shift();
        if (!paint) {
          frame.close();
          return;
        }
        // Use the VISIBLE region, NOT codedWidth/codedHeight. H.264 pads coded
        // dimensions up to 16-px macroblock multiples; the padding columns/rows
        // carry chroma U=V=0, which YUV→RGB renders as saturated green. Cropping
        // copyTo to coded dims (the old bug) sampled that padding → green bands on
        // the right/bottom of each AVC rect ("moving green chunks"). visibleRect
        // is the real picture; displayWidth/Height is the fallback.
        const visX = frame.visibleRect?.x ?? 0;
        const visY = frame.visibleRect?.y ?? 0;
        const frameW = frame.visibleRect?.width ?? frame.displayWidth;
        const frameH = frame.visibleRect?.height ?? frame.displayHeight;
        if (frameW === 0 || frameH === 0) {
          frame.close();
          return;
        }
        // Fast path: hand the decoded frame straight to the GPU texture — no
        // copyTo readback, no CPU copies (the GPU also does YUV→RGB). Falls back
        // to the CPU copyTo path below if wasm can't place it (video spans a
        // monitor edge, or no framebuffer yet). The GPU path already uploads the
        // VISIBLE region (display_width/height), so it doesn't have the
        // macroblock-padding green bug the CPU crop had — both paths are now
        // correct.
        if (decoder) {
          let handled = false;
          try {
            handled = decoder.blitAvcFrame(paint.desktopX, paint.desktopY, frame);
          } catch (e) {
            // eslint-disable-next-line no-console
            console.warn('[codec-test-worker] blitAvcFrame failed', e);
          }
          if (handled) {
            perfProbe.gpu++;
            frame.close();
            return;
          }
        }
        // The decoded H.264 frame is the FULL SURFACE (codedW/H = surface size,
        // e.g. 3200x976), NOT the update region. The changed region lives at its
        // surface-relative position WITHIN that frame, so the crop ORIGIN must be
        // there — cropping (0,0) read the wrong (often uninitialized → green)
        // corner and placed it over the region. For the single full-desktop
        // surface the origin is (0,0), so the region's frame offset == its desktop
        // position (desktopX/Y). (Multi-surface needs the surface origin threaded
        // through from Rust — a follow-up.)
        const regionX = visX + paint.desktopX;
        const regionY = visY + paint.desktopY;
        const cropW = Math.max(0, Math.min(paint.destWidth, frameW - paint.desktopX));
        const cropH = Math.max(0, Math.min(paint.destHeight, frameH - paint.desktopY));
        if (cropW === 0 || cropH === 0) {
          frame.close();
          return;
        }
        perfProbe.cpu++;
        // Pool the copyTo buffer (full-screen AVC is ~8 MB/frame). copyTo is
        // async and frames can overlap, so each in-flight copy gets its own
        // backing buffer from the pool, returned in `finally`.
        const need = cropW * cropH * 4;
        let pooled = rgbaPool.pop();
        if (!pooled || pooled.byteLength < need) {
          pooled = new Uint8Array(need);
        }
        const backing = pooled;
        // copyTo writes `need` bytes; blitAvcRgba uses the view's length, so
        // pass an exact-length view of the (possibly larger) backing buffer.
        const rgba =
          backing.byteLength === need ? backing : backing.subarray(0, need);
        const opts: VideoFrameCopyToOptions = {
          format: 'RGBA',
          // Crop the region at its position WITHIN the full-surface frame
          // (regionX/Y), not (0,0). WebCodecs interprets `rect` in the frame's
          // coded coordinate space.
          rect: { x: regionX, y: regionY, width: cropW, height: cropH },
        };
        frame
          .copyTo(rgba, opts)
          .then(() => {
            // DIAGNOSTIC (green-rects): one-shot per surface — proves whether the
            // crop was reaching H.264 macroblock padding (coded > visible) and
            // samples the right-edge pixel (green ≈ 0,~135,0 ⇒ padding was hit).
            // After the visible-crop fix the edge should be normal content.
            if (counters.outputs <= 3) {
              const vr = frame.visibleRect;
              // eslint-disable-next-line no-console
              console.log(
                `[avc-green] surface=${surface} codecId=${codecId} enc=${encoding} ` +
                  `coded=${frame.codedWidth}x${frame.codedHeight} ` +
                  `visible=${vr ? `${vr.width}x${vr.height}@${vr.x},${vr.y}` : 'null'} ` +
                  `display=${frame.displayWidth}x${frame.displayHeight} ` +
                  `destXY=${paint.desktopX},${paint.desktopY} destWH=${paint.destWidth}x${paint.destHeight} ` +
                  `cropOrigin=${regionX},${regionY} cropWH=${cropW}x${cropH} ` +
                  `format=${frame.format} colorSpace=${JSON.stringify(frame.colorSpace)}`
              );
              // Sample inward from each edge to size the green border and see
              // which edges it's on (uniform thin ring ⇒ chroma bleed; only
              // right/bottom ⇒ per-region MB padding).
              const px = (x: number, y: number) => {
                const i = (y * cropW + x) * 4;
                return `${rgba[i]},${rgba[i + 1]},${rgba[i + 2]}`;
              };
              const cx = cropW >> 1;
              const cy = cropH >> 1;
              // eslint-disable-next-line no-console
              console.log(
                `[avc-green] L@midY x=0,2,4,8: ${px(0, cy)} ${px(2, cy)} ${px(4, cy)} ${px(8, cy)} | ` +
                  `R x=W-1,W-3,W-5: ${px(cropW - 1, cy)} ${px(cropW - 3, cy)} ${px(cropW - 5, cy)} | ` +
                  `T@midX y=0,2,4: ${px(cx, 0)} ${px(cx, 2)} ${px(cx, 4)} | ` +
                  `B y=H-1,H-3: ${px(cx, cropH - 1)} ${px(cx, cropH - 3)} | C: ${px(cx, cy)}`
              );
            }
            // DIAGNOSTIC (revert): muted — tagged AVC/H.264-painted regions with
            // a red top-left corner so AVC rects were obvious on-screen.
            // Un-comment to re-enable.
            /*
            {
              const n = Math.min(8, cropW);
              const m = Math.min(8, cropH);
              for (let yy = 0; yy < m; yy++) {
                for (let xx = 0; xx < n; xx++) {
                  const i = (yy * cropW + xx) * 4;
                  rgba[i] = 0xff;
                  rgba[i + 1] = 0x00;
                  rgba[i + 2] = 0x00;
                  rgba[i + 3] = 0xff;
                }
              }
            }
            */
            // Kill the green chroma-bleed border: the H.264 frame is green
            // (U=V=0) outside the video region, and 4:2:0 chroma upsampling
            // bleeds it ~1px into every edge (~3px on the bottom). Replicate the
            // nearest interior pixel outward over a BLEED_PX ring so the edge
            // shows the (current) video edge instead of green. Done in-place on
            // the RGBA buffer before blit; keeps the region full-size (no stale
            // ring an inset would leave). BLEED_PX covers the observed worst case
            // (bottom); raise if any green survives.
            const BLEED_PX = 4;
            if (cropW > 2 * BLEED_PX && cropH > 2 * BLEED_PX) {
              const rowBytes = cropW * 4;
              // Top & bottom: copy the first/last interior row over the ring.
              for (let y = 0; y < BLEED_PX; y++) {
                rgba.copyWithin(
                  y * rowBytes,
                  BLEED_PX * rowBytes,
                  (BLEED_PX + 1) * rowBytes
                );
                const dstRow = cropH - 1 - y;
                const srcRow = cropH - 1 - BLEED_PX;
                rgba.copyWithin(
                  dstRow * rowBytes,
                  srcRow * rowBytes,
                  (srcRow + 1) * rowBytes
                );
              }
              // Left & right: copy the first/last interior column per row.
              for (let y = 0; y < cropH; y++) {
                const row = y * rowBytes;
                const lsrc = row + BLEED_PX * 4;
                const rsrc = row + (cropW - 1 - BLEED_PX) * 4;
                for (let x = 0; x < BLEED_PX; x++) {
                  rgba.copyWithin(row + x * 4, lsrc, lsrc + 4);
                  rgba.copyWithin(row + (cropW - 1 - x) * 4, rsrc, rsrc + 4);
                }
              }
            }
            // Hand the decoded RGBA to wasm fb, which blits + renders into
            // the registered OffscreenCanvas. This is the same path used by
            // EgfxBitmap and fast-path PDUs, so all three converge on one
            // visible canvas with no further round-trip to main.
            if (decoder) {
              decoder.blitAvcRgba(
                paint.desktopX,
                paint.desktopY,
                cropW,
                cropH,
                rgba
              );
            }
          })
          .catch(e => {
            // eslint-disable-next-line no-console
            console.warn(`[codec-test-worker] copyTo failed:`, e);
          })
          .finally(() => {
            frame.close();
            // Return the backing buffer to the pool (capped to bound growth).
            if (rgbaPool.length < 4) rgbaPool.push(backing);
          });
      },
      error: e => {
        // eslint-disable-next-line no-console
        console.warn(
          `[avc-probe] decoder error surface=${surface} decodes=${counters.decodes} outputs=${counters.outputs} pending=${pending.length}:`,
          e
        );
      },
    });
    entry = {
      decoder: vd,
      configured: false,
      loggedInit: false,
      counters,
      pending,
      rgbaPool,
    };
    avcDecoders.set(m.surface, entry);
    // DIAGNOSTIC (revert): muted.
    // console.log(`[codec-test-worker] created decoder surface=${m.surface}`);
  }

  // One-time per-surface AVC probe: confirms AVC/H.264 is actually driving this
  // content (vs RFX/bitmap) and reports the NAL format. (Temporarily un-muted
  // while diagnosing video performance.)
  if (!entry.loggedInit) {
    entry.loggedInit = true;
    const head = Array.from(m.luma.slice(0, 16))
      .map(b => b.toString(16).padStart(2, '0'))
      .join(' ');
    const annexB =
      m.luma.length >= 4 &&
      m.luma[0] === 0 &&
      m.luma[1] === 0 &&
      (m.luma[2] === 1 || (m.luma[2] === 0 && m.luma[3] === 1));
    // eslint-disable-next-line no-console
    console.log(
      `[avc-probe] surface=${m.surface} codecId=${m.codecId} enc=${m.encoding} ` +
        `lumaLen=${m.luma.length} annexB=${annexB} head16=[${head}]`
    );
  }

  if (!entry.configured) {
    const codecString = codecStringFromAvc(m.luma);
    if (!codecString) {
      const head = Array.from(m.luma.slice(0, 24))
        .map(b => b.toString(16).padStart(2, '0'))
        .join(' ');
      // eslint-disable-next-line no-console
      console.warn(
        `[codec-test-worker] surface=${m.surface} no SPS NAL; first 24 bytes: ${head} (len=${m.luma.length})`
      );
      return;
    }
    try {
      entry.decoder.configure({
        codec: codecString,
        // Prefer GPU decode and emit frames with minimal reorder buffering —
        // this is a live remote-desktop stream, not a seekable file.
        hardwareAcceleration: 'prefer-hardware',
        optimizeForLatency: true,
      });
      entry.configured = true;
      // DIAGNOSTIC (revert): muted.
      // console.log(
      //   `[codec-test-worker] surface=${m.surface} configured codec=${codecString}`
      // );
    } catch (e) {
      // eslint-disable-next-line no-console
      console.warn(`[codec-test-worker] configure failed:`, e);
      return;
    }
  }

  const chunkType: EncodedVideoChunkType = avcContainsIdr(m.luma)
    ? 'key'
    : 'delta';
  let chunk: EncodedVideoChunk;
  try {
    chunk = new EncodedVideoChunk({
      type: chunkType,
      timestamp: performance.now() * 1000,
      data: m.luma,
    });
  } catch (e) {
    // eslint-disable-next-line no-console
    console.warn(`[codec-test-worker] EncodedVideoChunk new failed:`, e);
    return;
  }

  entry.pending.push({
    desktopX: m.desktopX,
    desktopY: m.desktopY,
    destWidth: m.destWidth,
    destHeight: m.destHeight,
  });
  entry.counters.decodes++;
  try {
    entry.decoder.decode(chunk);
  } catch (e) {
    entry.pending.pop();
    entry.counters.decodes--;
    // eslint-disable-next-line no-console
    console.warn(`[codec-test-worker] decode failed:`, e);
  }
  // DIAGNOSTIC (revert): muted heartbeat — a widening decodes-vs-outputs gap or
  // a growing pending queue means the per-surface FIFO is desyncing.
  // if (entry.counters.decodes % 120 === 0) {
  //   console.log(
  //     `[avc-probe] surface=${m.surface} decodes=${entry.counters.decodes} outputs=${entry.counters.outputs} pending=${entry.pending.length}`
  //   );
  // }
}

function openWebSocket(url: string, bearerToken: string) {
  state = 'authing';
  post({ type: 'state', state });

  const socket = new WebSocket(url);
  socket.binaryType = 'arraybuffer';
  ws = socket;
  let authed = false;

  socket.addEventListener('open', () => {
    socket.send(JSON.stringify({ token: bearerToken }));
  });

  socket.addEventListener('message', ev => {
    if (!authed) {
      if (typeof ev.data !== 'string') {
        setError('expected text auth response, got binary');
        return;
      }
      let parsed: { type?: string; status?: string; message?: string };
      try {
        parsed = JSON.parse(ev.data);
      } catch (e) {
        setError(`parse auth response failed: ${e}`);
        return;
      }
      if (parsed.type !== 'create_session_response') {
        setError(`unexpected auth response type: ${parsed.type}`);
        return;
      }
      if (parsed.status !== 'ok') {
        setError(`auth error: ${parsed.message ?? 'unknown'}`);
        return;
      }
      authed = true;
      state = 'open';
      post({ type: 'state', state });
      return;
    }

    if (typeof ev.data === 'string') return;
    if (!decoder) return;
    decoder.feedBytes(ev.data as ArrayBuffer);
  });

  socket.addEventListener('error', () => setError('websocket error'));
  socket.addEventListener('close', () => {
    state = 'closed';
    ws = null;
    post({ type: 'state', state });
  });
}

function setError(message: string) {
  state = 'error';
  lastErrorMessage = message;
  post({ type: 'state', state, message });
  closeSession();
}

function closeSession() {
  if (ws) {
    try {
      ws.close();
    } catch {
      /* ignore */
    }
    ws = null;
  }
  decoder = null;
  responseCodec = null;
  resetEgfxOffloadState();
  for (const entry of avcDecoders.values()) {
    try {
      entry.decoder.close();
    } catch {
      /* ignore */
    }
  }
  avcDecoders.clear();
  if (state !== 'error') {
    state = 'closed';
    lastErrorMessage = undefined;
  }
}

function post(msg: WorkerToClient, transfer?: Transferable[]) {
  try {
    if (transfer && transfer.length > 0) {
      self.postMessage(msg, transfer);
    } else {
      self.postMessage(msg);
    }
  } catch {
    /* main gone */
  }
}

// Surface lastErrorMessage so the type-checker doesn't flag it as unused — it
// is set during error transitions and carried in the next 'state' message.
void lastErrorMessage;

export {};
