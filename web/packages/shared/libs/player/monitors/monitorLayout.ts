/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * Pure, DOM-free monitor-layout math for the multi-monitor RDP harness.
 *
 * The RDP virtual desktop spans the bounding box of all monitors. We work in a
 * single coordinate space — **bbox-relative, non-negative, CSS pixels** — which
 * is what both consumers need:
 *   - `WorkerDecoder.addCanvas` viewports take `u32` (no negatives), and
 *   - the server's `build_monitor_layout` re-bases the primary to (0,0) itself
 *     (`m.x - primary.x`), so it accepts any consistent origin.
 *
 * So we never emit negative coordinates: a monitor physically left of / above
 * the primary just shifts the whole box right / down after translation.
 */

/** A monitor window whose layout we want to compute. Inputs are CSS pixels. */
export interface MonitorPlacement {
  /** Stable id (0 = main window, popups use their monitorIndex). */
  id: number;
  /** Window inner width in CSS px (the RDP monitor's logical width). */
  cssWidth: number;
  /** Window inner height in CSS px. */
  cssHeight: number;
  isPrimary: boolean;
  /**
   * The window's CONTENT (viewport) top-left in the OS virtual-screen
   * coordinate space (CSS px), when known — see `contentScreenPosition`.
   * Drives the auto-from-physical arrangement.
   */
  physical?: { left: number; top: number };
  /**
   * Manual placement relative to the primary's top-left (CSS px). When set it
   * overrides `physical` for this monitor (the user dragged the tile).
   */
  manualOffset?: { x: number; y: number };
}

/** A monitor positioned in the bbox-relative virtual-desktop space (CSS px). */
export interface VirtualMonitor {
  id: number;
  x: number;
  y: number;
  width: number;
  height: number;
  isPrimary: boolean;
}

export interface LayoutResult {
  monitors: VirtualMonitor[];
  bboxWidth: number;
  bboxHeight: number;
}

/** A rectangle in physical framebuffer pixels (what `addCanvas` viewports use). */
export interface PhysRect {
  x: number;
  y: number;
  width: number;
  height: number;
}

/**
 * CSS px → physical framebuffer px, matching the server's `applyScale`
 * exactly: `floor(v * s / 100)` with `s = max(scale, 100)`
 * (see `client.go` buildMonitorCArray). Used for canvas viewports so each
 * monitor's slice aligns with the decoded desktop.
 */
export function toPhysical(value: number, scalePercent: number): number {
  const s = Math.max(100, scalePercent);
  return Math.floor((value * s) / 100);
}

/**
 * Screen position of a window's CONTENT area (viewport top-left), not its OS
 * frame. Browser chrome height differs per window kind — the main window's
 * tab strip + toolbar is ~30px taller than a popup's slim bar — so using
 * frame coordinates (`screenLeft/screenTop`) misaligns the remote desktop
 * across the seam even when the OS window frames are perfectly aligned.
 * Assumes all chrome sits above the viewport (true for Chrome/Firefox/Safari
 * windows; docked DevTools skew it, undocked don't).
 */
export function contentScreenPosition(win: Window): {
  left: number;
  top: number;
} {
  return {
    left: win.screenLeft,
    top: win.screenTop + Math.max(0, win.outerHeight - win.innerHeight),
  };
}

/** A rectangle in OS virtual-screen (world) coordinates, CSS px. */
export interface SnapRect {
  left: number;
  top: number;
  width: number;
  height: number;
}

/** Candidate snap positions for one axis: flush-adjacent on either side of
 * the anchor, and same-edge aligned. */
function axisSnapCandidates(aLo: number, aHi: number, size: number): number[] {
  return [aLo - size, aHi, aLo, aHi - size];
}

/**
 * Snap a dragged monitor rect against anchor rects (the other monitors and
 * the physical displays), Windows-Snap/Magnet style: each axis independently
 * snaps to the nearest candidate within `threshold` — flush adjacency
 * (left-of / right-of / above / below an anchor) or edge alignment (matching
 * lefts/rights/tops/bottoms). Corners fall out of both axes snapping at
 * once. Returns the (possibly unchanged) top-left.
 */
export function snapRect(
  dragged: SnapRect,
  anchors: SnapRect[],
  threshold: number
): { left: number; top: number } {
  let bestX: { pos: number; d: number } | null = null;
  let bestY: { pos: number; d: number } | null = null;
  for (const a of anchors) {
    for (const c of axisSnapCandidates(a.left, a.left + a.width, dragged.width)) {
      const d = Math.abs(c - dragged.left);
      if (d <= threshold && (!bestX || d < bestX.d)) bestX = { pos: c, d };
    }
    for (const c of axisSnapCandidates(a.top, a.top + a.height, dragged.height)) {
      const d = Math.abs(c - dragged.top);
      if (d <= threshold && (!bestY || d < bestY.d)) bestY = { pos: c, d };
    }
  }
  return {
    left: bestX ? bestX.pos : dragged.left,
    top: bestY ? bestY.pos : dragged.top,
  };
}

/**
 * OS-style snap zone for a pointer over a display, like Windows Snap /
 * Magnet: in the edge band on one side → that half (left/right/top/bottom);
 * in two bands at once (a corner region) → that quarter. `bandFrac` is the
 * band depth as a FRACTION of the display dimension (proportional, so the
 * trigger regions feel the same on the small layout map at any zoom —
 * pixel-based bands were fiddly to hit). Returns the zone as FRAME bounds in
 * world CSS px, or null when the pointer is in the center region (or off
 * this display entirely).
 */
export function snapZoneForPointer(
  px: number,
  py: number,
  display: SnapRect,
  bandFrac: number
): SnapRect | null {
  const { left, top, width, height } = display;
  const right = left + width;
  const bottom = top + height;
  if (px < left || px > right || py < top || py > bottom) return null;
  const nearL = px - left <= width * bandFrac;
  const nearR = right - px <= width * bandFrac;
  const nearT = py - top <= height * bandFrac;
  const nearB = bottom - py <= height * bandFrac;
  const halfW = Math.round(width / 2);
  const halfH = Math.round(height / 2);
  if (nearL && nearT) return { left, top, width: halfW, height: halfH };
  if (nearR && nearT)
    return { left: left + halfW, top, width: width - halfW, height: halfH };
  if (nearL && nearB)
    return { left, top: top + halfH, width: halfW, height: height - halfH };
  if (nearR && nearB)
    return {
      left: left + halfW,
      top: top + halfH,
      width: width - halfW,
      height: height - halfH,
    };
  if (nearL) return { left, top, width: halfW, height };
  if (nearR) return { left: left + halfW, top, width: width - halfW, height };
  if (nearT) return { left, top, width, height: halfH };
  if (nearB) return { left, top: top + halfH, width, height: height - halfH };
  return null;
}

/**
 * Clamp a dragged window rect so it stays on a real display: pick the display
 * with the largest overlap (falling back to the nearest by center when the
 * rect is fully in the void) and clamp the position into its bounds. Dragging
 * across displays still works — as the rect crosses a boundary, the dominant
 * overlap switches and the clamp target follows.
 */
export function clampRectToDisplays(
  rect: SnapRect,
  displays: SnapRect[]
): { left: number; top: number } {
  if (displays.length === 0) return { left: rect.left, top: rect.top };
  const overlapArea = (d: SnapRect) =>
    Math.max(
      0,
      Math.min(rect.left + rect.width, d.left + d.width) -
        Math.max(rect.left, d.left)
    ) *
    Math.max(
      0,
      Math.min(rect.top + rect.height, d.top + d.height) -
        Math.max(rect.top, d.top)
    );
  const centerDist = (d: SnapRect) => {
    const cx = rect.left + rect.width / 2 - (d.left + d.width / 2);
    const cy = rect.top + rect.height / 2 - (d.top + d.height / 2);
    return cx * cx + cy * cy;
  };
  let best = displays[0];
  let bestArea = overlapArea(best);
  for (const d of displays.slice(1)) {
    const a = overlapArea(d);
    if (a > bestArea || (a === bestArea && centerDist(d) < centerDist(best))) {
      best = d;
      bestArea = a;
    }
  }
  return {
    left: Math.min(
      Math.max(rect.left, best.left),
      Math.max(best.left, best.left + best.width - rect.width)
    ),
    top: Math.min(
      Math.max(rect.top, best.top),
      Math.max(best.top, best.top + best.height - rect.height)
    ),
  };
}

/**
 * Where (and how large) a new popup monitor should open. Prefers the largest
 * free strip of the display beside `main` — shrinking the desired size to
 * fit — so a new monitor never covers the running session when any usable
 * strip exists. Falls back to a clamped cascade (overlap unavoidable, e.g.
 * the main window fills the display).
 */
export function pickSpawnRect(opts: {
  /** Main window content rect (world CSS px), if known. */
  main: SnapRect | null;
  /** Target display's available rect, if a display was chosen. */
  bounds: SnapRect | null;
  desired: { width: number; height: number };
  gap?: number;
}): SnapRect {
  const { main, bounds, desired } = opts;
  const gap = opts.gap ?? 90;
  const MIN_W = 480;
  const MIN_H = 320;
  if (!main) {
    return {
      left: bounds ? bounds.left : 48,
      top: bounds ? bounds.top : 48,
      width: desired.width,
      height: desired.height,
    };
  }
  if (!bounds) {
    // No display info: open beside the main window at the desired size.
    return {
      left: main.left + main.width + gap,
      top: main.top,
      width: desired.width,
      height: desired.height,
    };
  }
  const bRight = bounds.left + bounds.width;
  const bBottom = bounds.top + bounds.height;
  const mRight = main.left + main.width;
  const mBottom = main.top + main.height;
  // Main window not on this display → the whole display is free.
  const mainOnDisplay =
    main.left < bRight &&
    mRight > bounds.left &&
    main.top < bBottom &&
    mBottom > bounds.top;
  if (!mainOnDisplay) {
    return {
      left: bounds.left,
      top: bounds.top,
      width: Math.min(desired.width, bounds.width),
      height: Math.min(desired.height, bounds.height),
    };
  }
  // Free strips around the main window, largest first.
  const strips: SnapRect[] = [
    // right of main
    {
      left: mRight + gap,
      top: bounds.top,
      width: bRight - (mRight + gap),
      height: bounds.height,
    },
    // left of main
    {
      left: bounds.left,
      top: bounds.top,
      width: main.left - gap - bounds.left,
      height: bounds.height,
    },
    // below main
    {
      left: bounds.left,
      top: mBottom + gap,
      width: bounds.width,
      height: bBottom - (mBottom + gap),
    },
    // above main
    {
      left: bounds.left,
      top: bounds.top,
      width: bounds.width,
      height: main.top - gap - bounds.top,
    },
  ].filter(s => s.width >= MIN_W && s.height >= MIN_H);
  strips.sort((a, b) => b.width * b.height - a.width * a.height);
  const strip = strips[0];
  if (strip) {
    return {
      left: strip.left,
      top: strip.top,
      width: Math.min(desired.width, strip.width),
      height: Math.min(desired.height, strip.height),
    };
  }
  // Cascade off the main window, clamped into the display.
  const width = Math.min(desired.width, bounds.width);
  const height = Math.min(desired.height, bounds.height);
  return {
    left: Math.min(Math.max(bounds.left, main.left + 48), bRight - width),
    top: Math.min(Math.max(bounds.top, main.top + 48), bBottom - height),
    width,
    height,
  };
}

/** Physical-pixel viewport for a monitor, for `register`/`reposition-canvas`. */
export function physViewport(m: VirtualMonitor, scalePercent: number): PhysRect {
  return {
    x: toPhysical(m.x, scalePercent),
    y: toPhysical(m.y, scalePercent),
    width: toPhysical(m.width, scalePercent),
    height: toPhysical(m.height, scalePercent),
  };
}

/** Round a dimension down to an even value (MS-RDPEDISP / driver constraint). */
function evenDim(n: number): number {
  return Math.min(8192, Math.max(2, Math.floor(n) & ~1));
}

function rectsOverlap(a: VirtualMonitor, b: VirtualMonitor): boolean {
  return (
    a.x < b.x + b.width &&
    b.x < a.x + a.width &&
    a.y < b.y + b.height &&
    b.y < a.y + a.height
  );
}

/**
 * Returns every overlapping pair `[idA, idB]` (idA < idB), in id order. Used to
 * warn the user before a layout is normalized.
 */
export function detectOverlaps(monitors: VirtualMonitor[]): Array<[number, number]> {
  const sorted = [...monitors].sort((a, b) => a.id - b.id);
  const pairs: Array<[number, number]> = [];
  for (let i = 0; i < sorted.length; i++) {
    for (let j = i + 1; j < sorted.length; j++) {
      if (rectsOverlap(sorted[i], sorted[j])) {
        pairs.push([sorted[i].id, sorted[j].id]);
      }
    }
  }
  return pairs;
}

/**
 * Closes dead gaps in a de-overlapped layout by pulling each monitor flush
 * against its neighbors. RDP virtual desktops must be contiguous — Windows
 * rejects a monitor layout with a hole between displays (it never applies the
 * resize, so the far monitor's framebuffer region is never grown/painted and
 * renders black). We position monitors from physical screen coordinates but
 * size them by window-inner dimensions, so a window that doesn't fill its
 * display leaves a gap before the next monitor; this removes it while
 * preserving left/right/above/below order. The popup windows stay at their
 * physical screen locations (separate OS windows), so packing the framebuffer
 * is invisible to the user.
 *
 * Two independent passes (X then Y): each monitor is pulled to sit flush after
 * the furthest-right (resp. furthest-down) already-placed monitor that shares
 * its row (resp. column); a monitor with no such neighbor anchors at the
 * minimum edge. Idempotent on already-contiguous layouts.
 */
function closeGaps(monitors: VirtualMonitor[]): VirtualMonitor[] {
  const yOverlap = (a: VirtualMonitor, b: VirtualMonitor) =>
    a.y < b.y + b.height && b.y < a.y + a.height;
  const xOverlap = (a: VirtualMonitor, b: VirtualMonitor) =>
    a.x < b.x + b.width && b.x < a.x + a.width;

  const out = monitors.map(m => ({ ...m }));

  // Horizontal pass: pull each monitor left to touch its nearest row-neighbor.
  const minX = Math.min(...out.map(m => m.x));
  const placedX: VirtualMonitor[] = [];
  for (const m of [...out].sort((a, b) => a.x - b.x || a.id - b.id)) {
    const rowNeighbors = placedX.filter(p => yOverlap(p, m));
    m.x = rowNeighbors.length
      ? Math.max(...rowNeighbors.map(p => p.x + p.width))
      : minX;
    placedX.push(m);
  }

  // Vertical pass: a monitor that already y-overlaps something placed is
  // SIDE-CONNECTED (the horizontal pass pulled it flush against that row
  // neighbor) and KEEPS its y — vertical offsets between side-by-side
  // monitors are real (differing window chrome heights), and flattening
  // them was the cross-seam misalignment bug; that includes a stacked
  // monitor that also touches a tall side neighbor (L-shaped layouts: the
  // lower-right popup must not be pulled up under the upper-right one, or
  // every row it shares with the tall main monitor misaligns). Only when
  // nothing connects it sideways does it pull flush under its column
  // neighbors (a vertical gap there would be a real hole), or anchor to the
  // top edge if fully disconnected.
  const minY = Math.min(...out.map(m => m.y));
  const placedY: VirtualMonitor[] = [];
  for (const m of [...out].sort((a, b) => a.y - b.y || a.id - b.id)) {
    const sideConnected = placedY.some(p => yOverlap(p, m));
    if (!sideConnected) {
      const colNeighbors = placedY.filter(p => xOverlap(p, m));
      if (colNeighbors.length) {
        m.y = Math.max(...colNeighbors.map(p => p.y + p.height));
      } else if (placedY.length) {
        m.y = minY;
      }
    }
    placedY.push(m);
  }

  return out;
}

/**
 * Makes a raw arrangement valid for RDP: even dimensions, no overlaps (later
 * monitors are shoved right past earlier ones), no dead gaps (monitors packed
 * flush — RDP desktops must be contiguous), and the bounding box translated so
 * its top-left sits at the origin. The primary is treated as the anchor and
 * placed first, so it never moves relative to the others.
 */
export function normalizeToValidRdpLayout(
  monitors: VirtualMonitor[]
): VirtualMonitor[] {
  // Even the dimensions up front.
  const sized = monitors.map(m => ({
    ...m,
    width: evenDim(m.width),
    height: evenDim(m.height),
  }));

  // De-overlap: place the primary first, then the rest in id order. Any monitor
  // overlapping an already-placed one is shifted right to clear it.
  const order = [...sized].sort((a, b) => {
    if (a.isPrimary !== b.isPrimary) return a.isPrimary ? -1 : 1;
    return a.id - b.id;
  });
  const placed: VirtualMonitor[] = [];
  for (const m of order) {
    const cur = { ...m };
    // Repeat until it clears everything already placed (handles cascades).
    let moved = true;
    while (moved) {
      moved = false;
      for (const p of placed) {
        if (rectsOverlap(cur, p)) {
          cur.x = p.x + p.width;
          moved = true;
        }
      }
    }
    placed.push(cur);
  }

  // Pack out any dead gaps so the desktop is contiguous (Windows rejects a
  // monitor layout with a hole).
  const compacted = closeGaps(placed);

  // Translate so the bounding box top-left is the origin (no negatives).
  const minX = Math.min(...compacted.map(m => m.x));
  const minY = Math.min(...compacted.map(m => m.y));
  return compacted
    .map(m => ({ ...m, x: m.x - minX, y: m.y - minY }))
    .sort((a, b) => a.id - b.id);
}

/**
 * Derives each monitor's raw position from its physical position / manual
 * override (falling back to dense horizontal stacking when neither is
 * available). Dimensions are evened, but the result is NOT de-overlapped or
 * translated — use it to surface overlaps before they're normalized away.
 */
export function deriveRawLayout(placements: MonitorPlacement[]): VirtualMonitor[] {
  const primary = placements.find(p => p.isPrimary) ?? placements[0] ?? null;
  if (!primary) return [];

  const primaryRaw = primary.physical
    ? { x: primary.physical.left, y: primary.physical.top }
    : { x: 0, y: 0 };

  // First pass: position everything we can from physical / manual info.
  const raw = new Map<number, VirtualMonitor>();
  const fallback: MonitorPlacement[] = [];
  for (const p of placements) {
    const isPrimary = p.id === primary.id;
    const width = evenDim(p.cssWidth);
    const height = evenDim(p.cssHeight);
    if (isPrimary) {
      raw.set(p.id, { id: p.id, ...primaryRaw, width, height, isPrimary: true });
    } else if (p.manualOffset) {
      raw.set(p.id, {
        id: p.id,
        x: primaryRaw.x + p.manualOffset.x,
        y: primaryRaw.y + p.manualOffset.y,
        width,
        height,
        isPrimary: false,
      });
    } else if (p.physical && primary.physical) {
      raw.set(p.id, {
        id: p.id,
        x: p.physical.left,
        y: p.physical.top,
        width,
        height,
        isPrimary: false,
      });
    } else {
      fallback.push(p);
    }
  }

  // Second pass: stack any monitors we couldn't place to the right of the
  // current bounding box, in id order. Covers the no-Window-Management case.
  let cursorX = raw.size
    ? Math.max(...Array.from(raw.values(), m => m.x + m.width))
    : 0;
  for (const p of [...fallback].sort((a, b) => a.id - b.id)) {
    const width = evenDim(p.cssWidth);
    const height = evenDim(p.cssHeight);
    raw.set(p.id, {
      id: p.id,
      x: cursorX,
      y: 0,
      width,
      height,
      isPrimary: p.id === primary.id,
    });
    cursorX += width;
  }

  return Array.from(raw.values());
}

/**
 * Computes the RDP virtual-desktop layout from a set of monitor windows:
 * derive raw positions, normalize to a valid non-negative de-overlapped
 * layout, and report the bounding-box dimensions.
 */
export function computeRdpLayout(placements: MonitorPlacement[]): LayoutResult {
  const monitors = normalizeToValidRdpLayout(deriveRawLayout(placements));
  const bboxWidth = monitors.length
    ? Math.max(...monitors.map(m => m.x + m.width))
    : 0;
  const bboxHeight = monitors.length
    ? Math.max(...monitors.map(m => m.y + m.height))
    : 0;
  return { monitors, bboxWidth, bboxHeight };
}
