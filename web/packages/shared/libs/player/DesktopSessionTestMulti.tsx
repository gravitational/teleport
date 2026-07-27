/**
 * Multi-monitor variant of `DesktopSessionTest`.
 *
 * Architecture:
 *   - A DedicatedWorker (`codecTestWorker.ts`) per main window hosts the
 *     WebSocket, IronRDP `WorkerDecoder`, and a `VideoDecoder` per AVC
 *     surface. The decoder owns ONE framebuffer (the whole virtual desktop)
 *     and drives N registered canvases — one per monitor — off a single
 *     decode (`WorkerDecoder.addCanvas`).
 *   - Main transfers control of its own canvas to the worker
 *     (`transferControlToOffscreen`) and registers it as viewport 0. Each
 *     popup does the same with its canvas and sends it up with its 'ready'
 *     message; main forwards it to the worker (register-canvas) with that
 *     popup's physical viewport. wasm then paints every monitor's slice
 *     straight into its canvas over GL — no per-frame pixel copy, no
 *     paintFrame round-trip. (Works because same-origin opener-connected
 *     popups share main's renderer process, so the worker can drive their
 *     placeholder canvases.)
 *   - Outbound (mouse/keyboard) bytes are still encoded on main (its own
 *     `MainCodec`) and sent via `worker.postMessage({type:'outbound', ...})`.
 *     `RdpResponsePdu` wrapping happens inside the worker. Popups forward raw
 *     input events to main, which encodes them.
 */

import { useEffect, useMemo, useRef, useState } from 'react';

import init, {
  initWasmLog,
  MainCodec,
  type MainCodec as MainCodecType,
} from './pkg/session';
import CodecTestWorker from './codecTestWorker?worker';
import { installMainLogSink, attachLogSinkWorker } from './logsink';

import { KEY_SCANCODES } from 'shared/libs/tdp/codec';

import type { MonitorSpec } from './DesktopSessionTest';
import type { PopupToMain } from './DesktopSessionPopupDisplay';
import { MonitorModel } from './monitors/monitorModel';
import {
  contentScreenPosition,
  physViewport,
  pickSpawnRect,
} from './monitors/monitorLayout';
import {
  clearLayout,
  popupCount,
  saveLayout,
  serializeLayout,
  type SavedLayout,
} from './monitors/monitorPersistence';
import type {
  MonitorSession,
  MonitorSessionState,
} from './monitors/monitorSession';
import {
  useScreenTopology,
  type DisplayInfo,
} from './monitors/useScreenTopology';
import { MonitorTaskbar } from './monitors/MonitorTaskbar';
import { MonitorManagerPanel } from './monitors/MonitorManagerPanel';

// Capture main-realm logs and expose the async `globalThis.L` reducer that also
// queries the worker buffer. Idempotent (guarded inside). See logsink.ts.
installMainLogSink();

export type WindowMode = 'main' | 'popup';

export interface DesktopSessionTestMultiProps {
  wsUrl: string;
  bearerToken: string;
  username: string;
  scale?: number;
  keyboardLayout?: number;
  monitors: MonitorSpec[];
  monitorIndex: number;
  mode: WindowMode;
  popups?: PopupSpec[];
  /**
   * Builds the URL for a popup window backing monitor `monitorIndex`. Required
   * for adding monitors mid-session from the management UI; the harness route
   * supplies it (it knows how to encode the popup query string).
   */
  popupUrlFactory?: (monitorIndex: number) => string;
  /** Product cap on simultaneous monitors (mirrors the server). */
  maxMonitors?: number;
  /**
   * `localStorage` key under which the live monitor layout is saved (while it
   * has popups) and from which a restore offer is loaded. Omit to disable
   * persistence.
   */
  layoutStorageKey?: string;
  /**
   * A previously-saved layout to offer for one-click restore (the taskbar shows
   * "Restore N monitors"). Captured by the caller before this component clears
   * the stored entry, so a session that never restores forgets it.
   */
  restoreLayout?: SavedLayout | null;
}

export interface PopupSpec {
  monitorIndex: number;
  url: string;
  name: string;
  screenLeft: number;
  screenTop: number;
  width: number;
  height: number;
}

type Status =
  | { kind: 'idle' }
  | { kind: 'connecting' }
  | { kind: 'open' }
  | { kind: 'error'; message: string };

let wasmReady: Promise<unknown> | null = null;

// DedicatedWorker message shapes (must stay in lockstep with codecTestWorker.ts).
type WorkerInMsg =
  | { type: 'host'; wsUrl: string; bearerToken: string; egfxOffload?: boolean; workers?: number }
  | { type: 'outbound'; buffer: ArrayBuffer }
  | { type: 'unregister' }
  | {
      type: 'register-canvas';
      canvasId: number;
      canvas: OffscreenCanvas;
      viewportX: number;
      viewportY: number;
      viewportWidth: number;
      viewportHeight: number;
    }
  | { type: 'unregister-canvas'; canvasId: number }
  | {
      type: 'reposition-canvas';
      canvasId: number;
      viewportX: number;
      viewportY: number;
      viewportWidth: number;
      viewportHeight: number;
    };

type WorkerOutMsg =
  | { type: 'state'; state: 'closed' | 'authing' | 'open' | 'error'; message?: string }
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
  | { type: 'slowPdu'; class: string; ms: number; len: number }
  | { type: 'gpuBenchResult'; result: unknown }
  | {
      type: 'perf-data';
      data: {
        stages: { tiles: number; entropy: number; idwt: number; color: number };
        present: { p50: number; p95: number };
        arrival: { p50: number };
        inflight: number;
      };
    };

export function DesktopSessionTestMulti({
  wsUrl,
  bearerToken,
  username,
  scale = 100,
  keyboardLayout = 0,
  monitors,
  monitorIndex,
  mode,
  popups,
  popupUrlFactory,
  maxMonitors = 4,
  layoutStorageKey,
  restoreLayout,
}: DesktopSessionTestMultiProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const workerRef = useRef<Worker | null>(null);
  const codecRef = useRef<MainCodecType | null>(null);
  const dragOriginRef = useRef<{ x: number; y: number } | null>(null);
  // Sub-pixel wheel carry. macOS trackpads emit fractional pixel deltas; we
  // accumulate and only ship the whole-pixel part each event, keeping the
  // remainder so slow/momentum scrolls aren't rounded away. { x: horizontal,
  // y: vertical }, already sign-flipped for remote scroll direction.
  const wheelAccumRef = useRef<{ x: number; y: number }>({ x: 0, y: 0 });
  // Layout entry for THIS window (main) — viewport in virtual-desktop coords.
  // Set after all popups have reported ready and main has computed the
  // multi-monitor layout. Single-monitor sessions skip the wait.
  const layoutEntryRef = useRef<{
    x: number;
    y: number;
    width: number;
    height: number;
  } | null>(null);
  // Popup windows we opened, keyed by monitorIndex. Used both to route
  // paintFrame messages (postMessage with rgba transferable) and to close
  // popups when this component unmounts.
  const popupWindowsRef = useRef<Map<number, Window>>(new Map());
  // Viewport of each popup in virtual-desktop coords. Drives paintFrame
  // routing (which popup the frame belongs to).
  const popupViewportsRef = useRef<
    Map<number, { x: number; y: number; width: number; height: number }>
  >(new Map());
  const bboxRef = useRef<{ width: number; height: number } | null>(null);
  // Gates for the ClientHello: needs both the server's TDPB-upgrade signal
  // and the computed layout before it can encode. Sending ClientHello with
  // the right dims up front avoids a follow-up resize PDU that some Windows
  // builds reject with "display driver" errors.
  const tdpbReadyRef = useRef(false);
  const layoutForHelloRef = useRef<{
    monitors: Array<{
      monitorIndex: number;
      x: number;
      y: number;
      width: number;
      height: number;
    }>;
    bboxWidth: number;
    bboxHeight: number;
  } | null>(null);
  const helloSentRef = useRef(false);
  const [status, setStatus] = useState<Status>({ kind: 'idle' });
  // Render FPS, derived from the wasm `perf` payload's per-second `paints`
  // (canvas uploads/sec ≈ frames presented/sec). null until the first sample.
  const [fps, setFps] = useState<number | null>(null);
  // Worker CPU utilization for inbound decode (busy_ms/elapsed). ~90%+ means
  // the worker is the bottleneck; low means the server is only sending N fps.
  const [cpu, setCpu] = useState<number | null>(null);
  // Mirror of `status` so process* functions and the popup-message listener
  // (registered inside useEffect and thus prone to stale closures) can read
  // the latest status without React-state staleness.
  const statusRef = useRef<Status>({ kind: 'idle' });
  statusRef.current = status;

  // Live physical-display topology (Window Management API). Passed into the
  // session effect via a ref so the effect needn't re-run when it updates.
  const topology = useScreenTopology();
  const topologyRef = useRef(topology);
  topologyRef.current = topology;
  // Imperative monitor controller, created inside the session effect. The
  // taskbar + management panel drive it. `sessionState` mirrors it for render.
  const controllerRef = useRef<MonitorSession | null>(null);
  const [sessionState, setSessionState] = useState<MonitorSessionState | null>(
    null
  );
  const [monitorPanelOpen, setMonitorPanelOpen] = useState(false);
  // --- Cross-window input ---
  // Tier 1: a button-held drag started on this canvas keeps tracking the cursor
  // at the window level so it follows across the monitor seam into a popup.
  const dragActiveRef = useRef(false);
  // Set by beginDrag's window-level mouseup so the canvas's bubble-phase
  // onMouseUp doesn't fire a second release for the same drag. Reset on every
  // mousedown so a release-over-a-popup (no canvas onMouseUp) can't leave it stale.
  const suppressMouseUpRef = useRef(false);
  // Tier 2: pointer-lock "capture" mode. While captured, the real cursor is
  // hidden and all input flows through the main window as relative motion into
  // a single virtual-desktop cursor; popups become display-only and render a
  // synthetic cursor sprite.
  const [captureMode, setCaptureMode] = useState(false);
  // Sticky-capture intent: `captureMode` tracks the raw pointer lock (which
  // the browser force-drops on every Esc); "engaged" tracks the user's
  // intent, which survives those drops so the next keystroke/click can
  // re-acquire the lock. Ref for event handlers, state for the taskbar.
  const captureEngagedRef = useRef(false);
  const [captureEngaged, setCaptureEngaged] = useState(false);
  // Last pointer position forwarded to the server (any source: main-canvas
  // moves, popup moves, drags), in CSS-px bbox space. Seeds the capture-mode
  // virtual cursor so engaging capture starts from where the pointer is.
  const lastPointerCssRef = useRef<{ x: number; y: number } | null>(null);
  // HiDPI mid-stream toggle (AVC probe). `scaleRef` is the LIVE scale read by
  // every runtime coord/viewport/screenspec path, so toggling it doesn't
  // reconnect; `reflowRef` re-runs the layout apply (canvas reposition +
  // DisplayControl screenspec) so the host re-renders at the new resolution and
  // we can watch the node log for whether AVC keeps flowing. `hidpiOn` drives
  // only the taskbar button label.
  const scaleRef = useRef(scale);
  const reflowRef = useRef<(() => void) | null>(null);
  const [hidpiOn, setHidpiOn] = useState(scale > 100);
  const virtualCursorRef = useRef<{ x: number; y: number }>({ x: 0, y: 0 });
  const cursorSpriteRef = useRef<{
    url: string;
    hotspotX: number;
    hotspotY: number;
    scale: number;
  } | null>(null);
  const captureSpriteElRef = useRef<HTMLImageElement | null>(null);

  const params = useMemo(
    () => ({ wsUrl, bearerToken, username, scale, keyboardLayout, mode, monitorIndex }),
    [wsUrl, bearerToken, username, scale, keyboardLayout, mode, monitorIndex]
  );

  useEffect(() => {
    // Popup mode is implemented separately in step 3 of the refactor — for
    // now the popup just renders a placeholder. When the parent route
    // (`DesktopSessionCodecTest.tsx`) sees `window.opener !== null` it
    // mounts the dedicated passive component instead of this one.
    if (mode === 'popup') return;

    const canvas = canvasRef.current;
    if (!canvas) return;
    // CSS dimensions: the visible window area, used only for layout.
    // Drawing buffer dimensions: scaled by the requested percent (e.g. 200
    // on a Retina display) so the canvas can hold the physical-pixel
    // bitmap Windows is rendering for us. The CSS size stays at the CSS
    // pixel count via inline style so the canvas still occupies the same
    // visible footprint. Round both down to even values: MS-RDPEDISP
    // requires width to be even and several Windows display drivers
    // reject monitor layouts with odd heights.
    const cssWidth = Math.max(2, window.innerWidth & ~1);
    const cssHeight = Math.max(2, window.innerHeight & ~1);
    const scaleRatio = Math.max(1, scale / 100);
    const myWidth = Math.max(2, Math.round(cssWidth * scaleRatio) & ~1);
    const myHeight = Math.max(2, Math.round(cssHeight * scaleRatio) & ~1);
    canvas.width = myWidth;
    canvas.height = myHeight;
    // CSS size stays at 100%/100% (from the JSX) so the canvas follows the
    // window when it's resized — the wasm painter resizes the backing buffer on
    // reposition. (Pinning a fixed px size here froze the primary's footprint,
    // so resizing the main window never changed the primary's resolution.)

    // Hand control of this canvas to the worker via an OffscreenCanvas: the
    // wasm framebuffer paints into it directly, so fast-path bitmap PDUs,
    // ClearCodec EgfxBitmap, and decoded AVC RGBA all render into the
    // pixels the user sees with no per-frame `putImageData` round-trip.
    // transferControlToOffscreen can only be called once per canvas; React
    // strict-mode double-mount is safe because the effect's cleanup runs
    // first and we re-create the canvas element each mount via the JSX ref.
    let offscreen: OffscreenCanvas | null = null;
    try {
      offscreen = canvas.transferControlToOffscreen();
    } catch (e) {
      setStatus({
        kind: 'error',
        message: `transferControlToOffscreen failed: ${String(e)}`,
      });
      return;
    }

    setStatus({ kind: 'connecting' });

    let cancelled = false;
    // Polls for popup windows the user closed, so we can free their canvas and
    // shrink the RDP desktop. Set up once popups exist; cleared on unmount.
    let closePollId: ReturnType<typeof setInterval> | undefined;

    // --- Live monitor model + controller side state ---
    // The MonitorModel is the source of truth for the monitor set; the
    // management UI drives it through `controllerRef`. Seeded from props
    // (main + any initial popups); DOM/worker side state lives alongside.
    // Content position, not the OS frame — main's chrome is taller than a
    // popup's, and the layout must align canvases across the seam.
    const mainPhysical = contentScreenPosition(window);
    const model = new MonitorModel(
      monitors.map((m, idx) => ({
        id: idx,
        role: idx === monitorIndex ? ('main' as const) : ('popup' as const),
        // Main's RDP resolution must be the *window inner* size (what the
        // canvas backing buffer is sized to), NOT the display size from the
        // prop — otherwise the server renders the primary at a different size
        // than the canvas, stretching it. Popups self-correct to their inner
        // size in their 'ready' message, so the prop dims are just a seed.
        cssWidth: idx === monitorIndex ? cssWidth : m.width,
        cssHeight: idx === monitorIndex ? cssHeight : m.height,
        isPrimary: !!m.isPrimary,
        physical: idx === monitorIndex ? mainPhysical : undefined,
        status:
          idx === monitorIndex ? ('active' as const) : ('pending' as const),
      })),
      maxMonitors
    );

    // --- Layout persistence + one-click restore ---
    // The restore offer is the prior session's layout (loaded by the caller
    // before this component clears the stored entry). We only offer it when this
    // session starts single-monitor (no initial popups); a manual layout change
    // or an actual restore dismisses it.
    let restoreOffer: SavedLayout | null =
      (popups?.length ?? 0) === 0 ? (restoreLayout ?? null) : null;
    let restorablePopups = restoreOffer ? popupCount(restoreOffer) : 0;
    let persistDebounce: ReturnType<typeof setTimeout> | undefined;
    // Save the live layout while it has popups; clear it otherwise. So a session
    // that never restores (stays single-monitor) forgets the previous layout.
    function persistLayout() {
      if (!layoutStorageKey) return;
      clearTimeout(persistDebounce);
      persistDebounce = setTimeout(() => {
        if (cancelled) return;
        const live = serializeLayout(model.list(), model.getArrangement());
        if (popupCount(live) > 0) saveLayout(layoutStorageKey, live);
        else clearLayout(layoutStorageKey);
      }, 400);
    }
    // The model has no restore concept, so fold `restorable` in here.
    const currentState = (): MonitorSessionState => ({
      ...model.getState(),
      restorable: restorablePopups,
    });
    // Forget the restore offer — the user is building their own layout now.
    function dismissRestoreOffer() {
      restoreOffer = null;
      restorablePopups = 0;
    }

    // Each popup transfers control of its visible canvas up with its 'ready'
    // message; held here until the layout registers it with the worker.
    const popupCanvases = new Map<number, OffscreenCanvas>();
    // Zone placements (FRAME bounds) requested for popups still opening.
    // window.open can only size the CONTENT area (we estimate the chrome),
    // so once the popup is ready — real chrome readable — it's trued-up to
    // the exact frame bounds via setMonitorWindowBounds.
    const pendingPlacements = new Map<
      number,
      { left: number; top: number; width: number; height: number }
    >();
    // Canvas ids already registered with the worker (0 = main, idx+1 = popups).
    const registered = new Set<number>();
    // Stop-tracking callbacks for live physical-position polling, by monitor id.
    const untrackers = new Map<number, () => void>();
    // Initial popups we must hear 'ready' from before sending the first
    // ClientHello (so the server gets the final multi-monitor dims up front).
    const pendingInitialPopups = new Set<number>(
      (popups ?? []).map(p => p.monitorIndex)
    );
    // Main's OffscreenCanvas; registered lazily in the first applyLayout once
    // its viewport within the bounding box is known.
    let mainOffscreen: OffscreenCanvas | null = offscreen;
    // Latest server-layout arrays awaiting the ClientHello / screen-spec send.
    let pendingLayout: {
      bboxWidth: number;
      bboxHeight: number;
      xs: Int32Array;
      ys: Int32Array;
      widths: Uint32Array;
      heights: Uint32Array;
      primaries: Uint8Array;
    } | null = null;
    let refreshScheduled = false;
    let specDebounce: ReturnType<typeof setTimeout> | undefined;
    // True while the layout panel is mid-drag/resize: applyLayout stays
    // local-only (no server spec push) until the interaction commits.
    let layoutHold = false;
    // Signature of the last layout sent to the server (hello or screen-spec),
    // so we never send a redundant resize — a no-op DisplayControl PDU is
    // wasteful and can disturb Windows' multi-monitor surface state.
    let lastSentSig: string | null = null;
    // HiDPI toggle hook: re-run the layout apply with the LIVE scaleRef, busting
    // the dedup (layoutSig ignores scale, so a scale-only change has the same
    // sig). applyLayout is hoisted, so referencing it here is safe.
    reflowRef.current = () => {
      lastSentSig = null;
      applyLayout();
    };
    const layoutSig = (L: {
      bboxWidth: number;
      bboxHeight: number;
      xs: Int32Array;
      ys: Int32Array;
      widths: Uint32Array;
      heights: Uint32Array;
      primaries: Uint8Array;
    }) =>
      `${L.bboxWidth}x${L.bboxHeight}|${L.xs.join(',')}|${L.ys.join(',')}|` +
      `${L.widths.join(',')}|${L.heights.join(',')}|${L.primaries.join(',')}`;

    // Listen for popup → main messages: 'ready' (size report) and 'input'
    // (forwarded mouse/keyboard). Registered synchronously here, before any
    // window.open, so popups can't race us to send 'ready'.
    function handlePopupMessage(ev: MessageEvent<PopupToMain>) {
      let owned = false;
      for (const [, w] of popupWindowsRef.current) {
        if (w === ev.source) {
          owned = true;
          break;
        }
      }
      if (!owned) return;
      const m = ev.data;
      if (!m || typeof m !== 'object') return;
      switch (m.type) {
        case 'ready': {
          if (!model.has(m.monitorIndex)) return;
          const rdyWin = popupWindowsRef.current.get(m.monitorIndex);
          model.upsert({
            id: m.monitorIndex,
            role: 'popup',
            cssWidth: m.innerWidth,
            cssHeight: m.innerHeight,
            isPrimary: model.get(m.monitorIndex)?.isPrimary ?? false,
            physical: rdyWin ? contentScreenPosition(rdyWin) : undefined,
            status: 'active',
          });
          // Hold the transferred OffscreenCanvas until the layout registers it.
          if (m.canvas) popupCanvases.set(m.monitorIndex, m.canvas);
          pendingInitialPopups.delete(m.monitorIndex);
          if (rdyWin) trackMonitor(m.monitorIndex, rdyWin, true);
          // Zone add: true-up to the exact requested frame bounds now that
          // the real chrome height is readable — window.open sized the
          // CONTENT area from an estimate, and the estimate error showed up
          // as a gap between vertically adjacent zone placements.
          const want = pendingPlacements.get(m.monitorIndex);
          if (want) {
            pendingPlacements.delete(m.monitorIndex);
            controllerRef.current?.setMonitorWindowBounds(m.monitorIndex, want);
          }
          // eslint-disable-next-line no-console
          console.log('[monitors] popup ready', {
            id: m.monitorIndex,
            w: m.innerWidth,
            h: m.innerHeight,
            hasCanvas: !!m.canvas,
          });
          applyLayout();
          break;
        }
        case 'input':
          switch (m.subtype) {
            case 'mousemove':
            case 'mouseleave':
              processMouseMove(m.virtualX, m.virtualY);
              break;
            case 'mousebutton':
              processMouseButton(
                m.button,
                m.pressed,
                m.virtualX,
                m.virtualY
              );
              break;
            case 'key':
              // Popup keys carry their event's lock states so an armed sync
              // fires here too, not just for main-canvas keys.
              maybeSyncKeys(m);
              processKey(m.code, m.pressed);
              break;
            case 'armSync':
              syncBeforeNextKeyRef.current = true;
              break;
            case 'wheel':
              processMouseWheel(m.deltaX, m.deltaY);
              break;
            case 'paste':
              void processPaste(m.text);
              break;
          }
          break;
      }
    }
    // Close every secondary popup. Reused by the React cleanup AND a
    // `pagehide` listener: React effect cleanup does NOT run on a real page
    // refresh / close / navigate (only on SPA unmount), so without `pagehide`
    // the popups would be orphaned when the main window goes away.
    function closeAllPopups() {
      for (const [, win] of popupWindowsRef.current) {
        try {
          win.close();
        } catch {
          /* ignore */
        }
      }
      popupWindowsRef.current.clear();
    }
    window.addEventListener('message', handlePopupMessage);
    window.addEventListener('pagehide', closeAllPopups);

    void (async () => {
      try {
        if (!wasmReady) {
          wasmReady = init().then(() => initWasmLog('debug'));
        }
        await wasmReady;
        if (cancelled) return;

        const codec = new MainCodec();
        codecRef.current = codec;

        const worker = new CodecTestWorker();
        workerRef.current = worker;
        // Route log-reduction queries (`L.*`) to the worker's buffer. Uses a
        // separate `message` listener, so it coexists with `worker.onmessage`.
        attachLogSinkWorker(worker);
        worker.onmessage = (ev: MessageEvent<WorkerOutMsg>) =>
          handleWorkerMessage(ev.data);

        // [gpubench] Console-triggerable GPU plumbing microbench: runs in the
        // worker (real GL context), replies with the timing. Usage in the main
        // DevTools console: `await gpuBench()` (R16I) or `await gpuBench(1600, 12, 20, 32)`
        // to A/B the R32I baseline on the same hardware.
        (globalThis as { gpuBench?: unknown }).gpuBench = (
          numTiles?: number,
          passesPerComponent?: number,
          iters?: number,
          bits?: 16 | 32
        ) =>
          new Promise(resolve => {
            const onMsg = (ev: MessageEvent) => {
              const d = ev.data as { type?: string; result?: unknown };
              if (d?.type === 'gpuBenchResult') {
                worker.removeEventListener('message', onMsg);
                resolve(d.result);
              }
            };
            worker.addEventListener('message', onMsg);
            worker.postMessage({
              type: 'gpuBench',
              numTiles,
              passesPerComponent,
              iters,
              bits,
            });
          });

        // [simd-idwt] Runtime SIMD IDWT toggle + single-command A/B perf harness.
        // Mirror the gpuBench global pattern above.
        (globalThis as any).setSimdIdwt = (on: boolean) =>
          worker.postMessage({ type: 'set-simd-idwt', on });
        (globalThis as any).resetPerf = () =>
          worker.postMessage({ type: 'reset-perf' });
        (globalThis as any).perfData = () =>
          new Promise(res => {
            const onMsg = (ev: MessageEvent) => {
              if ((ev.data as any)?.type === 'perf-data') {
                worker.removeEventListener('message', onMsg);
                res((ev.data as any).data);
              }
            };
            worker.addEventListener('message', onMsg);
            worker.postMessage({ type: 'perf-data' });
          });
        (globalThis as any).perfAB = async (secs = 10) => {
          const sleep = (s: number) =>
            new Promise(r => setTimeout(r, s * 1000));
          const sample = async (on: boolean) => {
            (globalThis as any).setSimdIdwt(on);
            (globalThis as any).resetPerf();
            await sleep(secs);
            return await (globalThis as any).perfData();
          };
          const off = await sample(false);
          const on = await sample(true);
          (globalThis as any).setSimdIdwt(true);
          const tot = (p: any) =>
            p.stages.entropy + p.stages.idwt + p.stages.color;
          const row = (tag: string, p: any) =>
            `${tag} idwt=${p.stages.idwt.toFixed(1)}us present.p50=${p.present.p50.toFixed(1)}ms inflight=${p.inflight} per-tile=${tot(p).toFixed(1)}us`;
          // eslint-disable-next-line no-console
          console.log(
            '[perfAB]\n' +
              row('scalar', off) +
              '\n' +
              row('simd  ', on) +
              `\nidwt speedup ${(off.stages.idwt / on.stages.idwt).toFixed(2)}x   per-tile ${tot(off).toFixed(1)} -> ${tot(on).toFixed(1)}us`
          );
          return { off, on };
        };

        // Main's OffscreenCanvas (held in `mainOffscreen`) is registered by
        // the first applyLayout, once its viewport within the bounding box is
        // known. The wasm queues canvas registrations until ServerHello, so
        // doing it slightly later than `host` is fine.
        worker.postMessage({
          type: 'host',
          wsUrl,
          bearerToken,
          // Diagnostic A/B: `?offload=0` decodes RFX inline (wire-ordered),
          // anything else keeps the decode-worker pool on. See
          // RFX_POOL_CORRUPTION_HANDOFF.md "First diagnostic".
          egfxOffload:
            new URLSearchParams(window.location.search).get('offload') !== '0',
          // `?workers=N` overrides the decode-pool size — sweep it with perfAB across
          // monitor counts to find the multi-monitor sweet spot. Absent → hardwareConcurrency-derived.
          workers: (() => {
            const w = new URLSearchParams(window.location.search).get('workers');
            const n = w ? parseInt(w, 10) : NaN;
            return Number.isFinite(n) && n > 0 ? n : undefined;
          })(),
        } satisfies WorkerInMsg);

        // Open popup windows now. Each popup loads the same route with
        // ?popup=1 in its URL; the entry component renders
        // DesktopSessionPopupDisplay there. We don't block on opens because
        // popups need this main page to be live to receive their 'ready'
        // messages.
        if (popups) {
          for (const p of popups) {
            const win = window.open(
              p.url,
              p.name,
              `popup=yes,left=${p.screenLeft},top=${p.screenTop},width=${p.width},height=${p.height}`
            );
            if (win) {
              popupWindowsRef.current.set(p.monitorIndex, win);
            } else {
              // Popup blocked. Don't wait for it; reflect blocked in the UI.
              pendingInitialPopups.delete(p.monitorIndex);
              model.setStatus(p.monitorIndex, 'blocked');
            }
          }
        }

        applyLayout();
      } catch (e) {
        if (!cancelled) {
          setStatus({
            kind: 'error',
            message: e instanceof Error ? e.message : String(e),
          });
        }
      }
    })();

    function pushState() {
      setSessionState(currentState());
      persistLayout();
    }

    // Recompute the layout from the model and apply it: (re)register /
    // reposition each monitor's canvas with the worker, tell each popup its
    // viewport, and push the new layout to the server. Idempotent.
    function applyLayout() {
      if (cancelled) return;
      // Every layout change (add/remove/move/resize/primary) flows through here,
      // so this is where we persist the live layout.
      persistLayout();
      // Hold the first layout until every initial popup has reported in, so the
      // ClientHello carries the final multi-monitor dimensions.
      if (!helloSentRef.current && pendingInitialPopups.size > 0) {
        pushState();
        return;
      }
      const worker = workerRef.current;
      const { monitors: layout, bboxWidth, bboxHeight } = model.computeLayout();
      if (layout.length === 0) {
        pushState();
        return;
      }
      // eslint-disable-next-line no-console
      console.log('[monitors] applyLayout', {
        bbox: { w: bboxWidth, h: bboxHeight },
        helloSent: helloSentRef.current,
        monitors: layout.map(v => ({
          id: v.id,
          x: v.x,
          y: v.y,
          w: v.width,
          h: v.height,
          primary: v.isPrimary,
        })),
      });
      const s = scaleRef.current;
      for (const vm of layout) {
        const cssRect = { x: vm.x, y: vm.y, width: vm.width, height: vm.height };
        // Physical framebuffer-pixel viewport, matching the server's applyScale.
        const phys = physViewport(vm, s);
        if (vm.id === monitorIndex) {
          layoutEntryRef.current = cssRect;
          if (worker && !registered.has(0) && mainOffscreen) {
            worker.postMessage(
              {
                type: 'register-canvas',
                canvasId: 0,
                canvas: mainOffscreen,
                viewportX: phys.x,
                viewportY: phys.y,
                viewportWidth: phys.width,
                viewportHeight: phys.height,
              } satisfies WorkerInMsg,
              [mainOffscreen]
            );
            mainOffscreen = null;
            registered.add(0);
          } else if (worker && registered.has(0)) {
            worker.postMessage({
              type: 'reposition-canvas',
              canvasId: 0,
              viewportX: phys.x,
              viewportY: phys.y,
              viewportWidth: phys.width,
              viewportHeight: phys.height,
            } satisfies WorkerInMsg);
          }
        } else {
          popupViewportsRef.current.set(vm.id, cssRect);
          const canvasId = vm.id + 1;
          if (worker && !registered.has(canvasId)) {
            const off = popupCanvases.get(vm.id);
            if (off) {
              worker.postMessage(
                {
                  type: 'register-canvas',
                  canvasId,
                  canvas: off,
                  viewportX: phys.x,
                  viewportY: phys.y,
                  viewportWidth: phys.width,
                  viewportHeight: phys.height,
                } satisfies WorkerInMsg,
                [off]
              );
              popupCanvases.delete(vm.id);
              registered.add(canvasId);
            }
          } else if (worker && registered.has(canvasId)) {
            worker.postMessage({
              type: 'reposition-canvas',
              canvasId,
              viewportX: phys.x,
              viewportY: phys.y,
              viewportWidth: phys.width,
              viewportHeight: phys.height,
            } satisfies WorkerInMsg);
          }
          // Tell the popup its viewport so it can translate local mouse coords.
          const win = popupWindowsRef.current.get(vm.id);
          if (win && !win.closed) {
            try {
              win.postMessage(
                {
                  type: 'init',
                  layoutEntry: cssRect,
                  bbox: { width: bboxWidth, height: bboxHeight },
                  scale: s,
                },
                '*'
              );
            } catch {
              /* popup may be closing */
            }
          }
        }
      }
      bboxRef.current = { width: bboxWidth, height: bboxHeight };

      const sorted = [...layout].sort((a, b) => a.id - b.id);
      pendingLayout = {
        bboxWidth,
        bboxHeight,
        xs: new Int32Array(sorted.map(m => m.x)),
        ys: new Int32Array(sorted.map(m => m.y)),
        widths: new Uint32Array(sorted.map(m => m.width)),
        heights: new Uint32Array(sorted.map(m => m.height)),
        primaries: new Uint8Array(sorted.map(m => (m.isPrimary ? 1 : 0))),
      };
      // Keep the hello-layout ref populated (read by the tdpbUpgrade handler).
      layoutForHelloRef.current = {
        monitors: sorted.map(m => ({
          monitorIndex: m.id,
          x: m.x,
          y: m.y,
          width: m.width,
          height: m.height,
        })),
        bboxWidth,
        bboxHeight,
      };

      if (!helloSentRef.current) {
        maybeSendClientHello();
        if (!refreshScheduled) {
          refreshScheduled = true;
          scheduleSecondaryRefresh(bboxWidth, bboxHeight);
        }
      } else if (!layoutHold) {
        // Held during an interactive panel drag/resize: local canvas
        // placement above still applied, but the server layout push waits
        // for endMonitorInteraction — every DisplayControl resize makes
        // Windows reset graphics (surfaces destroyed/recreated), so a
        // mid-drag stream of them tears the picture apart.
        scheduleScreenSpec();
      }
      pushState();
    }


    // Debounced screen-spec send for mid-session layout changes (drag, add,
    // remove, monitor move/resize) — coalesces rapid updates into one PDU.
    // The window is a QUIESCENCE period: every server push costs a graphics
    // reset, and an OS-level titlebar drag feeds position changes through the
    // tracker for seconds at a time — a short debounce sent one reset per
    // tracker tick for the whole drag (holes flashing in and out).
    const SPEC_QUIESCENCE_MS = 600;
    function scheduleScreenSpec() {
      if (specDebounce !== undefined) clearTimeout(specDebounce);
      specDebounce = setTimeout(() => {
        specDebounce = undefined;
        const codec = codecRef.current;
        const worker = workerRef.current;
        const L = pendingLayout;
        if (!codec || !worker || !L) return;
        // Skip when the layout is identical to what the server already has —
        // avoids redundant DisplayControl resize PDUs (and the early-resize
        // race before the DisplayControl channel exists).
        const sig = layoutSig(L);
        if (sig === lastSentSig) return;
        // Every send below costs a server-side graphics reset (brief blank
        // regions until repainted) — if these lines repeat while the windows
        // are idle, something is jittering the layout into a churn loop.
        // eslint-disable-next-line no-console
        console.log('[monitors] sending screen spec (server graphics reset)', {
          prev: lastSentSig,
          next: sig,
        });
        try {
          const spec = codec.encodeScreenSpecMulti(
            L.bboxWidth,
            L.bboxHeight,
            scaleRef.current,
            L.xs,
            L.ys,
            L.widths,
            L.heights,
            L.primaries
          );
          const buf = spec.buffer as ArrayBuffer;
          worker.postMessage(
            { type: 'outbound', buffer: buf } satisfies WorkerInMsg,
            [buf]
          );
          lastSentSig = sig;
        } catch (e) {
          // eslint-disable-next-line no-console
          console.warn('encodeScreenSpecMulti failed', e);
        }
      }, SPEC_QUIESCENCE_MS);
    }

    function maybeSendClientHello() {
      if (helloSentRef.current) return;
      if (!tdpbReadyRef.current) return;
      const L = pendingLayout;
      if (!L) return;
      const codec = codecRef.current;
      const worker = workerRef.current;
      if (!codec || !worker) return;
      try {
        const hello = codec.encodeClientHelloMulti(
          username,
          L.bboxWidth,
          L.bboxHeight,
          scale,
          keyboardLayout,
          L.xs,
          L.ys,
          L.widths,
          L.heights,
          L.primaries
        );
        const buf = hello.buffer as ArrayBuffer;
        worker.postMessage(
          { type: 'outbound', buffer: buf } satisfies WorkerInMsg,
          [buf]
        );
        helloSentRef.current = true;
        // The hello already carried this layout; record it so a follow-up
        // applyLayout with the same dims doesn't emit a redundant resize.
        lastSentSig = layoutSig(L);
        setStatus({ kind: 'open' });
      } catch (e) {
        // eslint-disable-next-line no-console
        console.warn('client_hello encode failed', e);
      }
    }

    // Watch a window's live position/size; update the model + reflow on change.
    function trackMonitor(id: number, win: Window, trackSize: boolean) {
      untrackers.get(id)?.();
      const stop = topologyRef.current.trackWindow(win, p => {
        if (cancelled) return;
        model.updatePhysical(
          id,
          { left: p.left, top: p.top },
          p.displayId,
          trackSize ? { width: p.width, height: p.height } : undefined
        );
        applyLayout();
      });
      untrackers.set(id, stop);
    }

    // Open one popup window at an explicit physical rect and register it with
    // the model. Shared by manual add and one-click restore. Returns the new
    // monitor id, or null if it couldn't be opened.
    function openPopup(opts: {
      left: number;
      top: number;
      width: number;
      height: number;
      displayId?: string | null;
    }): number | null {
      if (cancelled || model.count() >= maxMonitors) return null;
      if (!popupUrlFactory) {
        // eslint-disable-next-line no-console
        console.warn('addMonitor: no popupUrlFactory provided');
        return null;
      }
      const id = model.allocateId();
      const win = window.open(
        popupUrlFactory(id),
        `codec-test-popup-${id}`,
        `popup=yes,left=${opts.left},top=${opts.top},width=${opts.width},height=${opts.height}`
      );
      model.upsert({
        id,
        role: 'popup',
        cssWidth: opts.width,
        cssHeight: opts.height,
        isPrimary: false,
        physical: { left: opts.left, top: opts.top },
        displayId: opts.displayId ?? null,
        status: win ? 'pending' : 'blocked',
      });
      if (win) {
        popupWindowsRef.current.set(id, win);
        // Don't leave the user's focus on the new popup — they're mid-session
        // in the main window. Browsers always focus a fresh popup and offer
        // no "open in background" for same-origin windows (rel=noopener would
        // sever the postMessage channel), so reclaim right after. Best-effort.
        try {
          window.focus();
        } catch {
          /* ignore */
        }
        canvasRef.current?.focus();
      }
      return id;
    }

    // Estimated popup chrome height: `window.open`'s width/height size the
    // CONTENT area while left/top place the FRAME, so opening into a frame-
    // bounds zone (half/quarter of a display) shaves the chrome off the
    // height. Real chrome is read post-open by the tracker; this only
    // controls the initial fit.
    const POPUP_CHROME_EST = 80;

    function addMonitorImpl(
      display?: DisplayInfo,
      placement?: { left: number; top: number; width: number; height: number }
    ) {
      // A manual add means the user isn't restoring — forget the prior layout.
      dismissRestoreOffer();
      let rect: { left: number; top: number; width: number; height: number };
      if (placement) {
        // Zone picked on the layout map: frame bounds → content size.
        rect = {
          left: placement.left,
          top: placement.top,
          width: placement.width,
          height: Math.max(200, placement.height - POPUP_CHROME_EST),
        };
      } else {
        const desired = {
          width: display
            ? Math.min(display.width, 1920)
            : Math.min(1280, window.innerWidth),
          height: display
            ? Math.min(display.height, 1080)
            : Math.min(720, window.innerHeight),
        };
        // Spawn in the largest free strip beside the MAIN window, shrinking to
        // fit — a desired-size-or-nothing placement always fell back to
        // overlapping the session when the display didn't have a popup-sized
        // hole. The user can re-snap it via the layout panel afterwards.
        const main = model.get(monitorIndex);
        rect = pickSpawnRect({
          main: main?.physical
            ? {
                left: main.physical.left,
                top: main.physical.top,
                width: main.cssWidth,
                height: main.cssHeight,
              }
            : null,
          bounds: display
            ? {
                left: display.availLeft,
                top: display.availTop,
                width: display.width - (display.availLeft - display.left),
                height: display.height - (display.availTop - display.top),
              }
            : null,
          desired,
        });
      }
      const { left, top, width, height } = rect;
      const id = openPopup({ left, top, width, height, displayId: display?.id });
      // Zone adds get trued-up to the exact frame bounds on 'ready'.
      if (placement && id != null && popupWindowsRef.current.get(id)) {
        pendingPlacements.set(id, placement);
      }
      // eslint-disable-next-line no-console
      console.log('[monitors] addMonitor', {
        id,
        display: display?.id ?? null,
        opened: id != null && !!popupWindowsRef.current.get(id),
        count: model.count(),
        maxMonitors,
      });
      pushState();
    }

    // One-click restore of the previous session's popups at their saved rects.
    // Must run from a user gesture (taskbar button) so the window.opens aren't
    // popup-blocked. Best-effort: opens as many as fit under maxMonitors.
    function restoreMonitors() {
      const offer = restoreOffer;
      dismissRestoreOffer();
      if (!offer || cancelled) {
        pushState();
        return;
      }
      const savedPopups = offer.monitors.filter(m => m.role === 'popup');
      const opened: Array<{ id: number; saved: (typeof savedPopups)[number] }> =
        [];
      for (const sp of savedPopups) {
        if (!sp.physical) continue; // no rect -> can't place it
        const id = openPopup({
          left: sp.physical.left,
          top: sp.physical.top,
          width: sp.width,
          height: sp.height,
        });
        if (id != null) opened.push({ id, saved: sp });
      }
      // Restore a non-main primary, then any manual offsets (setManualOffset
      // also flips the model into 'manual' arrangement).
      const primary = opened.find(o => o.saved.isPrimary);
      if (primary) model.setPrimary(primary.id);
      if (offer.arrangement === 'manual') {
        for (const o of opened) {
          if (o.saved.manualOffset)
            model.setManualOffset(o.id, o.saved.manualOffset);
        }
      }
      // eslint-disable-next-line no-console
      console.log('[monitors] restoreMonitors', {
        requested: savedPopups.length,
        opened: opened.length,
      });
      applyLayout();
      pushState();
    }

    function removeMonitorImpl(id: number) {
      if (cancelled) return;
      // A manual remove is a layout change — forget any pending restore offer.
      dismissRestoreOffer();
      // A key held in the closing popup never delivers its keyup anywhere —
      // resync the server's key state on the next key event.
      syncBeforeNextKeyRef.current = true;
      const m = model.get(id);
      if (!m || m.role === 'main') return;
      const win = popupWindowsRef.current.get(id);
      if (win && !win.closed) {
        try {
          win.close();
        } catch {
          /* ignore */
        }
      }
      popupWindowsRef.current.delete(id);
      popupCanvases.delete(id);
      popupViewportsRef.current.delete(id);
      pendingInitialPopups.delete(id);
      untrackers.get(id)?.();
      untrackers.delete(id);
      const canvasId = id + 1;
      if (registered.has(canvasId)) {
        workerRef.current?.postMessage({
          type: 'unregister-canvas',
          canvasId,
        } satisfies WorkerInMsg);
        registered.delete(canvasId);
      }
      model.remove(id);
      applyLayout();
    }

    // Expose the controller to the management UI (taskbar + panel).
    controllerRef.current = {
      getState: () => currentState(),
      subscribe: cb => {
        cb(currentState());
        return () => undefined;
      },
      addMonitor: (display, placement) => addMonitorImpl(display, placement),
      removeMonitor: id => removeMonitorImpl(id),
      restoreMonitors: () => restoreMonitors(),
      setPrimary: id => {
        model.setPrimary(id);
        applyLayout();
      },
      setManualOffset: (id, offset) => {
        model.setManualOffset(id, offset);
        applyLayout();
      },
      moveMonitorWindow: (id, pos) => {
        const win = popupWindowsRef.current.get(id);
        if (!win || win.closed) return;
        try {
          // `moveTo` positions the window FRAME; `pos` is the desired CONTENT
          // position, so subtract the chrome height (chrome sits above the
          // viewport — same assumption as contentScreenPosition).
          const chromeY = Math.max(0, win.outerHeight - win.innerHeight);
          win.moveTo(Math.round(pos.left), Math.round(pos.top - chromeY));
        } catch {
          /* window mid-close */
        }
        // Immediate map feedback; the topology tracker confirms (and corrects
        // for any OS clamping of moveTo) on its next poll.
        model.updatePhysical(id, pos);
        applyLayout();
      },
      setMonitorWindowBounds: (id, bounds) => {
        const win = popupWindowsRef.current.get(id);
        if (!win || win.closed) return;
        let chromeY = 0;
        try {
          chromeY = Math.max(0, win.outerHeight - win.innerHeight);
          // FRAME bounds, like a real OS snap: the window incl. its chrome
          // fills the zone; the content is the zone minus the chrome.
          win.moveTo(Math.round(bounds.left), Math.round(bounds.top));
          win.resizeTo(Math.round(bounds.width), Math.round(bounds.height));
        } catch {
          /* window mid-close */
        }
        // Immediate feedback with the implied content rect; the tracker
        // confirms and the popup's resize flow updates its canvas + the RDP
        // resolution.
        model.updatePhysical(
          id,
          { left: bounds.left, top: bounds.top + chromeY },
          undefined,
          { width: bounds.width, height: Math.max(1, bounds.height - chromeY) }
        );
        applyLayout();
      },
      setArrangement: mode => {
        model.setArrangement(mode);
        applyLayout();
      },
      tidy: () => {
        model.tidy();
        applyLayout();
      },
      beginMonitorInteraction: () => {
        layoutHold = true;
      },
      endMonitorInteraction: () => {
        layoutHold = false;
        // Commit the final layout once.
        applyLayout();
      },
    };
    // Track the main window's position AND size, so moving it re-arranges the
    // layout and resizing it updates the primary's resolution (the wasm painter
    // resizes the main canvas backing buffer on reposition, same as popups).
    trackMonitor(monitorIndex, window, true);
    pushState();

    function scheduleSecondaryRefresh(bboxW: number, bboxH: number) {
      for (const delay of [1500, 4000, 8000, 15000, 25000]) {
        setTimeout(() => {
          if (cancelled) return;
          const c = codecRef.current;
          const w = workerRef.current;
          if (!c || !w) return;
          try {
            const refresh = c.encodeRefreshRect(
              0,
              0,
              Math.max(0, bboxW - 1),
              Math.max(0, bboxH - 1)
            );
            const rbuf = refresh.buffer as ArrayBuffer;
            w.postMessage(
              { type: 'outbound', buffer: rbuf } satisfies WorkerInMsg,
              [rbuf]
            );
          } catch {
            /* ignore */
          }
        }, delay);
      }
    }

    function forwardCursorToPopups(m: WorkerOutMsg) {
      // Cursor follows the mouse across monitors, so each popup mirrors
      // main's cursor state. We forward the raw message — the popup's
      // handler accepts the same shape we receive from the worker.
      for (const [, win] of popupWindowsRef.current) {
        if (!win || win.closed) continue;
        try {
          win.postMessage(m, '*');
        } catch {
          /* popup may be closing */
        }
      }
    }

    function handleWorkerMessage(
      m:
        | WorkerOutMsg
        | {
            type: 'perf';
            paints: number;
            elapsed_ms: number;
            busy_ms: number;
            apply_clear_ms: number;
            apply_c2s_ms: number;
            apply_s2c_ms: number;
            apply_rfx_ms: number;
            apply_other_ms: number;
            // Present only when the wasm is built with `rfx-stage-timing`
            // (within-rfx decode breakdown — Step 1 measurement).
            rfx_entropy_ms?: number;
            rfx_idwt_ms?: number;
            rfx_color_ms?: number;
            rfx_blit_ms?: number;
          }
    ) {
      // The wasm posts a `perf` payload ~once/sec (see perf.rs). Derive FPS and
      // worker CPU%, and log the per-op CPU breakdown to find the dominant op.
      if (m.type === 'perf') {
        if (m.elapsed_ms > 0) {
          setFps((m.paints * 1000) / m.elapsed_ms);
          setCpu((m.busy_ms * 100) / m.elapsed_ms);
        }
        // When the wasm was built with `rfx-stage-timing`, break the rfx bucket
        // into entropy / inverse-DWT / color / blit (a within-rfx breakdown, not
        // a partition: decode dispatch + dirty/probe overhead are excluded, so
        // the parts sum to less than rfx).
        const rfxBreakdown =
          m.rfx_entropy_ms !== undefined
            ? ` | rfx{entropy=${m.rfx_entropy_ms.toFixed(0)} idwt=${(m.rfx_idwt_ms ?? 0).toFixed(0)} ` +
              `color=${(m.rfx_color_ms ?? 0).toFixed(0)} blit=${(m.rfx_blit_ms ?? 0).toFixed(0)}}`
            : '';
        // eslint-disable-next-line no-console
        console.log(
          `[apply] clear=${m.apply_clear_ms.toFixed(0)} c2s=${m.apply_c2s_ms.toFixed(0)} ` +
            `s2c=${m.apply_s2c_ms.toFixed(0)} rfx=${m.apply_rfx_ms.toFixed(0)} ` +
            `other=${m.apply_other_ms.toFixed(0)} ms (busy=${m.busy_ms.toFixed(0)}/${m.elapsed_ms.toFixed(0)})` +
            rfxBreakdown
        );
        return;
      }
      switch (m.type) {
        case 'state':
          // eslint-disable-next-line no-console
          console.log(
            `[worker-state][${mode}#${monitorIndex}] state=${m.state}`,
            m
          );
          if (m.state === 'error') {
            setStatus({
              kind: 'error',
              message: m.message ?? 'worker error',
            });
          } else if (m.state === 'closed' && status.kind === 'open') {
            setStatus({ kind: 'idle' });
          }
          // 'open' state alone doesn't flip the UI — we still need TDPB
          // upgrade + ClientHello before the session is usable.
          break;
        case 'log':
          // eslint-disable-next-line no-console
          console.log(`[decoder][${mode}#${monitorIndex}]`, m.text);
          break;
        case 'decoded':
          // eslint-disable-next-line no-console
          console.log(`[inbound][${mode}#${monitorIndex}]`, m.text);
          break;
        case 'resolution':
          // The server's desktop size — i.e. the bounding box it actually
          // allocated. After adding/moving a monitor this should grow to match
          // our requested bbox; if it doesn't, the server rejected the resize.
          // eslint-disable-next-line no-console
          console.log('[monitors] server desktop resolution', {
            width: m.width,
            height: m.height,
          });
          break;
        case 'cursorBitmap':
          cursorSpriteRef.current = applyCursorBitmap(canvas, m);
          forwardCursorToPopups(m);
          break;
        case 'cursorHidden':
          canvas.style.cursor = 'none';
          forwardCursorToPopups(m);
          break;
        case 'cursorDefault':
          canvas.style.cursor = 'default';
          forwardCursorToPopups(m);
          break;
        case 'tdpbUpgrade': {
          const codec = codecRef.current;
          if (!codec) break;
          codec.upgradeToTdpb();
          tdpbReadyRef.current = true;
          maybeSendClientHello();
          break;
        }
        case 'slowPdu':
          // eslint-disable-next-line no-console
          console.log(
            `[slow-pdu][${mode}#${monitorIndex}] ${m.class} ${m.ms.toFixed(1)}ms len=${m.len}`
          );
          break;
      }
    }

    // Detect popups the user closes (initial or added mid-session) and tear
    // them down (free their canvas + shrink the RDP desktop). Always armed.
    closePollId = setInterval(() => {
      if (cancelled) return;
      for (const [idx, win] of Array.from(popupWindowsRef.current.entries())) {
        if (win.closed) removeMonitorImpl(idx);
      }
    }, 1000);

    return () => {
      cancelled = true;
      if (closePollId !== undefined) clearInterval(closePollId);
      if (specDebounce !== undefined) clearTimeout(specDebounce);
      for (const stop of untrackers.values()) stop();
      untrackers.clear();
      controllerRef.current = null;
      window.removeEventListener('message', handlePopupMessage);
      window.removeEventListener('pagehide', closeAllPopups);
      closeAllPopups();
      popupViewportsRef.current.clear();
      const worker = workerRef.current;
      if (worker) {
        try {
          worker.postMessage({ type: 'unregister' } satisfies WorkerInMsg);
          worker.terminate();
        } catch {
          /* ignore */
        }
        workerRef.current = null;
      }
      codecRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [params]);

  function broadcastToPopups(msg: unknown) {
    for (const [, win] of popupWindowsRef.current) {
      if (win && !win.closed) {
        try {
          win.postMessage(msg, '*');
        } catch {
          /* popup closing */
        }
      }
    }
  }

  // Pointer-lock capture mode. While the main canvas holds the lock, all mouse
  // motion arrives as relative deltas (movementX/Y) which we accumulate into a
  // single virtual-desktop cursor, clamp to the bounding box, and forward to
  // the server. Popups are told to go display-only and to render a synthetic
  // cursor sprite at the broadcast virtual position. The lock itself drops on
  // any Esc (browser-forced), but capture stays ENGAGED — see enterCaptureMode.
  useEffect(() => {
    let onMove: ((ev: MouseEvent) => void) | undefined;
    let onDown: ((ev: MouseEvent) => void) | undefined;
    let onUp: ((ev: MouseEvent) => void) | undefined;
    let onWheel: ((ev: WheelEvent) => void) | undefined;

    const positionMainSprite = () => {
      const el = captureSpriteElRef.current;
      const le = layoutEntryRef.current;
      const sprite = cursorSpriteRef.current;
      if (!el || !le) return;
      const { x, y } = virtualCursorRef.current;
      const lx = x - le.x;
      const ly = y - le.y;
      if (sprite && lx >= 0 && lx <= le.width && ly >= 0 && ly <= le.height) {
        if (el.getAttribute('src') !== sprite.url) el.src = sprite.url;
        el.style.display = 'block';
        el.style.transform = `translate(${lx - sprite.hotspotX * sprite.scale}px, ${ly - sprite.hotspotY * sprite.scale}px)`;
      } else {
        el.style.display = 'none';
      }
    };

    const onChange = () => {
      const locked = document.pointerLockElement === canvasRef.current;
      setCaptureMode(locked);
      // NOTE: popup capture-on/off broadcasts live in enterCaptureMode /
      // exitCaptureMode (engaged transitions), NOT here — a browser-forced
      // Esc unlock mid-capture must leave popups display-only.
      if (locked) {
        const sr = Math.max(1, scaleRef.current) / 100;
        onMove = ev => {
          const bb = bboxRef.current;
          if (!bb) return;
          const vc = virtualCursorRef.current;
          vc.x = Math.max(0, Math.min(bb.width, vc.x + ev.movementX));
          vc.y = Math.max(0, Math.min(bb.height, vc.y + ev.movementY));
          processMouseMove(Math.round(vc.x * sr), Math.round(vc.y * sr));
          positionMainSprite();
          broadcastToPopups({ type: 'captureCursor', vx: vc.x, vy: vc.y });
        };
        const buttonOf = (ev: MouseEvent) =>
          (ev.button === 1 ? 1 : ev.button === 2 ? 2 : 0) as 0 | 1 | 2;
        onDown = ev => {
          ev.preventDefault();
          const vc = virtualCursorRef.current;
          processMouseButton(
            buttonOf(ev),
            true,
            Math.round(vc.x * sr),
            Math.round(vc.y * sr)
          );
        };
        onUp = ev => {
          const vc = virtualCursorRef.current;
          processMouseButton(
            buttonOf(ev),
            false,
            Math.round(vc.x * sr),
            Math.round(vc.y * sr)
          );
        };
        onWheel = ev => {
          ev.preventDefault();
          // Only pixel-precision deltas (trackpads, modern mice). Ignore
          // line/page mode so a stray legacy event can't lurch the remote.
          if (ev.deltaMode !== 0) return;
          processMouseWheel(ev.deltaX, ev.deltaY);
        };
        document.addEventListener('mousemove', onMove, true);
        document.addEventListener('mousedown', onDown, true);
        document.addEventListener('mouseup', onUp, true);
        document.addEventListener('wheel', onWheel, {
          capture: true,
          passive: false,
        });
        positionMainSprite();
      } else {
        if (onMove) document.removeEventListener('mousemove', onMove, true);
        if (onDown) document.removeEventListener('mousedown', onDown, true);
        if (onUp) document.removeEventListener('mouseup', onUp, true);
        if (onWheel) document.removeEventListener('wheel', onWheel, true);
        onMove = onDown = onUp = onWheel = undefined;
        if (captureSpriteElRef.current) {
          captureSpriteElRef.current.style.display = 'none';
        }
      }
    };

    document.addEventListener('pointerlockchange', onChange);
    return () => {
      document.removeEventListener('pointerlockchange', onChange);
      if (onMove) document.removeEventListener('mousemove', onMove, true);
      if (onDown) document.removeEventListener('mousedown', onDown, true);
      if (onUp) document.removeEventListener('mouseup', onUp, true);
      if (onWheel) document.removeEventListener('wheel', onWheel, true);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [scale]);

  // Native, non-passive canvas listeners for the latency-sensitive hot path.
  // We bypass React's synthetic-event system for `mousemove` (the highest-
  // frequency input) and `wheel`. `wheel` additionally REQUIRES a non-passive
  // listener: React's `onWheel` is passive, so `preventDefault()` would be a
  // no-op and the browser would scroll/zoom the page instead. Capture phase so
  // we run ahead of any bubble handler; rebinds on scale/captureMode change so
  // the values closed over here stay fresh. Buttons and keys deliberately stay
  // on the React handlers — they're low-frequency and tangled with paste/drag/
  // suppression logic, where per-event synthetic-event overhead is moot.
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const onMove = (e: MouseEvent) => {
      // In capture mode the document-level lock handler drives the cursor;
      // during a drag the window-level handler installed by beginDrag does.
      // Engaged-but-unlocked (sticky capture, browser Esc drop) also skips:
      // absolute coords would teleport the virtual cursor.
      if (captureMode || captureEngagedRef.current || dragActiveRef.current)
        return;
      const layoutEntry = layoutEntryRef.current;
      if (!layoutEntry) return;
      const { x, y } = canvasToVirtualCoords(
        e.clientX,
        e.clientY,
        canvas,
        layoutEntry,
        scaleRef.current
      );
      processMouseMove(x, y);
    };
    const onWheel = (e: WheelEvent) => {
      e.preventDefault();
      // Capture mode routes wheel through the document-level listener.
      if (captureMode) return;
      // Only pixel-precision deltas (trackpads, modern mice). Ignore line/page
      // mode so a stray legacy event can't lurch the remote.
      if (e.deltaMode !== 0) return;
      processMouseWheel(e.deltaX, e.deltaY);
    };
    canvas.addEventListener('mousemove', onMove, { capture: true });
    canvas.addEventListener('wheel', onWheel, {
      capture: true,
      passive: false,
    });
    return () => {
      canvas.removeEventListener('mousemove', onMove, true);
      canvas.removeEventListener('wheel', onWheel, true);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [scale, captureMode]);

  function send(buffer: Uint8Array) {
    const worker = workerRef.current;
    if (!worker) return;
    const buf = buffer.buffer as ArrayBuffer;
    worker.postMessage(
      { type: 'outbound', buffer: buf } satisfies WorkerInMsg,
      [buf]
    );
  }

  // -- Input processors --
  // These take pre-translated virtual-desktop coords / raw key codes and
  // emit codec-encoded bytes via `send`. The same processors run for both
  // main-window React events and forwarded popup messages — popup
  // pre-computes virtual coords on its side using its layoutEntry.

  function processMouseMove(virtualX: number, virtualY: number) {
    if (statusRef.current.kind !== 'open') return;
    const codec = codecRef.current;
    if (!codec) return;
    // Remember the last forwarded position in CSS-px bbox space (inputs here
    // are physical px), so entering capture mode can seed its virtual cursor
    // from where the pointer actually is instead of teleporting to center.
    const sr = Math.max(1, scaleRef.current) / 100;
    lastPointerCssRef.current = { x: virtualX / sr, y: virtualY / sr };
    try {
      send(codec.encodeMouseMove(virtualX, virtualY));
    } catch {
      /* codec mid-teardown */
    }
  }

  function processMouseButton(
    button: 0 | 1 | 2,
    pressed: boolean,
    virtualX: number,
    virtualY: number
  ) {
    if (statusRef.current.kind !== 'open') return;
    const codec = codecRef.current;
    if (!codec) return;
    try {
      send(codec.encodeMouseButton(button, pressed));
    } catch {
      /* ignore */
    }
    // Drag-refresh: after a left-button drag that moved beyond a tiny
    // threshold, ask the server to repaint the whole desktop. Catches
    // window-move / scroll-jump regions Windows sometimes leaves stale.
    const DRAG_REFRESH_THRESHOLD_PX = 4;
    if (button === 0) {
      if (pressed) {
        dragOriginRef.current = { x: virtualX, y: virtualY };
        return;
      }
      const origin = dragOriginRef.current;
      dragOriginRef.current = null;
      if (!origin) return;
      const dx = Math.abs(virtualX - origin.x);
      const dy = Math.abs(virtualY - origin.y);
      if (dx <= DRAG_REFRESH_THRESHOLD_PX && dy <= DRAG_REFRESH_THRESHOLD_PX) {
        return;
      }
      const bb = bboxRef.current;
      if (!bb) return;
      try {
        send(
          codec.encodeRefreshRect(
            0,
            0,
            Math.max(0, bb.width - 1),
            Math.max(0, bb.height - 1)
          )
        );
      } catch {
        /* ignore */
      }
    }
  }

  // Forward a wheel event. `deltaX`/`deltaY` are raw browser `WheelEvent`
  // pixel deltas (the caller has already filtered to DOM_DELTA_PIXEL). We
  // negate to match the remote scroll direction (same convention as the
  // production TDP client) and carry the sub-pixel remainder so trackpad
  // micro-scrolls aren't rounded to zero. The server applies the wheel at the
  // last cursor position, so no coordinates travel with the event.
  function processMouseWheel(deltaX: number, deltaY: number) {
    if (statusRef.current.kind !== 'open') return;
    const codec = codecRef.current;
    if (!codec) return;
    // Axis codes mirror the Rust ScrollAxis enum: 0 vertical, 1 horizontal.
    const VERTICAL = 0;
    const HORIZONTAL = 1;
    const acc = wheelAccumRef.current;
    acc.y += -deltaY;
    acc.x += -deltaX;
    const sendY = Math.trunc(acc.y);
    const sendX = Math.trunc(acc.x);
    acc.y -= sendY;
    acc.x -= sendX;
    try {
      if (sendY !== 0) send(codec.encodeMouseWheel(VERTICAL, sendY));
      if (sendX !== 0) send(codec.encodeMouseWheel(HORIZONTAL, sendX));
    } catch {
      /* codec mid-teardown */
    }
  }

  function processKey(code: string, pressed: boolean) {
    if (statusRef.current.kind !== 'open') return;
    const codec = codecRef.current;
    if (!codec) return;
    const scancodes = KEY_SCANCODES[code];
    if (!scancodes) return;
    // Track which modifiers the SERVER currently believes are held (fed by
    // every source: main keys, popup relays, paste synthetics) so
    // releaseStuckRemoteMods can free ones the user no longer holds.
    if (REMOTE_MOD_CODES.has(code)) {
      if (pressed) remoteModsRef.current.add(code);
      else remoteModsRef.current.delete(code);
    }
    for (const sc of scancodes) {
      try {
        send(codec.encodeKey(sc, pressed));
      } catch {
        /* ignore */
      }
    }
  }

  // --- Stuck-modifier defenses (ports of the production client's Withholder
  // and modifier reconciliation, InputHandler.tsx / Withholder.tsx) ---

  // Meta/Alt keydowns are WITHHELD: only forwarded when a following
  // non-modifier key proves this is a chord typed INTO the session. For an
  // OS-level chord (Cmd+Tab, Cmd+Space, Cmd+Shift+5 screen recording) the
  // trailing key never reaches the page — previously the already-forwarded
  // Meta down latched LWIN on the server when the OS swallowed the keyup
  // (teleport#24342, and the recurring stuck-Win bug in this harness).
  const withheldModsRef = useRef<string[]>([]);
  // Server-side held state for the eight modifier keys.
  const remoteModsRef = useRef(new Set<string>());

  function flushWithheldMods() {
    for (const code of withheldModsRef.current) processKey(code, true);
    withheldModsRef.current = [];
  }

  // Release-only reconciliation before every forwarded non-modifier key:
  // any modifier the server holds that the user is NOT physically holding
  // gets released. Self-heals stuck modifiers (e.g. Shift after Cmd+Shift+5,
  // whose keyup the macOS recorder HUD swallowed) on the next keystroke,
  // even when no blur event ever fired to arm a SyncKeys.
  function releaseStuckRemoteMods(e: React.KeyboardEvent<HTMLCanvasElement>) {
    // AltGr synthesizes Ctrl+Alt while getModifierState reports both false —
    // releasing them mid-AltGr would corrupt the chord (production parity).
    const altGraph = e.getModifierState('AltGraph');
    for (const [mod, left, right] of [
      ['Shift', 'ShiftLeft', 'ShiftRight'],
      ['Control', 'ControlLeft', 'ControlRight'],
      ['Alt', 'AltLeft', 'AltRight'],
      ['Meta', 'MetaLeft', 'MetaRight'],
    ] as const) {
      if (altGraph && (mod === 'Control' || mod === 'Alt')) continue;
      if (e.getModifierState(mod)) continue;
      if (remoteModsRef.current.has(left)) processKey(left, false);
      if (remoteModsRef.current.has(right)) processKey(right, false);
    }
  }

  // Armed: the next key event is prefixed with a SyncKeys message. The server
  // turns it into an RDP Synchronize Event, which per MS-RDPBCGR resets the
  // server key state to ALL KEYS UP before re-applying the lock keys — the
  // recovery path for a modifier the server still believes is held because
  // its keyup never reached us (a Cmd chord that moved focus away, a popup
  // closing mid-press). Starts true: Windows sessions keep held keys latched
  // across RDP reconnects, so the first key of a new connection must heal
  // whatever a previous session left behind. Re-armed on every focus loss
  // (main canvas blur, popup blur, popup close). Mirrors the production
  // client's InputHandler.syncBeforeNextKey.
  const syncBeforeNextKeyRef = useRef(true);

  function maybeSyncKeys(locks: {
    scrollLock: boolean;
    numLock: boolean;
    capsLock: boolean;
  }) {
    if (!syncBeforeNextKeyRef.current) return;
    if (statusRef.current.kind !== 'open') return;
    const codec = codecRef.current;
    if (!codec) return;
    try {
      // KanaLock hardcoded up, as in production encodeSyncKeys (not a
      // browser-queryable modifier).
      send(
        codec.encodeSyncKeys(
          locks.scrollLock,
          locks.numLock,
          locks.capsLock,
          false
        )
      );
      syncBeforeNextKeyRef.current = false;
    } catch {
      /* ignore — stays armed for the next key event */
    }
  }

  function lockStatesFrom(e: React.KeyboardEvent<HTMLCanvasElement>) {
    return {
      scrollLock: e.getModifierState('ScrollLock'),
      numLock: e.getModifierState('NumLock'),
      capsLock: e.getModifierState('CapsLock'),
    };
  }

  // Window-level blur — the canvas may not be the active element when the OS
  // chord steals focus (e.g. Cmd+Shift+5's recorder HUD): arm the next-key
  // resync and discard withheld modifiers so they're never delivered late.
  useEffect(() => {
    const onWindowBlur = () => {
      syncBeforeNextKeyRef.current = true;
      withheldModsRef.current = [];
    };
    window.addEventListener('blur', onWindowBlur);
    return () => window.removeEventListener('blur', onWindowBlur);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Ships clipboard text to the server then synthesizes Ctrl+V so Windows
  // actually pastes. Mac's Cmd (sent earlier as LWIN) is released first so
  // Windows reads the synthetic Ctrl+V as a clean shortcut rather than
  // Win+Ctrl+V.
  async function processPaste(text: string) {
    if (status.kind !== 'open' || !text) return;
    const codec = codecRef.current;
    if (!codec) return;
    processKey('MetaLeft', false);
    processKey('MetaRight', false);
    try {
      send(codec.encodeClipboard(text));
    } catch (e) {
      // eslint-disable-next-line no-console
      console.warn('encodeClipboard failed:', e);
      return;
    }
    // Brief wait so Windows clipboard updates before the synthetic Ctrl+V.
    await new Promise(r => setTimeout(r, 80));
    processKey('ControlLeft', true);
    processKey('KeyV', true);
    processKey('KeyV', false);
    processKey('ControlLeft', false);
  }

  // Tracks keys whose down-event we intercepted so the matching keyup
  // isn't forwarded either (Windows would otherwise see a release for a
  // key it never saw pressed).
  const suppressedKeysRef = useRef(new Set<string>());

  async function performPasteFromHost() {
    if (statusRef.current.kind !== 'open') return;
    let text = '';
    try {
      text = await navigator.clipboard.readText();
    } catch (e) {
      // eslint-disable-next-line no-console
      console.warn('clipboard.readText failed (browser denied?):', e);
      return;
    }
    if (!text) return;
    await processPaste(text);
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLCanvasElement>) {
    if (statusRef.current.kind !== 'open') return;
    maybeSyncKeys(lockStatesFrom(e));
    if (captureEngagedRef.current) {
      // Capture-exit chord — deliberate enough that nothing in a normal
      // session hits it. The Esc down is suppressed (its keyup too, via the
      // suppressed set); the modifier downs already reached the server and
      // their releases follow naturally when the user lets go.
      if (e.ctrlKey && e.altKey && e.shiftKey && e.code === 'Escape') {
        e.preventDefault();
        suppressedKeysRef.current.add(e.code);
        exitCaptureMode();
        return;
      }
      // Sticky capture: the browser dropped the lock (bare Esc / overlay
      // dialog), and this keydown is a user gesture — re-acquire, then
      // forward the key below as usual.
      if (document.pointerLockElement !== canvasRef.current) {
        resumeCapture();
      }
    }
    if ((e.metaKey || e.ctrlKey) && e.code === 'KeyV') {
      e.preventDefault();
      suppressedKeysRef.current.add(e.code);
      // The held Cmd is consumed by the paste — never deliver its withheld
      // down, or the NEXT keystroke would flush a stale Win-down.
      withheldModsRef.current = withheldModsRef.current.filter(
        c => !WITHHELD_MOD_CODES.has(c)
      );
      void performPasteFromHost();
      return;
    }
    if (!KEY_SCANCODES[e.code]) return;
    if (WITHHELD_MOD_CODES.has(e.code)) {
      e.preventDefault();
      if (!withheldModsRef.current.includes(e.code)) {
        withheldModsRef.current.push(e.code);
      }
      return;
    }
    e.preventDefault();
    if (!PLAIN_MOD_CODES.has(e.code)) {
      // A real key typed into the session: free anything stuck, then commit
      // the withheld modifiers ahead of it (chord order preserved).
      releaseStuckRemoteMods(e);
      flushWithheldMods();
    }
    processKey(e.code, true);
  }

  function handleKeyUp(e: React.KeyboardEvent<HTMLCanvasElement>) {
    if (statusRef.current.kind !== 'open') return;
    maybeSyncKeys(lockStatesFrom(e));
    if (withheldModsRef.current.includes(e.code)) {
      // The down was never sent (bare Cmd tap / OS chord) — swallow the up.
      e.preventDefault();
      withheldModsRef.current = withheldModsRef.current.filter(
        c => c !== e.code
      );
      return;
    }
    if (suppressedKeysRef.current.delete(e.code)) {
      e.preventDefault();
      return;
    }
    if (!KEY_SCANCODES[e.code]) return;
    e.preventDefault();
    processKey(e.code, false);
  }

  // Normal-mode mousemove is handled by the native capture-phase listener
  // attached above (see the `[scale, captureMode]` effect), not a React prop.

  function handleMouseButton(pressed: boolean) {
    return (e: React.MouseEvent<HTMLCanvasElement>) => {
      e.preventDefault();
      if (captureMode) return; // pointer-lock capture handles input itself
      if (captureEngagedRef.current) {
        // Engaged but unlocked (browser Esc drop): consume the click to
        // re-acquire the lock — forwarding its absolute coords would
        // teleport the virtual-desktop cursor. The taskbar shows a
        // "capture paused" pill in this state explaining the behavior.
        if (pressed) resumeCapture();
        return;
      }
      // A click is also chord-intent (Cmd+click into the session): deliver
      // any withheld modifiers ahead of the button event.
      if (pressed) flushWithheldMods();
      const layoutEntry = layoutEntryRef.current;
      if (!layoutEntry) return;
      const button = (e.button === 1 ? 1 : e.button === 2 ? 2 : 0) as
        | 0
        | 1
        | 2;
      if (pressed) {
        if (dragActiveRef.current) return;
        suppressMouseUpRef.current = false;
        const { x, y } = canvasToVirtualCoords(
          e.clientX,
          e.clientY,
          canvasRef.current!,
          layoutEntry,
          scaleRef.current
        );
        processMouseButton(button, true, x, y);
        beginDrag();
      } else {
        if (dragActiveRef.current) return;
        // The window listener in beginDrag already released this drag and set
        // suppressMouseUpRef, so skip the bubble-phase duplicate.
        if (suppressMouseUpRef.current) {
          suppressMouseUpRef.current = false;
          return;
        }
        const { x, y } = canvasToVirtualCoords(
          e.clientX,
          e.clientY,
          canvasRef.current!,
          layoutEntry,
          scaleRef.current
        );
        processMouseButton(button, false, x, y);
      }
    };
  }

  // Track a button-held drag at the window level so the cursor keeps mapping to
  // whole-desktop virtual coords after it leaves the canvas (across the seam
  // into a popup). The OS routes mousemove to the window where the drag began
  // until release, so canvasToVirtualCoords extrapolates past the canvas bounds.
  function beginDrag() {
    if (dragActiveRef.current) return;
    dragActiveRef.current = true;
    const onMove = (ev: MouseEvent) => {
      const layoutEntry = layoutEntryRef.current;
      const canvas = canvasRef.current;
      if (!layoutEntry || !canvas) return;
      const { x, y } = canvasToVirtualCoords(
        ev.clientX,
        ev.clientY,
        canvas,
        layoutEntry,
        scaleRef.current
      );
      processMouseMove(x, y);
    };
    const onUp = (ev: MouseEvent) => {
      const layoutEntry = layoutEntryRef.current;
      const canvas = canvasRef.current;
      if (layoutEntry && canvas) {
        const { x, y } = canvasToVirtualCoords(
          ev.clientX,
          ev.clientY,
          canvas,
          layoutEntry,
          scaleRef.current
        );
        const button = (ev.button === 1 ? 1 : ev.button === 2 ? 2 : 0) as
          | 0
          | 1
          | 2;
        processMouseButton(button, false, x, y);
      }
      window.removeEventListener('mousemove', onMove, true);
      window.removeEventListener('mouseup', onUp, true);
      dragActiveRef.current = false;
      suppressMouseUpRef.current = true;
    };
    window.addEventListener('mousemove', onMove, true);
    window.addEventListener('mouseup', onUp, true);
  }

  // Enter pointer-lock capture mode. Must be called from a user gesture
  // (the taskbar "Capture input" button). Capture is STICKY: the browser
  // force-exits pointer lock on any Esc press (not rebindable outside
  // fullscreen keyboard-lock), so a bare Esc — common in remote apps — only
  // drops the raw lock. While `captureEngagedRef` stays true we re-acquire
  // the lock on the next keystroke or click (both count as user gestures);
  // only the explicit chord (Ctrl+Alt+Shift+Esc) truly exits.
  function enterCaptureMode() {
    const canvas = canvasRef.current;
    if (!canvas) return;
    // Seed the captured cursor from where the pointer actually was (last
    // position forwarded to the server, main or popup) so engaging capture
    // doesn't teleport the remote cursor. Center is only the cold-start
    // fallback before any mouse motion was seen.
    const last = lastPointerCssRef.current;
    const le = layoutEntryRef.current;
    const bb = bboxRef.current;
    if (last) {
      virtualCursorRef.current = { x: last.x, y: last.y };
    } else if (le) {
      virtualCursorRef.current = {
        x: le.x + le.width / 2,
        y: le.y + le.height / 2,
      };
    } else if (bb) {
      virtualCursorRef.current = { x: bb.width / 2, y: bb.height / 2 };
    }
    captureEngagedRef.current = true;
    setCaptureEngaged(true);
    // Popups go display-only for the whole engaged period (including
    // momentary Esc-induced unlock gaps), not just while the lock is held.
    broadcastToPopups({ type: 'capture', active: true });
    // Keyboard focus must be on the canvas for the key handlers (incl. the
    // exit chord) — the click that got us here may have focused a button.
    canvas.focus();
    canvas.requestPointerLock?.();
  }

  // Re-acquire the lock after a browser-forced drop (bare Esc). Unlike
  // enterCaptureMode this keeps the virtual cursor exactly where it was, so
  // resuming never moves the remote pointer.
  function resumeCapture() {
    const canvas = canvasRef.current;
    if (!canvas) return;
    canvas.focus();
    canvas.requestPointerLock?.();
  }

  function exitCaptureMode() {
    captureEngagedRef.current = false;
    setCaptureEngaged(false);
    broadcastToPopups({ type: 'capture', active: false });
    if (document.pointerLockElement === canvasRef.current) {
      document.exitPointerLock?.();
    }
  }

  function handleMouseLeave(e: React.MouseEvent<HTMLCanvasElement>) {
    if (statusRef.current.kind !== 'open') return;
    if (captureMode || captureEngagedRef.current || dragActiveRef.current)
      return;
    const canvas = canvasRef.current;
    const layoutEntry = layoutEntryRef.current;
    if (!canvas || !layoutEntry) return;
    const rect = canvas.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) return;
    // Bridge the cross-window pointer-capture gap: send one extra mousemove
    // just past the canvas boundary in the direction of exit so the server
    // sees continuous motion across the monitor seam (avoids Aero Snap).
    // layoutEntry / NUDGE_PX are in CSS pixels; convert to server-side
    // physical pixels at the very end via scaleRatio.
    const NUDGE_PX = 8;
    let outX = layoutEntry.x;
    let outY = layoutEntry.y;
    if (e.clientX <= rect.left) outX = layoutEntry.x - NUDGE_PX;
    else if (e.clientX >= rect.right - 1)
      outX = layoutEntry.x + layoutEntry.width + NUDGE_PX - 1;
    else
      outX =
        layoutEntry.x +
        Math.round(((e.clientX - rect.left) * layoutEntry.width) / rect.width);
    if (e.clientY <= rect.top) outY = layoutEntry.y - NUDGE_PX;
    else if (e.clientY >= rect.bottom - 1)
      outY = layoutEntry.y + layoutEntry.height + NUDGE_PX - 1;
    else
      outY =
        layoutEntry.y +
        Math.round(
          ((e.clientY - rect.top) * layoutEntry.height) / rect.height
        );
    outX = Math.max(0, outX);
    outY = Math.max(0, outY);
    const scaleRatio = Math.max(1, scaleRef.current) / 100;
    processMouseMove(
      Math.round(outX * scaleRatio),
      Math.round(outY * scaleRatio)
    );
  }

  // HiDPI mid-stream toggle (AVC probe): flip the live scale between 1x and the
  // device DPR, then re-run the layout apply so the host re-renders at the new
  // resolution (canvas repositions + DisplayControl screenspec). Watch the node
  // log's `capabilities confirmed` / `[avc]` to see whether AVC survives the
  // resolution bump.
  function toggleHidpi() {
    const dpr = Math.max(100, Math.round((window.devicePixelRatio ?? 1) * 100));
    const next = scaleRef.current > 100 ? 100 : dpr;
    scaleRef.current = next;
    setHidpiOn(next > 100);
    // eslint-disable-next-line no-console
    console.log(
      `[hidpi-toggle] scale=${next} (${next > 100 ? 'HiDPI on' : '1x'}) — re-sending layout`
    );
    reflowRef.current?.();
  }

  return (
    <div style={outerWrapperStyle}>
      <canvas
        ref={c => {
          canvasRef.current = c;
          // Autofocus so keys flow to the canvas without an explicit click.
          if (c) c.focus();
        }}
        tabIndex={0}
        onMouseDown={handleMouseButton(true)}
        onMouseUp={handleMouseButton(false)}
        onMouseLeave={handleMouseLeave}
        onKeyDown={handleKeyDown}
        onKeyUp={handleKeyUp}
        // Any key still down when focus leaves the canvas (OS chord like
        // Cmd+Tab, or just clicking the taskbar overlay) may never deliver
        // its keyup here — resync the server's key state on the next key,
        // and drop withheld modifiers so they're never delivered late.
        onBlur={() => {
          syncBeforeNextKeyRef.current = true;
          withheldModsRef.current = [];
        }}
        onContextMenu={e => e.preventDefault()}
        style={{
          width: '100%',
          height: '100%',
          display: 'block',
          outline: 'none',
          imageRendering: 'pixelated',
        }}
      />
      {mode === 'main' && (
        <>
          <MonitorTaskbar
            monitorCount={sessionState?.monitors.length ?? monitors.length}
            maxMonitors={maxMonitors}
            fps={fps}
            cpu={cpu}
            statusLabel={status.kind !== 'open' ? status.kind : undefined}
            onOpenMonitors={() => setMonitorPanelOpen(true)}
            captureActive={captureMode}
            capturePaused={captureEngaged && !captureMode}
            onCaptureInput={enterCaptureMode}
            onResumeCapture={resumeCapture}
            onExitCapture={exitCaptureMode}
            restorable={sessionState?.restorable ?? 0}
            onRestore={() => controllerRef.current?.restoreMonitors()}
            onToggleHidpi={toggleHidpi}
            hidpiActive={hidpiOn}
          />
          {sessionState && controllerRef.current && (
            <MonitorManagerPanel
              open={monitorPanelOpen}
              onClose={() => setMonitorPanelOpen(false)}
              session={controllerRef.current}
              state={sessionState}
              topology={topology}
            />
          )}
          {/* Synthetic cursor for pointer-lock capture mode (the real cursor
              is hidden while locked). Positioned imperatively in the lock
              handler; hidden otherwise. */}
          <img
            ref={captureSpriteElRef}
            alt=""
            draggable={false}
            style={{
              position: 'fixed',
              left: 0,
              top: 0,
              pointerEvents: 'none',
              zIndex: 6,
              display: 'none',
              imageRendering: 'pixelated',
            }}
          />
          {captureMode && (
            <div style={captureHintStyle}>
              Input captured — press Esc to release
            </div>
          )}
        </>
      )}
      <StatusBadge status={status} mode={mode} monitor={monitorIndex} />
    </div>
  );
}

function canvasToVirtualCoords(
  clientX: number,
  clientY: number,
  canvas: HTMLCanvasElement,
  monitor: { x: number; y: number; width: number; height: number },
  scalePercent: number
) {
  const rect = canvas.getBoundingClientRect();
  if (rect.width === 0 || rect.height === 0) return { x: 0, y: 0 };
  // monitor.{x,y,width,height} are stored in CSS pixels (so server-side
  // `applyScale` derives physical pixels from `scale`). Mouse events
  // arrive in CSS pixels. The server expects mouse coords in **physical**
  // desktop pixels (same coord system as the framebuffer), so we multiply
  // by scale/100 before sending.
  //
  // This is linear, so it also extrapolates correctly when (clientX, clientY)
  // is *outside* the canvas — which is how a button-held drag tracks the
  // cursor across the seam into a neighboring monitor's window: the OS keeps
  // delivering mousemove to the window where the drag began, with coordinates
  // that run past the canvas bounds into the adjacent monitor's virtual space.
  const scaleRatio = Math.max(1, scalePercent) / 100;
  const localX = ((clientX - rect.left) * monitor.width) / rect.width;
  const localY = ((clientY - rect.top) * monitor.height) / rect.height;
  return {
    x: Math.round((localX + monitor.x) * scaleRatio),
    y: Math.round((localY + monitor.y) * scaleRatio),
  };
}

// Memo for the last cursor encode: servers often re-send the same cursor
// bitmap repeatedly, and `toDataURL` (PNG-encode) is the expensive step. We
// reuse the last data URL when the incoming pixels are byte-identical — a 4 KB
// compare is far cheaper than re-encoding.
let cursorMemo: { data: Uint8ClampedArray; url: string; scale: number } | null =
  null;

function samePixels(a: Uint8ClampedArray, b: Uint8ClampedArray): boolean {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i++) {
    if (a[i] !== b[i]) return false;
  }
  return true;
}

function applyCursorBitmap(
  canvas: HTMLCanvasElement,
  msg: {
    imageData: ImageData;
    hotspotX: number;
    hotspotY: number;
  }
) {
  const { imageData, hotspotX, hotspotY } = msg;
  let url: string;
  let scale: number;
  if (cursorMemo && samePixels(cursorMemo.data, imageData.data)) {
    ({ url, scale } = cursorMemo);
  } else {
    let buffer = document.createElement('canvas');
    buffer.width = imageData.width;
    buffer.height = imageData.height;
    buffer
      .getContext('2d', { colorSpace: imageData.colorSpace })!
      .putImageData(imageData, 0, 0);

    scale = 1;
    if (buffer.width > 32 || buffer.height > 32) {
      scale = Math.min(32 / buffer.width, 32 / buffer.height);
      const resized = document.createElement('canvas');
      resized.width = Math.max(1, Math.round(buffer.width * scale));
      resized.height = Math.max(1, Math.round(buffer.height * scale));
      const ctx = resized.getContext('2d', {
        colorSpace: imageData.colorSpace,
      })!;
      ctx.scale(scale, scale);
      ctx.drawImage(buffer, 0, 0);
      buffer = resized;
    }
    url = buffer.toDataURL();
    cursorMemo = { data: imageData.data, url, scale };
  }

  canvas.style.cursor = `url(${url}) ${Math.round(hotspotX * scale)} ${Math.round(hotspotY * scale)}, auto`;
  return { url, hotspotX, hotspotY, scale };
}

function StatusBadge({
  status,
  mode,
  monitor,
}: {
  status: Status;
  mode: WindowMode;
  monitor: number;
}) {
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
      {`${status.kind} [${mode} #${monitor}]`}
      {status.kind === 'error' ? `: ${status.message}` : ''}
    </span>
  );
}

/** Modifier keys whose held-state the server is tracked for (reconciliation). */
const REMOTE_MOD_CODES = new Set([
  'ShiftLeft',
  'ShiftRight',
  'ControlLeft',
  'ControlRight',
  'AltLeft',
  'AltRight',
  'MetaLeft',
  'MetaRight',
]);
/** Keydowns withheld until a non-modifier key proves an in-session chord
 * (OS* = legacy Firefox names for Meta). */
const WITHHELD_MOD_CODES = new Set([
  'MetaLeft',
  'MetaRight',
  'OSLeft',
  'OSRight',
  'AltLeft',
  'AltRight',
]);
/** Modifiers forwarded immediately but NOT flush triggers: Shift/Ctrl going
 * down must not flush a withheld Meta — Cmd+Shift+5 would leak the Meta. */
const PLAIN_MOD_CODES = new Set([
  'ShiftLeft',
  'ShiftRight',
  'ControlLeft',
  'ControlRight',
]);

const captureHintStyle: React.CSSProperties = {
  position: 'fixed',
  bottom: 12,
  left: '50%',
  transform: 'translateX(-50%)',
  padding: '6px 14px',
  borderRadius: 999,
  background: 'rgba(0, 0, 0, 0.7)',
  color: '#fff',
  fontFamily: 'system-ui, sans-serif',
  fontSize: 13,
  pointerEvents: 'none',
  zIndex: 7,
};

const outerWrapperStyle: React.CSSProperties = {
  position: 'fixed',
  inset: 0,
  margin: 0,
  padding: 0,
  background: '#000',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
};
