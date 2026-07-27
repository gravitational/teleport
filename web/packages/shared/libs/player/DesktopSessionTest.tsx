/**
 * Test harness for the post-A.2 architecture: main thread owns the
 * WebSocket and the outbound codec; the worker is a decode-only service
 * that paints to a transferred OffscreenCanvas.
 *
 * Render path:
 *   ws.onmessage(binary) → postMessage(worker, buffer, [buffer])
 *   worker decodes → posts back cursor / response / tdpbUpgrade / perf
 *
 * Input path:
 *   onMouseMove → MainCodec.encodeMouseMove → ws.send. No worker hop.
 */

import { useEffect, useMemo, useRef, useState } from 'react';

import {
  formatPerfLine,
  makeMainPerf,
  recordPostMessage,
  resetMainPerf,
  startRafSampler,
  stopRafSampler,
} from './perfLogger';
import init, {
  initWasmLog,
  MainCodec,
  type MainCodec as MainCodecType,
} from './pkg/session';
import {
  IncomingMessageType,
  OutgoingMessageType,
  type IncomingMessage,
  type OutgoingMessage,
} from './types';
import SessionWorker from './worker?worker';

/**
 * One monitor's position and size in the RDP virtual desktop coordinate space.
 * Coordinates are relative to the primary monitor's origin. The composite-mode
 * harness sizes its single canvas to the bounding box of all monitors and lets
 * the browser scale-down to fit; popup-per-monitor rendering is a follow-up.
 */
export interface MonitorSpec {
  x: number;
  y: number;
  width: number;
  height: number;
  isPrimary: boolean;
}

export interface DesktopSessionTestProps {
  /** Full `wss://…` URL to the desktop endpoint (placeholders already filled in). */
  wsUrl: string;
  /** Bearer token for Teleport's WS auth handshake. */
  bearerToken: string;
  username: string;
  width?: number;
  height?: number;
  scale?: number;
  keyboardLayout?: number;
  /**
   * Per-monitor layout. When provided with >1 entry the harness uses the
   * codec's `encodeScreenSpecMulti` / `encodeClientHelloMulti` methods so the
   * server negotiates a multi-monitor virtual desktop. width/height are
   * derived from the bounding box of these entries.
   */
  monitors?: MonitorSpec[];
}

type Status =
  | { kind: 'idle' }
  | { kind: 'connecting' }
  | { kind: 'open' }
  | { kind: 'error'; message: string };

interface Resolution {
  width: number;
  height: number;
}

/**
 * Same JSON handshake the Rust `auth.rs` used to do — kept in TS now
 * because main owns the socket. Mirrors `AuthenticatedWebSocket` in
 * `teleport/src/lib/`, but inline to avoid pulling teleport-only deps
 * into `shared/`.
 */
function openAuthedWebSocket(
  url: string,
  bearerToken: string
): Promise<WebSocket> {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(url);
    ws.binaryType = 'arraybuffer';
    let settled = false;
    const fail = (msg: string) => {
      if (settled) return;
      settled = true;
      try {
        ws.close();
      } catch {
        /* ignore */
      }
      reject(new Error(msg));
    };
    ws.addEventListener('open', () => {
      ws.send(JSON.stringify({ token: bearerToken }));
    });
    ws.addEventListener('message', ev => {
      if (settled) return;
      if (typeof ev.data !== 'string') {
        fail('expected text auth response, got binary');
        return;
      }
      let parsed: { type?: string; status?: string; message?: string };
      try {
        parsed = JSON.parse(ev.data);
      } catch (e) {
        fail(`parse auth response failed: ${e}`);
        return;
      }
      if (parsed.type !== 'create_session_response') {
        fail(`unexpected auth response type: ${parsed.type}`);
        return;
      }
      if (parsed.status === 'ok') {
        settled = true;
        resolve(ws);
      } else {
        fail(`auth error: ${parsed.message ?? 'unknown'}`);
      }
    });
    ws.addEventListener('error', () => fail('websocket error during auth'));
    ws.addEventListener('close', () => fail('websocket closed during auth'));
  });
}

let wasmReady: Promise<unknown> | null = null;

export function DesktopSessionTest({
  wsUrl,
  bearerToken,
  username,
  width = 1280,
  height = 720,
  scale = 100,
  keyboardLayout = 0,
  monitors,
}: DesktopSessionTestProps) {
  // When monitors are supplied, the virtual desktop spans their bounding box.
  // We override width/height with the bbox so the canvas + initial screen
  // spec describe the composite desktop.
  const bbox = monitors && monitors.length > 0 ? computeBoundingBox(monitors) : null;
  if (bbox) {
    width = bbox.width;
    height = bbox.height;
  }
  const multiMonitorPayload =
    monitors && monitors.length > 1 ? toCodecArrays(monitors) : null;
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const workerRef = useRef<Worker | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const codecRef = useRef<MainCodecType | null>(null);
  const mainPerfRef = useRef<ReturnType<typeof makeMainPerf> | null>(null);
  /// Where the most recent mousedown landed (server-side coords). Used to
  /// detect drag motion at mouseup and, if it exceeded the threshold,
  /// ask the proxy to send a RefreshRect — fixes the RFX decode trails
  /// the user sees as faint vertical lines after window drags.
  const dragOriginRef = useRef<{ x: number; y: number } | null>(null);
  const [status, setStatus] = useState<Status>({ kind: 'idle' });
  const resolutionRef = useRef<Resolution>({ width, height });
  const initialDimsRef = useRef<{
    width: number;
    height: number;
    scale: number;
    keyboardLayout: number;
    username: string;
  } | null>(null);

  const params = useMemo(
    () => ({ wsUrl, bearerToken, username, width, height, scale, keyboardLayout }),
    [wsUrl, bearerToken, username, width, height, scale, keyboardLayout]
  );

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    // When monitors are provided, the virtual desktop is fixed at the bbox
    // and CSS handles scaling-to-fit. Without monitors we keep the previous
    // behavior of using the CSS layout size as the initial resolution.
    const cssRect = canvas.getBoundingClientRect();
    const initialWidth = bbox
      ? bbox.width
      : Math.max(1, Math.round(cssRect.width)) || width;
    const initialHeight = bbox
      ? bbox.height
      : Math.max(1, Math.round(cssRect.height)) || height;
    canvas.width = initialWidth;
    canvas.height = initialHeight;
    resolutionRef.current = { width: initialWidth, height: initialHeight };
    initialDimsRef.current = {
      width: initialWidth,
      height: initialHeight,
      scale,
      keyboardLayout,
      username,
    };

    const mainPerf = makeMainPerf();
    mainPerfRef.current = mainPerf;
    startRafSampler(mainPerf);

    setStatus({ kind: 'connecting' });

    let cancelled = false;
    const cleanup: (() => void)[] = [];

    void (async () => {
      try {
        if (!wasmReady) {
          wasmReady = init().then(() => initWasmLog('debug'));
        }
        await wasmReady;
        if (cancelled) return;

        const codec = new MainCodec();
        codecRef.current = codec;

        // Spin up the worker first; we don't ws-send anything until the
        // worker is ready to receive PDU bytes.
        const w = new SessionWorker();
        workerRef.current = w;
        const offscreen = canvas.transferControlToOffscreen();
        const initMsg: IncomingMessage = {
          type: IncomingMessageType.Init,
          canvas: offscreen,
        };
        const ready = new Promise<void>(resolve => {
          const onReady = (ev: MessageEvent<OutgoingMessage>) => {
            if (ev.data.type === OutgoingMessageType.Ready) {
              w.removeEventListener('message', onReady);
              resolve();
            }
          };
          w.addEventListener('message', onReady);
        });
        w.postMessage(initMsg, [offscreen]);
        await ready;
        if (cancelled) return;

        // Wire the worker's normal message stream — the post-ready path.
        w.onmessage = (ev: MessageEvent<OutgoingMessage>) =>
          handleWorkerMessage(ev.data);

        // Authenticate (TS-side, was Rust auth.rs before A.2).
        const ws = await openAuthedWebSocket(wsUrl, bearerToken);
        if (cancelled) {
          ws.close();
          return;
        }
        wsRef.current = ws;

        // Send the initial ScreenSpec exactly the way the worker used to
        // do it — same wire bytes, different thread.
        const screenSpec = multiMonitorPayload
          ? codec.encodeScreenSpecMulti(
              initialWidth,
              initialHeight,
              scale,
              multiMonitorPayload.xs,
              multiMonitorPayload.ys,
              multiMonitorPayload.widths,
              multiMonitorPayload.heights,
              multiMonitorPayload.primaries
            )
          : codec.encodeScreenSpec(initialWidth, initialHeight, scale);
        ws.send(screenSpec);

        // From here on, every binary frame goes to the worker as a
        // Transferable ArrayBuffer (zero-copy).
        ws.onmessage = ev => {
          if (typeof ev.data === 'string') return; // unexpected text frame
          const buffer = ev.data as ArrayBuffer;
          const msg: IncomingMessage = {
            type: IncomingMessageType.Bytes,
            buffer,
          };
          w.postMessage(msg, [buffer]);
        };
        ws.onerror = () => setStatus({ kind: 'error', message: 'ws error' });
        ws.onclose = () =>
          setStatus(prev => (prev.kind === 'error' ? prev : { kind: 'idle' }));

        setStatus({ kind: 'open' });
      } catch (e) {
        if (!cancelled) {
          setStatus({
            kind: 'error',
            message: e instanceof Error ? e.message : String(e),
          });
        }
      }
    })();

    function handleWorkerMessage(m: OutgoingMessage) {
      if (mainPerfRef.current) {
        recordPostMessage(mainPerfRef.current, m.type);
      }
      switch (m.type) {
        case OutgoingMessageType.Log:
        case OutgoingMessageType.Decoded:
          break;
        case OutgoingMessageType.Resolution:
          resolutionRef.current = { width: m.width, height: m.height };
          break;
        case OutgoingMessageType.CursorBitmap:
          applyCursorBitmap(canvas, m);
          break;
        case OutgoingMessageType.CursorHidden:
          canvas.style.cursor = 'none';
          break;
        case OutgoingMessageType.CursorDefault:
          canvas.style.cursor = 'default';
          break;
        case OutgoingMessageType.TdpbUpgrade: {
          const codec = codecRef.current;
          const ws = wsRef.current;
          const dims = initialDimsRef.current;
          if (!codec || !ws || !dims) break;
          codec.upgradeToTdpb();
          try {
            const hello = multiMonitorPayload
              ? codec.encodeClientHelloMulti(
                  dims.username,
                  dims.width,
                  dims.height,
                  dims.scale,
                  dims.keyboardLayout,
                  multiMonitorPayload.xs,
                  multiMonitorPayload.ys,
                  multiMonitorPayload.widths,
                  multiMonitorPayload.heights,
                  multiMonitorPayload.primaries
                )
              : codec.encodeClientHello(
                  dims.username,
                  dims.width,
                  dims.height,
                  dims.scale,
                  dims.keyboardLayout
                );
            ws.send(hello);
          } catch (e) {
            // eslint-disable-next-line no-console
            console.warn('client_hello encode failed', e);
          }
          break;
        }
        case OutgoingMessageType.RdpResponse: {
          const codec = codecRef.current;
          const ws = wsRef.current;
          if (!codec || !ws) break;
          try {
            // The Rust side already has the inner response payload; we
            // wrap it as a `RdpResponsePdu` envelope here.
            const wrapped = codec.encodeRdpResponse(new Uint8Array(m.buffer));
            ws.send(wrapped);
          } catch (e) {
            // eslint-disable-next-line no-console
            console.warn('rdp_response encode failed', e);
          }
          break;
        }
        case OutgoingMessageType.Perf: {
          // eslint-disable-next-line no-console
          console.log(formatPerfLine('new', m, mainPerfRef.current));
          if (mainPerfRef.current) resetMainPerf(mainPerfRef.current);
          break;
        }
        case OutgoingMessageType.SlowPdu: {
          // eslint-disable-next-line no-console
          console.log(
            `[slow-pdu] class=${m.class} len=${m.len}B ms=${m.ms.toFixed(2)}`
          );
          break;
        }
        case OutgoingMessageType.Ready:
          break;
      }
    }

    return () => {
      cancelled = true;
      const mp = mainPerfRef.current;
      if (mp) {
        stopRafSampler(mp);
        mainPerfRef.current = null;
      }
      const ws = wsRef.current;
      if (ws) {
        try {
          ws.close();
        } catch {
          /* ignore */
        }
        wsRef.current = null;
      }
      codecRef.current = null;
      const w = workerRef.current;
      if (w) {
        try {
          w.postMessage({ type: IncomingMessageType.Close });
        } catch {
          /* ignore */
        }
        w.terminate();
        workerRef.current = null;
      }
      cleanup.forEach(fn => fn());
    };
  }, [params]);

  function send(buffer: Uint8Array) {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(buffer);
  }

  function handleMouseMove(e: React.MouseEvent<HTMLCanvasElement>) {
    if (status.kind !== 'open') return;
    const mp = mainPerfRef.current;
    if (mp) mp.domMouseMoveCount += 1;
    const codec = codecRef.current;
    if (!codec) return;
    const rect = e.currentTarget.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) return;
    const { width: cw, height: ch } = resolutionRef.current;
    const x = Math.round(((e.clientX - rect.left) * cw) / rect.width);
    const y = Math.round(((e.clientY - rect.top) * ch) / rect.height);
    try {
      const bytes = codec.encodeMouseMove(x, y);
      if (mp) mp.outboundCount += 1;
      send(bytes);
    } catch {
      // codec is mid-teardown; drop silently
    }
  }

  function handleMouseButton(pressed: boolean) {
    return (e: React.MouseEvent<HTMLCanvasElement>) => {
      if (status.kind !== 'open') return;
      e.preventDefault();
      const codec = codecRef.current;
      if (!codec) return;
      const button = e.button === 1 ? 1 : e.button === 2 ? 2 : 0;

      // Compute current server-side coords once; we use them for both
      // the mouse-button payload and the drag-tracking logic.
      const rect = e.currentTarget.getBoundingClientRect();
      let curX = 0;
      let curY = 0;
      if (rect.width > 0 && rect.height > 0) {
        const { width: cw, height: ch } = resolutionRef.current;
        curX = Math.round(((e.clientX - rect.left) * cw) / rect.width);
        curY = Math.round(((e.clientY - rect.top) * ch) / rect.height);
      }

      try {
        const bytes = codec.encodeMouseButton(button, pressed);
        send(bytes);
      } catch {
        /* ignore */
      }

      // Drag tracking: remember mousedown origin, request a refresh on
      // mouseup if motion exceeded the threshold. Threshold avoids
      // hammering the server on simple clicks.
      const DRAG_REFRESH_THRESHOLD_PX = 4;
      if (button === 0) {
        if (pressed) {
          dragOriginRef.current = { x: curX, y: curY };
        } else {
          const origin = dragOriginRef.current;
          dragOriginRef.current = null;
          if (origin) {
            const dx = Math.abs(curX - origin.x);
            const dy = Math.abs(curY - origin.y);
            if (dx > DRAG_REFRESH_THRESHOLD_PX || dy > DRAG_REFRESH_THRESHOLD_PX) {
              const { width: cw, height: ch } = resolutionRef.current;
              try {
                // Refresh the whole screen. The bounding box of the drag
                // would be enough for a window move, but the server may
                // have repainted areas outside the drag (focus rings,
                // shadows, neighboring controls) — a full refresh is
                // ~50–200ms on the server side and cleans everything.
                const refreshBytes = codec.encodeRefreshRect(
                  0,
                  0,
                  Math.max(0, cw - 1),
                  Math.max(0, ch - 1)
                );
                send(refreshBytes);
              } catch {
                /* ignore */
              }
            }
          }
        }
      }
    };
  }

  return (
    <div style={containerStyle}>
      <canvas
        ref={canvasRef}
        tabIndex={-1}
        onMouseMove={handleMouseMove}
        onMouseDown={handleMouseButton(true)}
        onMouseUp={handleMouseButton(false)}
        onContextMenu={e => e.preventDefault()}
        style={canvasStyle}
      />
      <StatusBadge status={status} />
    </div>
  );
}

/**
 * Sets the canvas's CSS cursor from a server-supplied bitmap. Same recipe
 * as the existing client's `setPointer` — cursors > 32px get scaled down
 * because most browsers reject larger ones for `url()` cursor values.
 */
function applyCursorBitmap(
  canvas: HTMLCanvasElement,
  msg: {
    imageData: ImageData;
    hotspotX: number;
    hotspotY: number;
  }
) {
  const { imageData, hotspotX, hotspotY } = msg;
  let buffer = document.createElement('canvas');
  buffer.width = imageData.width;
  buffer.height = imageData.height;
  buffer
    .getContext('2d', { colorSpace: imageData.colorSpace })!
    .putImageData(imageData, 0, 0);

  let scale = 1;
  if (buffer.width > 32 || buffer.height > 32) {
    scale = Math.min(32 / buffer.width, 32 / buffer.height);
    const resized = document.createElement('canvas');
    resized.width = Math.max(1, Math.round(buffer.width * scale));
    resized.height = Math.max(1, Math.round(buffer.height * scale));
    const ctx = resized.getContext('2d', { colorSpace: imageData.colorSpace })!;
    ctx.scale(scale, scale);
    ctx.drawImage(buffer, 0, 0);
    buffer = resized;
  }

  canvas.style.cursor = `url(${buffer.toDataURL()}) ${Math.round(hotspotX * scale)} ${Math.round(hotspotY * scale)}, auto`;
}

function StatusBadge({ status }: { status: Status }) {
  if (status.kind === 'open') return null;
  const styles: Record<Status['kind'], React.CSSProperties> = {
    idle: { background: '#e0e0e0', color: '#333' },
    connecting: { background: '#fff7d6', color: '#7a5c00' },
    open: { background: '#d6f5d6', color: '#1f5e1f' },
    error: { background: '#f8d6d6', color: '#7a1f1f' },
  };
  return (
    <span
      style={{
        position: 'fixed',
        top: 8,
        right: 8,
        padding: '4px 10px',
        borderRadius: 12,
        fontFamily: 'monospace',
        fontSize: 12,
        pointerEvents: 'none',
        zIndex: 1,
        ...styles[status.kind],
      }}
    >
      {status.kind}
      {status.kind === 'error' ? `: ${status.message}` : ''}
    </span>
  );
}

/**
 * Bounding box of all monitor regions in virtual-desktop coordinates. Used
 * to size the canvas drawing buffer so server-side frame coords land in the
 * right pixels regardless of which monitor produced them.
 */
function computeBoundingBox(monitors: MonitorSpec[]): {
  width: number;
  height: number;
} {
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  for (const m of monitors) {
    if (m.x < minX) minX = m.x;
    if (m.y < minY) minY = m.y;
    if (m.x + m.width > maxX) maxX = m.x + m.width;
    if (m.y + m.height > maxY) maxY = m.y + m.height;
  }
  return { width: Math.max(1, maxX - minX), height: Math.max(1, maxY - minY) };
}

/**
 * Packs the monitor list into the parallel typed arrays the wasm-bindgen
 * `encode*Multi` methods accept. Done once at component mount; the resulting
 * arrays are reused for the initial ScreenSpec and the post-TDPB ClientHello.
 */
function toCodecArrays(monitors: MonitorSpec[]): {
  xs: Int32Array;
  ys: Int32Array;
  widths: Uint32Array;
  heights: Uint32Array;
  primaries: Uint8Array;
} {
  const n = monitors.length;
  const xs = new Int32Array(n);
  const ys = new Int32Array(n);
  const widths = new Uint32Array(n);
  const heights = new Uint32Array(n);
  const primaries = new Uint8Array(n);
  for (let i = 0; i < n; i++) {
    const m = monitors[i];
    xs[i] = m.x;
    ys[i] = m.y;
    widths[i] = m.width;
    heights[i] = m.height;
    primaries[i] = m.isPrimary ? 1 : 0;
  }
  return { xs, ys, widths, heights, primaries };
}

const containerStyle: React.CSSProperties = {
  position: 'fixed',
  inset: 0,
  background: '#000',
  margin: 0,
  padding: 0,
};
const canvasStyle: React.CSSProperties = {
  width: '100%',
  height: '100%',
  display: 'block',
  outline: 'none',
  // Preserve aspect ratio. Multi-monitor virtual desktops have unusual
  // aspect ratios (e.g. 5120×1080 for two side-by-side 2560×1080 monitors);
  // without scale-down the browser stretches them to the window shape.
  objectFit: 'scale-down',
  // Tell the browser to use NEAREST when scaling the WebGL drawing
  // buffer to display device pixels. Browsers vary in their default
  // (canvas2D often blurs with bilinear, WebGL sometimes does the
  // same) — fixing it explicitly avoids the faint regular stripes
  // we'd otherwise see at non-integer DPR ratios.
  imageRendering: 'pixelated',
};
