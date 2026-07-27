/**
 * Console-line formatter for the periodic perf snapshot the worker emits.
 * Same field shape is mirrored in the old client (see
 * `shared/libs/tdp/perfLogger.ts`) so a side-by-side terminal grep on
 * `[perf-` reads off matching columns.
 *
 * Post-A.2 split:
 *   - Worker emits decode-side stats (PerfPayload).
 *   - Main tracks input-side counters here (DOM mouse, outbound encode+send,
 *     RAF interval, postMessage-in by type) and merges them into the line.
 */

import type { PerfPayload } from './types';

/**
 * Mutable accumulator the host (`DesktopSessionTest.tsx`) updates as DOM
 * events fire and outbound bytes are sent. Reset to zero each time we
 * emit a perf line.
 */
export interface MainPerfState {
  /** DOM `mousemove` events on the canvas since the last flush. */
  domMouseMoveCount: number;
  /** Outbound messages sent over the WS (mouse + key + button) since flush. */
  outboundCount: number;
  /** Inbound worker→main `postMessage` count, bucketed by type. */
  postMessageCount: Record<string, number>;
  /** Captured RAF intervals (ms) since the last flush. */
  rafSamples: number[];
  /** Last RAF timestamp; used to compute the next interval. */
  lastRafMs: number;
  /** RAF id, so the host can cancel on unmount. */
  rafId: number | null;
}

export function makeMainPerf(): MainPerfState {
  return {
    domMouseMoveCount: 0,
    outboundCount: 0,
    postMessageCount: {},
    rafSamples: [],
    lastRafMs: 0,
    rafId: null,
  };
}

export function startRafSampler(state: MainPerfState): void {
  const tick = (now: number) => {
    if (state.lastRafMs !== 0) {
      state.rafSamples.push(now - state.lastRafMs);
    }
    state.lastRafMs = now;
    state.rafId = requestAnimationFrame(tick);
  };
  state.rafId = requestAnimationFrame(tick);
}

export function stopRafSampler(state: MainPerfState): void {
  if (state.rafId !== null) {
    cancelAnimationFrame(state.rafId);
    state.rafId = null;
  }
}

export function recordPostMessage(state: MainPerfState, type: string): void {
  state.postMessageCount[type] = (state.postMessageCount[type] ?? 0) + 1;
}

function fmt(ms: number): string {
  return ms.toFixed(2);
}

function maxOrZero(xs: number[]): number {
  let m = 0;
  for (const x of xs) {
    if (x > m) m = x;
  }
  return m;
}

function meanOrZero(xs: number[]): number {
  if (xs.length === 0) return 0;
  let s = 0;
  for (const x of xs) s += x;
  return s / xs.length;
}

/**
 * Format a perf line for `console.log`. `tag` is `new` or `old` so a grep
 * picks one client at a time. Worker stats come from `p`; main-side stats
 * come from `main` if provided.
 */
export function formatPerfLine(
  tag: string,
  p: PerfPayload,
  main: MainPerfState | null
): string {
  const sec = p.elapsed_ms / 1000;
  const pdusPerSec = sec > 0 ? p.pdus / sec : 0;
  const paintsPerSec = sec > 0 ? p.paints / sec : 0;
  const avgDirty = p.paints > 0 ? Math.round(p.dirty_pixels / p.paints) : 0;
  const mousePerSec = sec > 0 && main ? main.outboundCount / sec : 0;

  let line =
    `[perf-${tag}] ${sec.toFixed(2)}s ` +
    `pdus/s=${pdusPerSec.toFixed(1)} paints/s=${paintsPerSec.toFixed(1)} ` +
    `mouse_out/s=${mousePerSec.toFixed(1)} ` +
    `avg_dirty_px=${avgDirty} resp_bytes=${p.response_bytes} | ` +
    `decode=${fmt(p.codec_decode_mean_ms)}/${fmt(p.codec_decode_max_ms)}ms ` +
    `process=${fmt(p.ironrdp_process_mean_ms)}/${fmt(p.ironrdp_process_max_ms)}ms ` +
    `[surf=${p.process_surface_n}:${fmt(p.process_surface_mean_ms)}/${fmt(p.process_surface_max_ms)} ` +
    `bmp=${p.process_bitmap_n}:${fmt(p.process_bitmap_mean_ms)}/${fmt(p.process_bitmap_max_ms)} ` +
    `ptr=${p.process_pointer_n}:${fmt(p.process_pointer_mean_ms)}/${fmt(p.process_pointer_max_ms)} ` +
    `oth=${p.process_other_n}:${fmt(p.process_other_mean_ms)}/${fmt(p.process_other_max_ms)}] ` +
    `pix_copy=${fmt(p.pixel_copy_mean_ms)}/${fmt(p.pixel_copy_max_ms)}ms ` +
    `put_img=${fmt(p.put_image_mean_ms)}/${fmt(p.put_image_max_ms)}ms ` +
    `resp_send=${fmt(p.response_send_mean_ms)}/${fmt(p.response_send_max_ms)}ms ` +
    `cursor=${fmt(p.cursor_post_mean_ms)}/${fmt(p.cursor_post_max_ms)}ms`;

  if (main) {
    const domPerSec = sec > 0 ? main.domMouseMoveCount / sec : 0;
    const rafMean = meanOrZero(main.rafSamples);
    const rafMax = maxOrZero(main.rafSamples);
    const msgParts: string[] = [];
    for (const [t, n] of Object.entries(main.postMessageCount)) {
      msgParts.push(`${t}=${n}`);
    }
    line +=
      ` | dom_mouse/s=${domPerSec.toFixed(1)} ` +
      `raf=${fmt(rafMean)}/${fmt(rafMax)}ms ` +
      `msg_in={${msgParts.join(',')}}`;
  }

  return line;
}

export function resetMainPerf(state: MainPerfState): void {
  state.domMouseMoveCount = 0;
  state.outboundCount = 0;
  state.postMessageCount = {};
  state.rafSamples = [];
}
