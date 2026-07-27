/**
 * Passive display for a popup monitor in the codec-test multi-window flow.
 *
 * The popup has no WebSocket, no wasm decoder, and no `MainCodec`. It owns a
 * canvas whose control it hands to the main window's shared decode worker via
 * `transferControlToOffscreen` (sent up with the `ready` message). The single
 * wasm decoder then paints THIS popup's viewport slice straight into the canvas
 * over GL — exactly as it paints main's own canvas — with no per-frame pixel
 * copy and no `paintFrame` round-trip. The popup still listens for cursor
 * messages and forwards mouse/keyboard events back via `window.opener.postMessage`.
 *
 * This works because a same-origin `window.open` popup whose opener is intact
 * shares a renderer process with main; the worker is main's DedicatedWorker
 * (same process), so it can drive the popup's placeholder canvas. (Cross-origin
 * / OOP-iframe placeholders are a different process and would not render.)
 *
 * Protocol with main (lockstep with main's `popupMessageHandler`):
 *
 *   popup → main:
 *     { type: 'ready', monitorIndex, innerWidth, innerHeight, canvas } // canvas transferred
 *     { type: 'input', subtype: 'mousemove', virtualX, virtualY }
 *     { type: 'input', subtype: 'mousebutton', button, pressed, virtualX, virtualY }
 *     { type: 'input', subtype: 'mouseleave', virtualX, virtualY }
 *     { type: 'input', subtype: 'key', code, pressed, scrollLock, numLock, capsLock }
 *     { type: 'input', subtype: 'armSync' } // popup blurred: resync keys on next key
 *     { type: 'input', subtype: 'paste', text }
 *
 *   main → popup:
 *     { type: 'init', layoutEntry: {x,y,width,height}, bbox: {width,height} }
 *     { type: 'cursorBitmap', imageData, hotspotX, hotspotY }
 *     { type: 'cursorHidden' }
 *     { type: 'cursorDefault' }
 *
 * The popup's `OffscreenCanvas` is transferred (popup → main → worker) with the
 * `ready` message, so after handing it off the popup never touches pixels again.
 * When the main window refreshes / closes / navigates it actively closes its
 * popups (a `pagehide` listener); as a safety net the popup also self-closes if
 * it ever sees `window.opener.closed` (e.g. a main-window crash). `window.open`
 * children are NOT closed automatically by the browser when the opener goes.
 */

import { useEffect, useRef, useState } from 'react';

import { KEY_SCANCODES } from 'shared/libs/tdp/codec';

import type { MonitorSpec } from './DesktopSessionTest';

export interface DesktopSessionPopupDisplayProps {
  monitors: MonitorSpec[];
  monitorIndex: number;
}

type LayoutEntry = { x: number; y: number; width: number; height: number };

export type MainToPopup =
  | {
      type: 'init';
      layoutEntry: LayoutEntry;
      bbox: { width: number; height: number };
      /** RDP scale percent (e.g. 200 on a DPR-2 display). The popup multiplies
       * its CSS-pixel coords by scale/100 to reach the server's physical-pixel
       * desktop space — same as main's canvasToVirtualCoords. */
      scale: number;
    }
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
  // Pointer-lock capture mode (driven from main). While active the popup is
  // display-only (it sends no input) and renders a synthetic cursor sprite at
  // the broadcast virtual-desktop position.
  | { type: 'capture'; active: boolean }
  | { type: 'captureCursor'; vx: number; vy: number };

export type PopupToMain =
  | {
      type: 'ready';
      monitorIndex: number;
      innerWidth: number;
      innerHeight: number;
      /** Control of the popup's visible canvas, transferred so the shared
       * worker can paint this monitor's viewport straight into it over GL. */
      canvas: OffscreenCanvas;
    }
  | {
      type: 'input';
      subtype: 'mousemove' | 'mouseleave';
      virtualX: number;
      virtualY: number;
    }
  | {
      type: 'input';
      subtype: 'mousebutton';
      button: 0 | 1 | 2;
      pressed: boolean;
      virtualX: number;
      virtualY: number;
    }
  | {
      type: 'input';
      subtype: 'key';
      code: string;
      pressed: boolean;
      /** Lock-key state from the popup's KeyboardEvent (getModifierState),
       * carried along so main can prefix the relayed key with a SyncKeys
       * when one is armed — main has no event of its own to read them from. */
      scrollLock: boolean;
      numLock: boolean;
      capsLock: boolean;
    }
  | {
      type: 'input';
      /** Popup lost focus: a key held there may never deliver its keyup, so
       * main should arm sync-before-next-key (the next key event anywhere
       * resets the server key state — see main's maybeSyncKeys). */
      subtype: 'armSync';
    }
  | {
      type: 'input';
      subtype: 'wheel';
      // Raw browser WheelEvent pixel deltas (DOM_DELTA_PIXEL). Main negates,
      // accumulates the sub-pixel remainder, and encodes — kept there so all
      // wheel tuning lives in one place.
      deltaX: number;
      deltaY: number;
    }
  | {
      type: 'input';
      subtype: 'paste';
      text: string;
    };

type Status =
  | { kind: 'idle' }
  | { kind: 'waiting-for-main' }
  | { kind: 'open' }
  | { kind: 'error'; message: string };

export function DesktopSessionPopupDisplay({
  monitors: _monitors,
  monitorIndex,
}: DesktopSessionPopupDisplayProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const layoutEntryRef = useRef<LayoutEntry | null>(null);
  // RDP scale percent from main's `init`. Mirrors main's `scale` prop; used to
  // convert popup CSS px → physical desktop px. Default 100 until `init` lands.
  const scaleRef = useRef(100);
  const suppressedKeysRef = useRef(new Set<string>());
  // Cross-window input state (mirrors main): a button-held drag tracked at the
  // window level so it follows across the seam, and pointer-lock capture mode
  // where the popup is display-only and renders a synthetic cursor sprite.
  const dragActiveRef = useRef(false);
  // See main: prevents the bubble-phase onMouseUp double-firing after a drag.
  const suppressMouseUpRef = useRef(false);
  const captureActiveRef = useRef(false);
  const spriteRef = useRef<{
    url: string;
    hotspotX: number;
    hotspotY: number;
    scale: number;
  } | null>(null);
  const spriteElRef = useRef<HTMLImageElement | null>(null);
  const [status, setStatus] = useState<Status>({ kind: 'idle' });

  // Position the synthetic cursor sprite for the broadcast virtual position
  // (CSS px). Shown only while the virtual cursor is within this popup's region.
  function positionSprite(vx: number, vy: number) {
    const entry = layoutEntryRef.current;
    const sprite = spriteRef.current;
    const el = spriteElRef.current;
    const canvas = canvasRef.current;
    if (!entry || !el || !canvas) return;
    const rect = canvas.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) return;
    const inside =
      vx >= entry.x &&
      vx <= entry.x + entry.width &&
      vy >= entry.y &&
      vy <= entry.y + entry.height;
    if (sprite && inside) {
      const lx = ((vx - entry.x) * rect.width) / entry.width;
      const ly = ((vy - entry.y) * rect.height) / entry.height;
      if (el.getAttribute('src') !== sprite.url) el.src = sprite.url;
      el.style.display = 'block';
      el.style.transform = `translate(${lx - sprite.hotspotX * sprite.scale}px, ${ly - sprite.hotspotY * sprite.scale}px)`;
    } else {
      el.style.display = 'none';
    }
  }

  // Track a drag started in this popup at the window level so it keeps
  // forwarding virtual coords once the cursor leaves the popup (across the seam).
  function beginDrag() {
    if (dragActiveRef.current) return;
    dragActiveRef.current = true;
    const onMove = (ev: MouseEvent) => {
      const c = virtualCoords(ev.clientX, ev.clientY);
      if (c) send({ type: 'input', subtype: 'mousemove', virtualX: c.x, virtualY: c.y });
    };
    const onUp = (ev: MouseEvent) => {
      const c = virtualCoords(ev.clientX, ev.clientY);
      if (c) {
        const button = (ev.button === 1 ? 1 : ev.button === 2 ? 2 : 0) as 0 | 1 | 2;
        send({
          type: 'input',
          subtype: 'mousebutton',
          button,
          pressed: false,
          virtualX: c.x,
          virtualY: c.y,
        });
      }
      window.removeEventListener('mousemove', onMove, true);
      window.removeEventListener('mouseup', onUp, true);
      dragActiveRef.current = false;
      suppressMouseUpRef.current = true;
    };
    window.addEventListener('mousemove', onMove, true);
    window.addEventListener('mouseup', onUp, true);
  }

  useEffect(() => {
    const opener = window.opener as Window | null;
    if (!opener) {
      setStatus({
        kind: 'error',
        message:
          'window.opener is null — this page must be opened by the codec-test main window',
      });
      return;
    }

    const canvas = canvasRef.current;
    if (!canvas) return;
    const myWidth = Math.max(2, window.innerWidth & ~1);
    const myHeight = Math.max(2, window.innerHeight & ~1);
    canvas.width = myWidth;
    canvas.height = myHeight;

    // Hand control of this canvas to main's shared decode worker (popup → main
    // → worker). The single wasm decoder then paints this popup's viewport slice
    // straight into the canvas over GL — same path as main's own canvas, no
    // per-frame pixel copy. transferControlToOffscreen can only run once per
    // element; React re-creates the element each mount via the ref, so effect
    // cleanup + remount is safe.
    let offscreen: OffscreenCanvas;
    try {
      offscreen = canvas.transferControlToOffscreen();
    } catch (e) {
      setStatus({
        kind: 'error',
        message: `transferControlToOffscreen failed: ${e instanceof Error ? e.message : String(e)}`,
      });
      return;
    }

    setStatus({ kind: 'waiting-for-main' });

    function handleMessage(ev: MessageEvent<MainToPopup>) {
      if (ev.source !== opener) return;
      const m = ev.data;
      if (!m || typeof m !== 'object' || typeof m.type !== 'string') return;
      switch (m.type) {
        case 'init':
          layoutEntryRef.current = m.layoutEntry;
          scaleRef.current = m.scale;
          setStatus({ kind: 'open' });
          break;
        case 'cursorBitmap':
          spriteRef.current = applyCursorBitmap(canvas, m);
          // In capture mode the real cursor stays hidden; the sprite shows it.
          if (captureActiveRef.current) canvas.style.cursor = 'none';
          break;
        case 'cursorHidden':
          if (!captureActiveRef.current) canvas.style.cursor = 'none';
          break;
        case 'cursorDefault':
          if (!captureActiveRef.current) canvas.style.cursor = 'default';
          break;
        case 'capture':
          captureActiveRef.current = m.active;
          canvas.style.cursor = m.active ? 'none' : 'default';
          if (!m.active && spriteElRef.current) {
            spriteElRef.current.style.display = 'none';
          }
          break;
        case 'captureCursor':
          positionSprite(m.vx, m.vy);
          break;
      }
    }

    window.addEventListener('message', handleMessage);

    // Send 'ready' WITH the transferred canvas so main can forward it to the
    // worker. Transferables require the 3-arg postMessage form, so this can't
    // go through `sendToMain` (which is for plain, untransferred messages).
    try {
      opener.postMessage(
        {
          type: 'ready',
          monitorIndex,
          innerWidth: myWidth,
          innerHeight: myHeight,
          canvas: offscreen,
        } satisfies PopupToMain,
        '*',
        [offscreen]
      );
    } catch (e) {
      // eslint-disable-next-line no-console
      console.warn('[popup-display] postMessage(ready) failed', e);
    }

    // Self-close if the main window goes away without closing us — e.g. a
    // crash where its `pagehide` never fired. On a normal refresh / close /
    // navigate the main actively closes us; this is the safety net for the rest.
    const openerWatch = setInterval(() => {
      if (opener.closed) {
        clearInterval(openerWatch);
        try {
          window.close();
        } catch {
          /* ignore */
        }
      }
    }, 1000);

    return () => {
      window.removeEventListener('message', handleMessage);
      clearInterval(openerWatch);
    };
  }, [monitorIndex]);

  // Native non-passive wheel listener: React's `onWheel` is passive, so
  // `preventDefault()` (needed to stop the browser scrolling/zooming the popup
  // page) would be ignored. We forward raw deltas to main, which owns the
  // negate/accumulate/encode. Rebinds on status change so the gate stays fresh.
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const onWheel = (e: WheelEvent) => {
      e.preventDefault();
      if (status.kind !== 'open') return;
      if (captureActiveRef.current) return; // display-only in capture mode
      if (e.deltaMode !== 0) return; // pixel-precision deltas only
      send({
        type: 'input',
        subtype: 'wheel',
        deltaX: e.deltaX,
        deltaY: e.deltaY,
      });
    };
    canvas.addEventListener('wheel', onWheel, { capture: true, passive: false });
    return () => canvas.removeEventListener('wheel', onWheel, true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [status.kind]);

  function virtualCoords(clientX: number, clientY: number) {
    const canvas = canvasRef.current;
    const entry = layoutEntryRef.current;
    if (!canvas || !entry) return null;
    const rect = canvas.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) return null;
    // CSS-pixel local + viewport offset, then scale to physical desktop px —
    // identical to main's canvasToVirtualCoords. Linear, so it extrapolates
    // past the canvas bounds for cross-seam drag tracking.
    const scaleRatio = Math.max(1, scaleRef.current) / 100;
    const localX = ((clientX - rect.left) * entry.width) / rect.width;
    const localY = ((clientY - rect.top) * entry.height) / rect.height;
    return {
      x: Math.round((localX + entry.x) * scaleRatio),
      y: Math.round((localY + entry.y) * scaleRatio),
    };
  }

  function send(msg: PopupToMain) {
    const opener = window.opener as Window | null;
    if (!opener) return;
    sendToMain(opener, msg);
  }

  function handleMouseMove(e: React.MouseEvent<HTMLCanvasElement>) {
    if (status.kind !== 'open') return;
    // Capture mode = display-only; a drag is tracked at the window level.
    if (captureActiveRef.current || dragActiveRef.current) return;
    const c = virtualCoords(e.clientX, e.clientY);
    if (!c) return;
    send({
      type: 'input',
      subtype: 'mousemove',
      virtualX: c.x,
      virtualY: c.y,
    });
  }

  function handleMouseButton(pressed: boolean) {
    return (e: React.MouseEvent<HTMLCanvasElement>) => {
      if (status.kind !== 'open') return;
      e.preventDefault();
      if (captureActiveRef.current) return; // display-only in capture mode
      const c = virtualCoords(e.clientX, e.clientY);
      if (!c) return;
      const button = (e.button === 1 ? 1 : e.button === 2 ? 2 : 0) as 0 | 1 | 2;
      if (pressed) {
        if (dragActiveRef.current) return;
        // Chord intent (Cmd+click): deliver withheld modifiers first.
        flushWithheldMods(e);
        suppressMouseUpRef.current = false;
        send({
          type: 'input',
          subtype: 'mousebutton',
          button,
          pressed: true,
          virtualX: c.x,
          virtualY: c.y,
        });
        beginDrag();
      } else {
        if (dragActiveRef.current) return;
        if (suppressMouseUpRef.current) {
          suppressMouseUpRef.current = false;
          return;
        }
        send({
          type: 'input',
          subtype: 'mousebutton',
          button,
          pressed: false,
          virtualX: c.x,
          virtualY: c.y,
        });
      }
    };
  }

  function handleMouseLeave(e: React.MouseEvent<HTMLCanvasElement>) {
    if (status.kind !== 'open') return;
    if (captureActiveRef.current || dragActiveRef.current) return;
    const canvas = canvasRef.current;
    const entry = layoutEntryRef.current;
    if (!canvas || !entry) return;
    const rect = canvas.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) return;
    const NUDGE_PX = 8;
    let outX = entry.x;
    let outY = entry.y;
    if (e.clientX <= rect.left) outX = entry.x - NUDGE_PX;
    else if (e.clientX >= rect.right - 1)
      outX = entry.x + entry.width + NUDGE_PX - 1;
    else
      outX =
        entry.x +
        Math.round(((e.clientX - rect.left) * entry.width) / rect.width);
    if (e.clientY <= rect.top) outY = entry.y - NUDGE_PX;
    else if (e.clientY >= rect.bottom - 1)
      outY = entry.y + entry.height + NUDGE_PX - 1;
    else
      outY =
        entry.y +
        Math.round(((e.clientY - rect.top) * entry.height) / rect.height);
    outX = Math.max(0, outX);
    outY = Math.max(0, outY);
    // CSS px → physical desktop px, same as virtualCoords / main's handleMouseLeave.
    const scaleRatio = Math.max(1, scaleRef.current) / 100;
    send({
      type: 'input',
      subtype: 'mouseleave',
      virtualX: Math.round(outX * scaleRatio),
      virtualY: Math.round(outY * scaleRatio),
    });
  }

  async function performPasteFromHost() {
    if (status.kind !== 'open') return;
    let text = '';
    try {
      text = await navigator.clipboard.readText();
    } catch (e) {
      // eslint-disable-next-line no-console
      console.warn('clipboard.readText failed (browser denied?):', e);
      return;
    }
    if (!text) return;
    send({ type: 'input', subtype: 'paste', text });
  }

  // Withhold Meta/Alt downs until a non-modifier key proves an in-session
  // chord — mirrors main's stuck-modifier defense (an OS chord like
  // Cmd+Shift+5 swallows its keyups, and a forwarded Meta down would latch
  // LWIN on the server). Modifier-set mirrors of main's WITHHELD/PLAIN sets.
  const withheldModsRef = useRef<string[]>([]);
  const WITHHELD = ['MetaLeft', 'MetaRight', 'OSLeft', 'OSRight', 'AltLeft', 'AltRight'];
  const PLAIN = ['ShiftLeft', 'ShiftRight', 'ControlLeft', 'ControlRight'];

  // Keyboard AND mouse events can prove chord intent (Cmd+click); both
  // expose getModifierState for the lock-key states the relay carries.
  function flushWithheldMods(ev: { getModifierState(key: string): boolean }) {
    for (const code of withheldModsRef.current) {
      send({
        type: 'input',
        subtype: 'key',
        code,
        pressed: true,
        scrollLock: ev.getModifierState('ScrollLock'),
        numLock: ev.getModifierState('NumLock'),
        capsLock: ev.getModifierState('CapsLock'),
      });
    }
    withheldModsRef.current = [];
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLCanvasElement>) {
    if (status.kind !== 'open') return;
    if ((e.metaKey || e.ctrlKey) && e.code === 'KeyV') {
      e.preventDefault();
      suppressedKeysRef.current.add(e.code);
      // The held Cmd is consumed by the paste — drop its withheld down.
      withheldModsRef.current = withheldModsRef.current.filter(
        c => !WITHHELD.includes(c)
      );
      void performPasteFromHost();
      return;
    }
    if (!KEY_SCANCODES[e.code]) return;
    if (WITHHELD.includes(e.code)) {
      e.preventDefault();
      if (!withheldModsRef.current.includes(e.code)) {
        withheldModsRef.current.push(e.code);
      }
      return;
    }
    e.preventDefault();
    if (!PLAIN.includes(e.code)) flushWithheldMods(e);
    send({ type: 'input', subtype: 'key', ...keyFields(e, true) });
  }

  function handleKeyUp(e: React.KeyboardEvent<HTMLCanvasElement>) {
    if (status.kind !== 'open') return;
    if (withheldModsRef.current.includes(e.code)) {
      // The down was never sent — swallow the up.
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
    send({ type: 'input', subtype: 'key', ...keyFields(e, false) });
  }

  // Lock-key state travels with every relayed key so main can prefix it with
  // a SyncKeys when armed (main has no KeyboardEvent of its own to read).
  function keyFields(e: React.KeyboardEvent<HTMLCanvasElement>, pressed: boolean) {
    return {
      code: e.code,
      pressed,
      scrollLock: e.getModifierState('ScrollLock'),
      numLock: e.getModifierState('NumLock'),
      capsLock: e.getModifierState('CapsLock'),
    };
  }

  return (
    <div style={outerWrapperStyle}>
      <canvas
        ref={c => {
          canvasRef.current = c;
          if (c) c.focus();
        }}
        tabIndex={0}
        onMouseMove={handleMouseMove}
        onMouseDown={handleMouseButton(true)}
        onMouseUp={handleMouseButton(false)}
        onMouseLeave={handleMouseLeave}
        onKeyDown={handleKeyDown}
        onKeyUp={handleKeyUp}
        // A key held when the popup loses focus may never deliver its keyup
        // here — tell main to resync the server's key state on the next key.
        onBlur={() => {
          // Withheld modifiers must never deliver after focus left (the OS
          // chord that stole it owns those keys now).
          withheldModsRef.current = [];
          send({ type: 'input', subtype: 'armSync' });
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
      {/* Synthetic cursor for pointer-lock capture mode (positioned by main's
          captureCursor broadcasts; hidden otherwise). */}
      <img
        ref={spriteElRef}
        alt=""
        draggable={false}
        style={{
          position: 'fixed',
          left: 0,
          top: 0,
          pointerEvents: 'none',
          zIndex: 2,
          display: 'none',
          imageRendering: 'pixelated',
        }}
      />
      <StatusBadge status={status} monitorIndex={monitorIndex} />
    </div>
  );
}

function sendToMain(opener: Window, msg: PopupToMain) {
  try {
    opener.postMessage(msg, '*');
  } catch (e) {
    // eslint-disable-next-line no-console
    console.warn('[popup-display] postMessage to main failed', e);
  }
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
  monitorIndex,
}: {
  status: Status;
  monitorIndex: number;
}) {
  if (status.kind === 'open') return null;
  const styles: Record<Status['kind'], React.CSSProperties> = {
    idle: { background: '#e0e0e0', color: '#333' },
    'waiting-for-main': { background: '#fff7d6', color: '#7a5c00' },
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
      {`${status.kind} [popup #${monitorIndex}]`}
      {status.kind === 'error' ? `: ${status.message}` : ''}
    </span>
  );
}

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
