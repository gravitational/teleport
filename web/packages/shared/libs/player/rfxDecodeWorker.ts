/**
 * Step 2 dedicated RFX/ClearCodec decode worker.
 *
 * A child DedicatedWorker spawned by `codecTestWorker.ts`. It holds ONE
 * `EgfxDecoder` (the stateful RFX-Progressive + ClearCodec decoders, moved off
 * the main worker's `Framebuffer`) and does nothing but decode: it receives
 * encoded EGFX PDUs, decodes them in arrival (= wire) order, and posts the
 * decoded pixels back — RFX as the self-describing binary tile blob, ClearCodec
 * as a raw BGRA buffer — transferring the result buffer zero-copy. The main
 * worker keeps the WebSocket, input relay, framebuffer image, and GL; moving
 * decode here frees that thread so input no longer queues behind decode.
 *
 * Ordering: this worker processes its message queue FIFO and returns FIFO, so
 * the per-surface progressive difference/upgrade chains and the ClearCodec
 * caches stay correctly ordered (both decoders are prior-frame-relative). Each
 * message carries a monotonic `seq` echoed back so the main worker can run its
 * deferred-present barrier (see RFX_POOL_STEP2_PLAN.md).
 *
 * No SharedArrayBuffer: results cross the worker boundary via postMessage
 * transfer; the wasm here is a fresh instance with its own linear memory.
 */

import init, { EgfxDecoder, initWasmLog, setSimdIdwt } from './pkg/session';
import { installWorkerLogSink } from './logsink';

installWorkerLogSink();

/** Messages from the parent (codecTestWorker) to this decode worker. */
type DecodeRequest =
  | {
      type: 'rfx';
      seq: number;
      surfaceId: number;
      ctxId: number;
      // This worker's index in the pool + the pool size. The PDU is broadcast to
      // every worker; each decodes only the tiles it owns (stable position hash).
      workerIndex: number;
      numWorkers: number;
      surfaceWidth: number;
      surfaceHeight: number;
      payload: ArrayBuffer;
    }
  | { type: 'delete'; surfaceId: number; ctxId: number }
  | { type: 'set-simd-idwt'; on: boolean }
  | { type: 'reset-perf' };

const scope = self as unknown as {
  postMessage(msg: unknown, transfer?: Transferable[]): void;
  addEventListener(
    type: 'message',
    listener: (e: MessageEvent<DecodeRequest>) => void
  ): void;
};

let dec: EgfxDecoder | null = null;
let ready = false;
// [perf-probe] decode counter for the periodic stage-timing flush below.
let decodeCount = 0;
// Buffer requests that arrive before the wasm finishes initializing, so the
// decoder still sees every PDU in order from the very first one (the parent
// enables offload immediately, before this worker's async init completes).
const queue: DecodeRequest[] = [];

function handle(msg: DecodeRequest) {
  if (!dec) return;
  try {
    switch (msg.type) {
      case 'rfx': {
        // Hands this chunk's surface dims to the decoder. A dim change resets
        // only THAT surface's decoder — multi-monitor sessions interleave
        // chunks from differently-sized surfaces constantly, and they must
        // not clear each other's progressive accumulators.
        dec.setSurface(msg.surfaceWidth, msg.surfaceHeight);
        const blob = dec.decodeWireToSurface2(
          msg.surfaceId,
          msg.ctxId,
          msg.workerIndex,
          msg.numWorkers,
          new Uint8Array(msg.payload)
        );
        scope.postMessage({ type: 'rfxDone', seq: msg.seq, blob }, [
          blob.buffer as ArrayBuffer,
        ]);
        // [perf-probe] Every ~60 decodes, surface this worker's accumulated RFX
        // per-stage time so the host's L.perf() can show the OFFLOAD-path
        // entropy:IDWT:color split (a 4-number postMessage, not per-tile).
        if (++decodeCount % 60 === 0) {
          const [entropyMs, idwtMs, colorMs, tiles] = dec.takeRfxStageTimings();
          scope.postMessage({ type: 'rfxStats', entropyMs, idwtMs, colorMs, tiles });
        }
        break;
      }
      case 'delete':
        dec.deleteContext(msg.surfaceId, msg.ctxId);
        break;
      case 'set-simd-idwt':
        setSimdIdwt(msg.on);
        break;
      case 'reset-perf':
        // Reset this worker's accumulated RFX stage-timing counters by triggering
        // a takeRfxStageTimings drain (discarding the result).
        dec.takeRfxStageTimings();
        decodeCount = 0;
        break;
    }
  } catch (e) {
    // On a decode error still echo the seq (no pixels) so the parent always
    // counts every expected reply (`rfxSeqReady` fires) and the wasm wire-order
    // queue never wedges waiting on this frame.
    if (msg.type === 'rfx') {
      scope.postMessage({ type: 'rfxDone', seq: msg.seq });
      // Loud on purpose: a blobless reply means this worker's tile partition
      // is silently MISSING from the frame — the on-screen symptom is a rect
      // that never paints, so make the cause findable from the console.
      // eslint-disable-next-line no-console
      console.error(
        `[rfx-decode-worker] rfx decode failed — tiles dropped for seq=${msg.seq} ` +
          `surface=${msg.surfaceId} ${msg.surfaceWidth}x${msg.surfaceHeight} ` +
          `worker=${msg.workerIndex}/${msg.numWorkers}`,
        e
      );
    } else {
      // eslint-disable-next-line no-console
      console.warn('[rfx-decode-worker] decode failed', msg.type, e);
    }
  }
}

scope.addEventListener('message', e => {
  if (!ready) {
    queue.push(e.data);
    return;
  }
  handle(e.data);
});

void init()
  .then(() => initWasmLog('debug'))
  .then(() => {
    dec = new EgfxDecoder();
    ready = true;
    // Drain anything that arrived during init, in order.
    const buffered = queue.splice(0);
    for (const m of buffered) handle(m);
    scope.postMessage({ type: 'ready' });
  })
  .catch(e => {
    scope.postMessage({ type: 'fatal', error: String(e) });
  });
