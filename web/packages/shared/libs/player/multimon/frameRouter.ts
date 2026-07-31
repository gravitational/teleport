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

import type { BitmapFrame } from 'shared/libs/tdp';

import type { PhysRect } from '../monitors/monitorLayout';

/**
 * One canvas the router can paint into: the monitor's id and its viewport
 * (the slice of the whole virtual-desktop framebuffer that this monitor
 * occupies, in physical/framebuffer pixels — see `physViewport`).
 */
export interface MonitorViewport {
  id: number;
  phys: PhysRect;
}

/**
 * Sink for a routed slice: a `BitmapFrame` whose coordinates have been
 * translated into `id`'s local canvas space (0,0 = that monitor's top-left).
 * Wire this to `CanvasRendererRef.renderBitmapFrame`.
 */
export type FrameSink = (id: number, frame: BitmapFrame) => void;

/**
 * Route a single fast-path `BitmapFrame` — addressed in whole-virtual-desktop
 * framebuffer coordinates — to the per-monitor canvases it touches.
 *
 * This is the canvas-path equivalent of the wasm decoder's multi-viewport
 * paint: on the "one client, N canvases" boundary the server streams ONE
 * framebuffer covering the whole arrangement (bbox), and each `BitmapFrame`
 * must be dispatched to whichever monitor(s) its rectangle overlaps, clipped
 * to the seam and re-based to that canvas's local origin.
 *
 * Most fast-path tiles land wholly inside one monitor (the common, zero-copy
 * path). A tile straddling a monitor seam is split: each covered monitor gets
 * the intersecting sub-rectangle.
 */
export function routeBitmapFrame(
  frame: BitmapFrame,
  viewports: MonitorViewport[],
  sink: FrameSink
): void {
  const img = frame.image_data;
  const fx = frame.left;
  const fy = frame.top;
  const fw = img.width;
  const fh = img.height;

  for (const { id, phys } of viewports) {
    // Intersection of the frame rect with this monitor's viewport, in world
    // (framebuffer) coordinates.
    const ix0 = Math.max(fx, phys.x);
    const iy0 = Math.max(fy, phys.y);
    const ix1 = Math.min(fx + fw, phys.x + phys.width);
    const iy1 = Math.min(fy + fh, phys.y + phys.height);
    if (ix1 <= ix0 || iy1 <= iy0) {
      continue; // no overlap with this monitor
    }

    // Fast path: the whole tile sits inside this viewport — no pixel copy,
    // just re-base the origin to the monitor's local canvas space.
    if (ix0 === fx && iy0 === fy && ix1 === fx + fw && iy1 === fy + fh) {
      sink(id, { left: fx - phys.x, top: fy - phys.y, image_data: img });
      continue;
    }

    // Straddling tile: copy out the intersecting sub-rectangle.
    const sw = ix1 - ix0;
    const sh = iy1 - iy0;
    const sub = cropImageData(img, ix0 - fx, iy0 - fy, sw, sh);
    sink(id, { left: ix0 - phys.x, top: iy0 - phys.y, image_data: sub });
  }
}

/**
 * Copy a sub-rectangle out of an `ImageData` into a fresh, tightly-packed
 * `ImageData`. Used only for tiles that cross a monitor seam.
 */
export function cropImageData(
  src: ImageData,
  sx: number,
  sy: number,
  sw: number,
  sh: number
): ImageData {
  const out = new ImageData(sw, sh, { colorSpace: src.colorSpace });
  const srcData = src.data;
  const dstData = out.data;
  const srcStride = src.width * 4;
  const dstStride = sw * 4;
  for (let row = 0; row < sh; row++) {
    const srcStart = (sy + row) * srcStride + sx * 4;
    const dstStart = row * dstStride;
    dstData.set(srcData.subarray(srcStart, srcStart + dstStride), dstStart);
  }
  return out;
}
