/**
 * SharedWorker that owns the WebSocket, a single IronRDP `WorkerDecoder`,
 * and the per-window canvas registrations. One decoder paints into N
 * OffscreenCanvases — main + each popup — so adding/removing a monitor is
 * just registering or dropping a canvas with its viewport.
 *
 * Lifecycle:
 *  1. First port connects and sends `host` with wsUrl + bearer token. The
 *     worker initializes wasm, opens the WS, completes Teleport
 *     `create_session_response` auth, then broadcasts state=open.
 *  2. Later ports connect, see state=open, and register their canvases via
 *     `register-canvas`. The canvas is transferred over MessagePort.
 *  3. WS binary frames are fed directly into the decoder. The decoder's
 *     callback dispatches outbound events: `rdpResponse` is wrapped with
 *     `MainCodec.encodeRdpResponse` and sent on the WS; cursor / log /
 *     decoded / resolution events are broadcast to all ports; `tdpbUpgrade`
 *     flips the local codec mode and is broadcast so each port's own
 *     `MainCodec` (used for mouse/keyboard encode on main) can upgrade too.
 *  4. Ports send outbound (mouse/keyboard) bytes already encoded — the
 *     worker just calls `ws.send`.
 *  5. When the last port disconnects, the WS closes and decoder state is
 *     discarded so the next session starts fresh.
 */

import init, {
  initWasmLog,
  MainCodec,
  WorkerDecoder,
} from './pkg/session';

// eslint-disable-next-line no-console
console.log(
  '[shared-session-worker] module loaded at',
  new Date().toISOString(),
  '— if you can see this in the SharedWorker DevTools console, the inspector is attached to the right context.'
);
// eslint-disable-next-line no-console
console.warn(
  '[shared-session-worker] (warn-level test message — DevTools level filter check)'
);

type State = 'closed' | 'authing' | 'open' | 'error';

interface HostMessage {
  type: 'host';
  wsUrl: string;
  bearerToken: string;
  /**
   * How many windows (= canvases) will participate in this session. The
   * SharedWorker holds canvas registrations until this many have arrived,
   * then computes a layout and emits `layout-ready` to the host port. This
   * lets every monitor's reported width/height match its actual window's
   * visible CSS dimensions instead of the picker-supplied physical size.
   */
  expectedMonitorCount: number;
}

interface OutboundMessage {
  type: 'outbound';
  buffer: ArrayBuffer;
}

interface UnregisterMessage {
  type: 'unregister';
}

interface RegisterCanvasMessage {
  type: 'register-canvas';
  canvasId: number;
  /**
   * Index of this canvas in the monitor layout. The worker stacks canvases
   * horizontally in increasing index order to compute the virtual-desktop
   * layout once all expected canvases have registered.
   */
  monitorIndex: number;
  canvas: OffscreenCanvas;
  /** Actual size of this canvas's window (window.innerWidth/innerHeight). */
  width: number;
  height: number;
}

interface UpdateViewportMessage {
  type: 'update-viewport';
  canvasId: number;
  canvas: OffscreenCanvas;
  viewport: { x: number; y: number; width: number; height: number };
}

interface UnregisterCanvasMessage {
  type: 'unregister-canvas';
  canvasId: number;
}

interface AvcPaintMessage {
  type: 'avcPaint';
  desktopX: number;
  desktopY: number;
  width: number;
  height: number;
  rgba: Uint8Array;
}

type ClientToWorker =
  | HostMessage
  | OutboundMessage
  | UnregisterMessage
  | RegisterCanvasMessage
  | UpdateViewportMessage
  | UnregisterCanvasMessage
  | AvcPaintMessage;

interface StateMessage {
  type: 'state';
  state: State;
  message?: string;
}

// Decoder-produced events that get fanned out to all ports. Field shapes
// mirror what the Rust callback emits via Reflect::set on the message
// object; main-thread consumers (DesktopSessionTestMulti.tsx) read the
// same field names.
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
      // EGFX AVC444/v2 H.264 chunk forwarded from the wasm decoder for
      // browser-side decode. The wasm has no `VideoDecoder` (the API is
      // not exposed in the SharedWorker scope on current Chrome stable),
      // so each main-thread port runs its own decoder and posts back a
      // `paint` message with the resulting RGBA.
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
    };

/**
 * Sent to the host port once all expected canvases have registered. Carries
 * the computed per-monitor layout so main can encode a ScreenSpec at the
 * actual window dimensions (no squish from picker-vs-visible mismatch).
 */
interface LayoutReadyMessage {
  type: 'layout-ready';
  monitors: Array<{
    monitorIndex: number;
    x: number;
    y: number;
    width: number;
    height: number;
  }>;
  bboxWidth: number;
  bboxHeight: number;
}

type WorkerToClient = StateMessage | DecoderEvent | LayoutReadyMessage;

const ports = new Set<MessagePort>();
let ws: WebSocket | null = null;
let state: State = 'closed';
let lastErrorMessage: string | undefined;

let wasmReady: Promise<void> | null = null;
let decoder: WorkerDecoder | null = null;
let responseCodec: MainCodec | null = null;
// Tracks whether the inbound stream has switched to TDPB. We need this so a
// port that joins mid-session (e.g. a popup) can replay the upgrade and put
// its local MainCodec in the right mode before sending mouse/keyboard
// events. Without this the popup would ship TDP-framed bytes into a TDPB
// server, the server rejects, and the WS disconnects.
let tdpbUpgraded = false;

// Cache of the most recent SPS-containing AVC chunk per surface. Late-joining
// popups subscribe to the broadcast stream of `avcChunk` events but miss the
// initial IDR/SPS that arrived before they registered, so their VideoDecoder
// can never configure. On port join we re-emit each cached chunk to that port
// so its decoder gets an SPS to configure with + an IDR to start decoding.
const cachedSpsChunks = new Map<number, AvcChunkBroadcast>();
type AvcChunkBroadcast = Extract<DecoderEvent, { type: 'avcChunk' }>;

// How many canvases (windows) we expect for this session, set when the host
// sends its 'host' message. We hold canvas registrations until all expected
// ones arrive so we can compute a per-monitor layout from actual window
// sizes (which is the thing that prevents server-side squish).
let expectedMonitorCount: number | null = null;
// Host port — the one that opened the WS, so we know where to send
// `layout-ready`. Popups don't need it; only main encodes ScreenSpec.
let hostPort: MessagePort | null = null;

interface PendingCanvas {
  id: number;
  monitorIndex: number;
  canvas: OffscreenCanvas;
  width: number;
  height: number;
  port: MessagePort;
}
// Canvas registrations waiting for either the decoder to come up or the rest
// of the expected windows to arrive. Drained by `maybeFlushPendingLayout`.
const pendingCanvases = new Map<number, PendingCanvas>();

// Track which port owns which canvas ids so a port disconnect can clean up
// its registrations.
const portCanvases = new WeakMap<MessagePort, Set<number>>();

declare const self: {
  addEventListener(
    type: 'connect',
    listener: (event: MessageEvent) => void
  ): void;
};

self.addEventListener('connect', (event: MessageEvent) => {
  const port = event.ports[0];
  ports.add(port);
  portCanvases.set(port, new Set());
  port.start();

  port.addEventListener('message', (e: MessageEvent<ClientToWorker>) => {
    handleClientMessage(port, e.data);
  });

  port.addEventListener('messageerror', () => disconnectPort(port));

  send(port, { type: 'state', state, message: lastErrorMessage });

  // If TDPB has already been negotiated before this port joined, replay the
  // upgrade so the port's local codec flips to TDPB before it sends anything.
  if (tdpbUpgraded) {
    send(port, { type: 'tdpbUpgrade' });
  }

  // Replay cached SPS-containing AVC chunks so a late-joining port's
  // VideoDecoder can configure and start producing frames. Without this,
  // popups that connect after the initial IDR would only see P-frames and
  // never paint anything.
  for (const chunk of cachedSpsChunks.values()) {
    send(port, chunk);
  }
});

function handleClientMessage(port: MessagePort, msg: ClientToWorker) {
  switch (msg.type) {
    case 'host':
      if (state === 'closed') {
        hostPort = port;
        expectedMonitorCount = msg.expectedMonitorCount;
        void start(msg.wsUrl, msg.bearerToken);
      }
      break;
    case 'outbound':
      if (ws && state === 'open') {
        ws.send(msg.buffer);
      }
      break;
    case 'unregister':
      disconnectPort(port);
      break;
    case 'register-canvas':
      registerCanvas(port, msg);
      break;
    case 'update-viewport':
      // No-op for now — layout positions are computed by the SharedWorker
      // from reported sizes. Live resize support can re-use this hook.
      break;
    case 'unregister-canvas':
      unregisterCanvas(port, msg.canvasId);
      break;
    case 'avcPaint':
      // RGBA decoded by a main-thread VideoDecoder for an AVC444/v2 frame
      // we forwarded via `avcChunk`. Multiple ports may both decode the
      // same frames; blitting is idempotent so we just accept all calls.
      if (decoder) {
        decoder.blitAvcRgba(
          msg.desktopX,
          msg.desktopY,
          msg.width,
          msg.height,
          msg.rgba
        );
      }
      break;
  }
}

function registerCanvas(port: MessagePort, msg: RegisterCanvasMessage) {
  pendingCanvases.set(msg.canvasId, {
    id: msg.canvasId,
    monitorIndex: msg.monitorIndex,
    canvas: msg.canvas,
    width: msg.width,
    height: msg.height,
    port,
  });
  portCanvases.get(port)?.add(msg.canvasId);
  maybeFlushPendingLayout();
}

function unregisterCanvas(port: MessagePort, canvasId: number) {
  pendingCanvases.delete(canvasId);
  portCanvases.get(port)?.delete(canvasId);
  if (decoder) {
    decoder.removeCanvas(canvasId);
  }
}

/**
 * If the decoder is up and every expected canvas has reported in, compute
 * the per-monitor layout (stacked horizontally in monitorIndex order),
 * register each canvas with the decoder at its computed image-space
 * position, and tell the host port so it can encode a ScreenSpec at the
 * actual window sizes.
 */
function maybeFlushPendingLayout() {
  if (!decoder) return;
  if (expectedMonitorCount === null) return;
  if (pendingCanvases.size < expectedMonitorCount) return;

  const sorted = Array.from(pendingCanvases.values()).sort(
    (a, b) => a.monitorIndex - b.monitorIndex
  );
  // Each monitor reports its own window-actual size. Width and height are
  // rounded down to even because MS-RDPEDISP rejects odd dimensions on
  // some Windows builds.
  let cursorX = 0;
  let bboxHeight = 0;
  const layout: LayoutReadyMessage['monitors'] = [];
  for (const pc of sorted) {
    const w = Math.max(2, pc.width & ~1);
    const h = Math.max(2, pc.height & ~1);
    const x = cursorX;
    const y = 0;
    layout.push({
      monitorIndex: pc.monitorIndex,
      x,
      y,
      width: w,
      height: h,
    });
    try {
      decoder.addCanvas(pc.id, pc.canvas, x, y, w, h);
    } catch (e) {
      // eslint-disable-next-line no-console
      console.warn('[shared-session-worker] addCanvas failed', e);
    }
    cursorX += w;
    if (h > bboxHeight) bboxHeight = h;
  }
  pendingCanvases.clear();

  // Notify ALL ports so each window can set its `layoutEntryRef` (needed
  // by popup windows to translate local mouse coords → virtual desktop
  // coords). The host port additionally uses this layout to encode the
  // ScreenSpec, but popups need their own entry too.
  const layoutMsg = {
    type: 'layout-ready' as const,
    monitors: layout,
    bboxWidth: cursorX,
    bboxHeight,
  };
  broadcast(layoutMsg);
}

function disconnectPort(port: MessagePort) {
  const owned = portCanvases.get(port);
  if (owned && decoder) {
    for (const id of owned) {
      decoder.removeCanvas(id);
    }
  }
  portCanvases.delete(port);
  ports.delete(port);
  try {
    port.close();
  } catch {
    /* ignore */
  }
  if (ports.size === 0) {
    closeSession();
  }
}

async function start(url: string, bearerToken: string) {
  // eslint-disable-next-line no-console
  console.log('[shared-session-worker] start() invoked, url=', url);
  if (!wasmReady) {
    wasmReady = init().then(() => initWasmLog('debug'));
  }
  try {
    await wasmReady;
  } catch (e) {
    setError(`wasm init failed: ${e}`);
    return;
  }
  // eslint-disable-next-line no-console
  console.log('[shared-session-worker] wasm + initWasmLog ready; opening WS');

  responseCodec = new MainCodec();
  decoder = new WorkerDecoder(handleDecoderEvent);
  maybeFlushPendingLayout();

  openWebSocket(url, bearerToken);
}

function handleDecoderEvent(msg: DecoderEvent | { type: 'rdpResponse'; buffer: ArrayBuffer }) {
  switch (msg.type) {
    case 'rdpResponse': {
      if (!ws || state !== 'open' || !responseCodec) return;
      try {
        const wrapped = responseCodec.encodeRdpResponse(new Uint8Array(msg.buffer));
        // Cast: ArrayBufferLike covers SharedArrayBuffer which we never use
        // (no COOP/COEP), so wrapped.buffer is always a plain ArrayBuffer.
        ws.send(wrapped.buffer as ArrayBuffer);
      } catch (e) {
        // eslint-disable-next-line no-console
        console.warn('[shared-session-worker] encodeRdpResponse failed', e);
      }
      return;
    }
    case 'tdpbUpgrade':
      tdpbUpgraded = true;
      if (responseCodec) responseCodec.upgradeToTdpb();
      broadcast(msg);
      return;
    case 'log':
    case 'decoded':
    case 'resolution':
    case 'cursorBitmap':
    case 'cursorHidden':
    case 'cursorDefault':
    case 'avcChunk':
      if (avcChunkContainsSps(msg.luma)) {
        cachedSpsChunks.set(msg.surface, msg);
      }
      broadcast(msg);
      return;
  }
}

// True if the Annex-B H.264 buffer contains at least one SPS NAL (type 7).
function avcChunkContainsSps(data: Uint8Array): boolean {
  const len = data.length;
  let i = 0;
  while (i + 3 <= len) {
    if (
      i + 4 <= len &&
      data[i] === 0 &&
      data[i + 1] === 0 &&
      data[i + 2] === 0 &&
      data[i + 3] === 1
    ) {
      if (i + 4 < len && (data[i + 4] & 0x1f) === 7) return true;
      i += 4;
    } else if (data[i] === 0 && data[i + 1] === 0 && data[i + 2] === 1) {
      if (i + 3 < len && (data[i + 3] & 0x1f) === 7) return true;
      i += 3;
    } else {
      i++;
    }
  }
  return false;
}

function openWebSocket(url: string, bearerToken: string) {
  state = 'authing';
  broadcast({ type: 'state', state });

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
      broadcast({ type: 'state', state });
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
    broadcast({ type: 'state', state });
  });
}

function setError(message: string) {
  state = 'error';
  lastErrorMessage = message;
  broadcast({ type: 'state', state, message });
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
  pendingCanvases.clear();
  cachedSpsChunks.clear();
  expectedMonitorCount = null;
  hostPort = null;
  tdpbUpgraded = false;
  if (state !== 'error') {
    state = 'closed';
    lastErrorMessage = undefined;
  }
}

function broadcast(msg: WorkerToClient) {
  for (const port of ports) {
    send(port, msg);
  }
}

function send(port: MessagePort, msg: WorkerToClient) {
  try {
    port.postMessage(msg);
  } catch {
    /* port already closed; cleanup is in disconnectPort */
  }
}

export {};
