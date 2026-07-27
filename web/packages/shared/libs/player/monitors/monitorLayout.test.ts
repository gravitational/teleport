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

import {
  clampRectToDisplays,
  computeRdpLayout,
  contentScreenPosition,
  detectOverlaps,
  normalizeToValidRdpLayout,
  physViewport,
  pickSpawnRect,
  snapRect,
  snapZoneForPointer,
  toPhysical,
  type MonitorPlacement,
  type VirtualMonitor,
} from './monitorLayout';

// Helper: build a placement with sensible defaults.
function place(p: Partial<MonitorPlacement> & { id: number }): MonitorPlacement {
  return {
    cssWidth: 1920,
    cssHeight: 1080,
    isPrimary: false,
    ...p,
  };
}

function byId(monitors: VirtualMonitor[]): Map<number, VirtualMonitor> {
  return new Map(monitors.map(m => [m.id, m]));
}

describe('computeRdpLayout', () => {
  it('single monitor → one entry at origin, bbox = its size', () => {
    const { monitors, bboxWidth, bboxHeight } = computeRdpLayout([
      place({ id: 0, isPrimary: true, physical: { left: 100, top: 200 } }),
    ]);
    expect(monitors).toEqual([
      { id: 0, x: 0, y: 0, width: 1920, height: 1080, isPrimary: true },
    ]);
    expect(bboxWidth).toBe(1920);
    expect(bboxHeight).toBe(1080);
  });

  it('two side-by-side displays preserve the physical gap-free arrangement', () => {
    const { monitors, bboxWidth, bboxHeight } = computeRdpLayout([
      place({ id: 0, isPrimary: true, physical: { left: 0, top: 0 } }),
      place({ id: 1, physical: { left: 1920, top: 0 } }),
    ]);
    const m = byId(monitors);
    expect(m.get(0)).toMatchObject({ x: 0, y: 0, width: 1920, height: 1080 });
    expect(m.get(1)).toMatchObject({ x: 1920, y: 0, width: 1920, height: 1080 });
    expect(bboxWidth).toBe(3840);
    expect(bboxHeight).toBe(1080);
    expect(detectOverlaps(monitors)).toEqual([]);
  });

  it('preserves the vertical offset between side-by-side monitors', () => {
    // The main window's chrome (tab strip + toolbar) is taller than a
    // popup's, so with both OS windows top-aligned the popup's CONTENT sits
    // ~30px higher. That offset is real and must survive normalization —
    // flattening it misaligns the remote desktop across the seam. (Windows
    // accepts vertically offset side-by-side monitors; only disconnected
    // monitors need pulling together.)
    const { monitors, bboxHeight } = computeRdpLayout([
      place({
        id: 0,
        isPrimary: true,
        cssWidth: 1600,
        cssHeight: 900,
        physical: { left: 0, top: 160 },
      }),
      place({
        id: 1,
        cssWidth: 1280,
        cssHeight: 870,
        physical: { left: 1600, top: 130 },
      }),
    ]);
    const m = byId(monitors);
    // Popup content is 30px higher than main content: after origin
    // translation the popup sits at y=0 and the main at y=30.
    expect(m.get(0)).toMatchObject({ x: 0, y: 30 });
    expect(m.get(1)).toMatchObject({ x: 1600, y: 0 });
    expect(bboxHeight).toBe(930);
    expect(detectOverlaps(monitors)).toEqual([]);
  });

  it('keeps the y of a stacked monitor that is side-connected to a tall one', () => {
    // L-shape: tall main on the left, two popups stacked top-right and
    // bottom-right. The lower popup's content starts ~80px below the upper
    // popup's content bottom (its own chrome occupies that screen strip).
    // It must NOT be pulled flush under the upper popup: it already touches
    // the main monitor on its left (connected — no hole), and pulling it up
    // misaligns every row it shares with the main monitor across the seam.
    const { monitors, bboxHeight } = computeRdpLayout([
      place({
        id: 0,
        isPrimary: true,
        cssWidth: 1600,
        cssHeight: 1400,
        physical: { left: 0, top: 160 },
      }),
      place({
        id: 1,
        cssWidth: 1280,
        cssHeight: 660,
        physical: { left: 1600, top: 130 },
      }),
      place({
        id: 2,
        cssWidth: 1280,
        cssHeight: 660,
        physical: { left: 1600, top: 870 },
      }),
    ]);
    const m = byId(monitors);
    expect(m.get(0)).toMatchObject({ x: 0, y: 30 });
    expect(m.get(1)).toMatchObject({ x: 1600, y: 0 });
    // Real content offset preserved: 870 - 130 = 740, not flush at 660.
    expect(m.get(2)).toMatchObject({ x: 1600, y: 740 });
    expect(bboxHeight).toBe(1430);
    expect(detectOverlaps(monitors)).toEqual([]);
  });

  it('still pulls a stacked monitor flush when nothing connects it sideways', () => {
    // Two monitors stacked vertically with a chrome gap and NO side
    // neighbor: the gap would disconnect the desktop (a real hole), so the
    // lower one pulls flush under the upper.
    const { monitors } = computeRdpLayout([
      place({
        id: 0,
        isPrimary: true,
        cssWidth: 1280,
        cssHeight: 660,
        physical: { left: 0, top: 130 },
      }),
      place({
        id: 1,
        cssWidth: 1280,
        cssHeight: 660,
        physical: { left: 0, top: 870 },
      }),
    ]);
    const m = byId(monitors);
    expect(m.get(0)).toMatchObject({ x: 0, y: 0 });
    expect(m.get(1)).toMatchObject({ x: 0, y: 660 });
  });

  it('closes a horizontal gap left by a window narrower than its display', () => {
    // Repro: monitor 1's popup window (1920) is narrower than its physical
    // display, so monitor 2's screen position (6400) sits 1280px past where
    // monitor 1's window ends (5120). RDP desktops must be contiguous, so the
    // gap is packed out — monitor 2 moves flush to 5120.
    const { monitors, bboxWidth } = computeRdpLayout([
      place({
        id: 0,
        isPrimary: true,
        cssWidth: 3200,
        cssHeight: 1124,
        physical: { left: 0, top: 0 },
      }),
      place({
        id: 1,
        cssWidth: 1920,
        cssHeight: 1078,
        physical: { left: 3200, top: 0 },
      }),
      place({
        id: 2,
        cssWidth: 1920,
        cssHeight: 1078,
        physical: { left: 6400, top: 0 },
      }),
    ]);
    const m = byId(monitors);
    expect(m.get(0)).toMatchObject({ x: 0, width: 3200 });
    expect(m.get(1)).toMatchObject({ x: 3200, width: 1920 });
    expect(m.get(2)).toMatchObject({ x: 5120, width: 1920 }); // packed flush, not 6400
    expect(bboxWidth).toBe(7040); // no 1280px dead gap
    expect(detectOverlaps(monitors)).toEqual([]);
  });

  it('closes a vertical gap between stacked displays', () => {
    const { monitors, bboxHeight } = computeRdpLayout([
      place({
        id: 0,
        isPrimary: true,
        cssWidth: 1920,
        cssHeight: 1080,
        physical: { left: 0, top: 0 },
      }),
      // Physically below, but with a 500px vertical gap to the primary.
      place({
        id: 1,
        cssWidth: 1920,
        cssHeight: 1080,
        physical: { left: 0, top: 1580 },
      }),
    ]);
    const m = byId(monitors);
    expect(m.get(0)).toMatchObject({ x: 0, y: 0 });
    expect(m.get(1)).toMatchObject({ x: 0, y: 1080 }); // packed flush, not 1580
    expect(bboxHeight).toBe(2160);
    expect(detectOverlaps(monitors)).toEqual([]);
  });

  it('a display physically left of primary translates to a non-negative box', () => {
    // Second monitor sits at screen x=-1920 (to the left of the primary).
    const { monitors, bboxWidth } = computeRdpLayout([
      place({ id: 0, isPrimary: true, physical: { left: 0, top: 0 } }),
      place({ id: 1, physical: { left: -1920, top: 0 } }),
    ]);
    const m = byId(monitors);
    // Everything shifted right so the leftmost edge is x=0; nothing negative.
    expect(m.get(1)!.x).toBe(0);
    expect(m.get(0)!.x).toBe(1920);
    for (const mon of monitors) {
      expect(mon.x).toBeGreaterThanOrEqual(0);
      expect(mon.y).toBeGreaterThanOrEqual(0);
    }
    expect(bboxWidth).toBe(3840);
  });

  it('vertically stacked displays (second above primary)', () => {
    const { monitors, bboxWidth, bboxHeight } = computeRdpLayout([
      place({ id: 0, isPrimary: true, physical: { left: 0, top: 0 } }),
      place({ id: 1, physical: { left: 0, top: -1080 } }),
    ]);
    const m = byId(monitors);
    expect(m.get(1)).toMatchObject({ x: 0, y: 0 });
    expect(m.get(0)).toMatchObject({ x: 0, y: 1080 });
    expect(bboxWidth).toBe(1920);
    expect(bboxHeight).toBe(2160);
  });

  it('different-sized displays side by side', () => {
    const { monitors, bboxWidth, bboxHeight } = computeRdpLayout([
      place({ id: 0, isPrimary: true, physical: { left: 0, top: 0 } }),
      place({
        id: 1,
        cssWidth: 1280,
        cssHeight: 1024,
        physical: { left: 1920, top: 0 },
      }),
    ]);
    const m = byId(monitors);
    expect(m.get(1)).toMatchObject({ x: 1920, y: 0, width: 1280, height: 1024 });
    expect(bboxWidth).toBe(3200);
    expect(bboxHeight).toBe(1080);
  });

  it('odd dimensions are rounded down to even', () => {
    const { monitors } = computeRdpLayout([
      place({
        id: 0,
        isPrimary: true,
        cssWidth: 1921,
        cssHeight: 1081,
        physical: { left: 0, top: 0 },
      }),
    ]);
    expect(monitors[0]).toMatchObject({ width: 1920, height: 1080 });
  });

  it('two monitors on the same physical display overlap, then de-overlap', () => {
    // Hybrid: a second monitor added onto a display already in use → both
    // windows report the same screen origin.
    const placements = [
      place({ id: 0, isPrimary: true, physical: { left: 0, top: 0 } }),
      place({ id: 1, physical: { left: 0, top: 0 } }),
    ];
    // The raw arrangement overlaps...
    const raw = placements.map<VirtualMonitor>(p => ({
      id: p.id,
      x: 0,
      y: 0,
      width: p.cssWidth,
      height: p.cssHeight,
      isPrimary: p.isPrimary,
    }));
    expect(detectOverlaps(raw)).toEqual([[0, 1]]);

    // ...but the computed RDP layout shoves the second one clear.
    const { monitors } = computeRdpLayout(placements);
    expect(detectOverlaps(monitors)).toEqual([]);
    const m = byId(monitors);
    expect(m.get(1)!.x).toBeGreaterThanOrEqual(m.get(0)!.width);
  });

  it('manual override wins over physical position', () => {
    const { monitors } = computeRdpLayout([
      place({ id: 0, isPrimary: true, physical: { left: 0, top: 0 } }),
      // Physically on top of primary, but manually placed to the right.
      place({
        id: 1,
        physical: { left: 0, top: 0 },
        manualOffset: { x: 1920, y: 0 },
      }),
    ]);
    const m = byId(monitors);
    expect(m.get(1)).toMatchObject({ x: 1920, y: 0 });
    expect(detectOverlaps(monitors)).toEqual([]);
  });

  it('no flagged primary → first monitor is treated as primary', () => {
    const { monitors } = computeRdpLayout([
      place({ id: 0, physical: { left: 0, top: 0 } }),
      place({ id: 1, physical: { left: 1920, top: 0 } }),
    ]);
    const m = byId(monitors);
    expect(m.get(0)!.isPrimary).toBe(true);
    expect(m.get(1)!.isPrimary).toBe(false);
    expect(monitors.filter(x => x.isPrimary)).toHaveLength(1);
  });

  it('falls back to dense horizontal stacking when no physical info', () => {
    const { monitors, bboxWidth } = computeRdpLayout([
      place({ id: 0, isPrimary: true }),
      place({ id: 1 }),
      place({ id: 2, cssWidth: 1280 }),
    ]);
    const m = byId(monitors);
    expect(m.get(0)).toMatchObject({ x: 0, y: 0 });
    expect(m.get(1)).toMatchObject({ x: 1920, y: 0 });
    expect(m.get(2)).toMatchObject({ x: 3840, y: 0 });
    expect(bboxWidth).toBe(1920 + 1920 + 1280);
    expect(detectOverlaps(monitors)).toEqual([]);
  });

  it('output is sorted by id (stable canvas registration order)', () => {
    const { monitors } = computeRdpLayout([
      place({ id: 2, physical: { left: 3840, top: 0 } }),
      place({ id: 0, isPrimary: true, physical: { left: 0, top: 0 } }),
      place({ id: 1, physical: { left: 1920, top: 0 } }),
    ]);
    expect(monitors.map(x => x.id)).toEqual([0, 1, 2]);
  });
});

describe('normalizeToValidRdpLayout', () => {
  it('evens dimensions and clamps to a minimum', () => {
    const out = normalizeToValidRdpLayout([
      { id: 0, x: 0, y: 0, width: 1, height: 1, isPrimary: true },
    ]);
    expect(out[0].width).toBeGreaterThanOrEqual(2);
    expect(out[0].height).toBeGreaterThanOrEqual(2);
    expect(out[0].width % 2).toBe(0);
    expect(out[0].height % 2).toBe(0);
  });

  it('translates so the bounding box top-left is at the origin', () => {
    const out = normalizeToValidRdpLayout([
      { id: 0, x: 100, y: 50, width: 800, height: 600, isPrimary: true },
      { id: 1, x: -200, y: -100, width: 800, height: 600, isPrimary: false },
    ]);
    const minX = Math.min(...out.map(m => m.x));
    const minY = Math.min(...out.map(m => m.y));
    expect(minX).toBe(0);
    expect(minY).toBe(0);
  });
});

describe('toPhysical / physViewport', () => {
  it('scales by max(scale,100)/100 and floors', () => {
    expect(toPhysical(1920, 100)).toBe(1920);
    expect(toPhysical(1920, 200)).toBe(3840);
    expect(toPhysical(1920, 150)).toBe(2880);
    // scale below 100 is clamped to 100 (no downscaling of the framebuffer).
    expect(toPhysical(1920, 50)).toBe(1920);
    // floors fractional results.
    expect(toPhysical(101, 150)).toBe(151);
  });

  it('converts a whole monitor viewport', () => {
    expect(
      physViewport(
        { id: 1, x: 1920, y: 0, width: 1280, height: 1024, isPrimary: false },
        200
      )
    ).toEqual({ x: 3840, y: 0, width: 2560, height: 2048 });
  });
});

describe('contentScreenPosition', () => {
  it('offsets the frame top by the chrome height (outer - inner)', () => {
    // A main window with full chrome (tabs + toolbar = 110px) and a popup
    // with a slim bar (80px) whose OS frames are top-aligned: content
    // positions must differ by the chrome delta so the seam lines up.
    const main = {
      screenLeft: 0,
      screenTop: 50,
      outerHeight: 900,
      innerHeight: 790,
    } as Window;
    const popup = {
      screenLeft: 1280,
      screenTop: 50,
      outerHeight: 900,
      innerHeight: 820,
    } as Window;
    expect(contentScreenPosition(main)).toEqual({ left: 0, top: 160 });
    expect(contentScreenPosition(popup)).toEqual({ left: 1280, top: 130 });
  });

  it('never produces a negative chrome offset', () => {
    // Defensive: a fullscreen/kiosk window can report outer == inner (or a
    // transiently smaller outer during resize); clamp at the frame top.
    const win = {
      screenLeft: 10,
      screenTop: 20,
      outerHeight: 700,
      innerHeight: 720,
    } as Window;
    expect(contentScreenPosition(win)).toEqual({ left: 10, top: 20 });
  });
});

describe('snapRect', () => {
  const anchor = { left: 0, top: 0, width: 1920, height: 1080 };

  it('snaps flush right-of an anchor within the threshold', () => {
    const dragged = { left: 1928, top: 200, width: 1280, height: 720 };
    expect(snapRect(dragged, [anchor], 16)).toEqual({ left: 1920, top: 200 });
  });

  it('snaps flush left-of an anchor (dragged right edge to anchor left)', () => {
    const dragged = { left: -1285, top: 200, width: 1280, height: 720 };
    expect(snapRect(dragged, [anchor], 16)).toEqual({ left: -1280, top: 200 });
  });

  it('aligns matching edges (top-align while beside the anchor)', () => {
    const dragged = { left: 1920, top: 9, width: 1280, height: 720 };
    expect(snapRect(dragged, [anchor], 16)).toEqual({ left: 1920, top: 0 });
  });

  it('snaps both axes at once (corner)', () => {
    const dragged = { left: 1925, top: 1085, width: 1280, height: 720 };
    expect(snapRect(dragged, [anchor], 16)).toEqual({ left: 1920, top: 1080 });
  });

  it('does not snap outside the threshold', () => {
    const dragged = { left: 1950, top: 333, width: 1280, height: 720 };
    expect(snapRect(dragged, [anchor], 16)).toEqual({ left: 1950, top: 333 });
  });

  it('prefers the nearest candidate across anchors', () => {
    const other = { left: 3210, top: 0, width: 1280, height: 720 };
    // 5px from other's left-align candidate (3210); every anchor candidate
    // is far outside the threshold. The nearest in-threshold candidate wins.
    const dragged = { left: 3205, top: 400, width: 1280, height: 720 };
    expect(snapRect(dragged, [anchor, other], 16).left).toBe(3210);
  });
});

describe('snapZoneForPointer', () => {
  const display = { left: 0, top: 0, width: 2560, height: 1440 };

  it('maps the left band to the left half (proportional band)', () => {
    // 600 is well inside the left third (2560/3 ≈ 853) but far from the
    // edge in pixels — bands are proportional so zones are easy to hit.
    expect(snapZoneForPointer(600, 700, display, 1 / 3)).toEqual({
      left: 0,
      top: 0,
      width: 1280,
      height: 1440,
    });
  });

  it('maps the top band to the top half', () => {
    expect(snapZoneForPointer(1300, 200, display, 1 / 3)).toEqual({
      left: 0,
      top: 0,
      width: 2560,
      height: 720,
    });
  });

  it('maps a corner region to the quarter', () => {
    expect(snapZoneForPointer(2400, 1200, display, 1 / 3)).toEqual({
      left: 1280,
      top: 720,
      width: 1280,
      height: 720,
    });
  });

  it('returns null in the center region and off-display', () => {
    expect(snapZoneForPointer(1280, 720, display, 1 / 3)).toBeNull();
    expect(snapZoneForPointer(-10, 700, display, 1 / 3)).toBeNull();
  });

  it('narrower bands need the pointer closer to the edge', () => {
    // 600 is outside a 15% band (384px) — no zone for drag-style snapping.
    expect(snapZoneForPointer(600, 700, display, 0.15)).toBeNull();
    expect(snapZoneForPointer(200, 700, display, 0.15)).toEqual({
      left: 0,
      top: 0,
      width: 1280,
      height: 1440,
    });
  });
});

describe('clampRectToDisplays', () => {
  const left = { left: 0, top: 0, width: 2560, height: 1440 };
  const right = { left: 2560, top: 200, width: 1920, height: 1080 };

  it('clamps a rect dragged past the display edge', () => {
    expect(
      clampRectToDisplays(
        { left: -300, top: -50, width: 1280, height: 720 },
        [left, right]
      )
    ).toEqual({ left: 0, top: 0 });
  });

  it('clamps into the display with the dominant overlap when crossing', () => {
    // Mostly over the right display: clamp into IT (top must come down to
    // its bounds), not back onto the left display.
    expect(
      clampRectToDisplays(
        { left: 2700, top: 100, width: 1280, height: 720 },
        [left, right]
      )
    ).toEqual({ left: 2700, top: 200 });
  });

  it('pulls a rect fully in the void back to the nearest display', () => {
    expect(
      clampRectToDisplays(
        { left: 6000, top: 5000, width: 1280, height: 720 },
        [left, right]
      )
    ).toEqual({ left: 3200, top: 560 });
  });

  it('is a no-op when no displays are known', () => {
    expect(
      clampRectToDisplays({ left: 9999, top: -9999, width: 100, height: 100 }, [])
    ).toEqual({ left: 9999, top: -9999 });
  });
});

describe('pickSpawnRect', () => {
  const desired = { width: 1920, height: 1080 };

  it('shrinks into the free strip beside a large main window', () => {
    // Main covers the left 1600px of a 2560-wide display: the right strip is
    // 870px wide after the gap — the popup must shrink into it, not overlap.
    const r = pickSpawnRect({
      main: { left: 0, top: 0, width: 1600, height: 1400 },
      bounds: { left: 0, top: 0, width: 2560, height: 1440 },
      desired,
    });
    expect(r).toEqual({ left: 1690, top: 0, width: 870, height: 1080 });
  });

  it('uses the display origin when main is on another display', () => {
    const r = pickSpawnRect({
      main: { left: -2000, top: 0, width: 1500, height: 900 },
      bounds: { left: 0, top: 25, width: 2560, height: 1415 },
      desired,
    });
    expect(r).toEqual({ left: 0, top: 25, width: 1920, height: 1080 });
  });

  it('cascades (clamped) when the main window fills the display', () => {
    const r = pickSpawnRect({
      main: { left: 0, top: 25, width: 2560, height: 1415 },
      bounds: { left: 0, top: 25, width: 2560, height: 1415 },
      desired,
    });
    // Overlap unavoidable; clamped into the display.
    expect(r.width).toBe(1920);
    expect(r.left + r.width).toBeLessThanOrEqual(2560);
    expect(r.top).toBeGreaterThanOrEqual(25);
  });

  it('opens beside main when no display info exists', () => {
    const r = pickSpawnRect({
      main: { left: 100, top: 50, width: 1512, height: 900 },
      bounds: null,
      desired,
    });
    expect(r).toEqual({ left: 1702, top: 50, width: 1920, height: 1080 });
  });
});

describe('detectOverlaps', () => {
  it('treats edge-touching rectangles as non-overlapping', () => {
    expect(
      detectOverlaps([
        { id: 0, x: 0, y: 0, width: 100, height: 100, isPrimary: true },
        { id: 1, x: 100, y: 0, width: 100, height: 100, isPrimary: false },
      ])
    ).toEqual([]);
  });

  it('reports each overlapping pair once', () => {
    expect(
      detectOverlaps([
        { id: 0, x: 0, y: 0, width: 100, height: 100, isPrimary: true },
        { id: 1, x: 50, y: 50, width: 100, height: 100, isPrimary: false },
      ])
    ).toEqual([[0, 1]]);
  });
});
