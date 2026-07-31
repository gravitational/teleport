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

import type { CanvasRendererRef } from 'shared/components/CanvasRenderer';
import type { TdpClient } from 'shared/libs/tdp';

import { physViewport, type PhysRect } from '../monitors/monitorLayout';
import { MonitorModel, type ModelMonitor } from '../monitors/monitorModel';
import type {
  ArrangementMode,
  MonitorSession,
  MonitorSessionState,
} from '../monitors/monitorSession';
import type { DisplayInfo } from '../monitors/useScreenTopology';
import { routeBitmapFrame, type MonitorViewport } from './frameRouter';

/**
 * `TdpMonitorSession` implements the transport-agnostic {@link MonitorSession}
 * control surface over a single {@link TdpClient} on the classic canvas
 * (fast-path bitmap) rendering path — the graduation target the interface's
 * doc comment describes.
 *
 * The chosen boundary is **one client, N canvases, routed**:
 *   - a single `TdpClient`/WebSocket streams ONE framebuffer covering the whole
 *     monitor arrangement (the layout bounding box);
 *   - each monitor registers a `CanvasRenderer`; the session tracks that
 *     monitor's *physical* viewport (its slice of the framebuffer);
 *   - every inbound `BitmapFrame` is dispatched to the canvas(es) it overlaps
 *     via {@link routeBitmapFrame}, clipped and re-based to local coordinates.
 *
 * Popup *windows* (opening/moving/closing OS windows for popup monitors) are a
 * DOM concern and live in the React host; this class owns only the model, the
 * transport, and the canvas registry.
 *
 * NOTE — transport limitation: the master `ClientScreenSpec` carries a single
 * `{ width, height, scale }`, so we can only tell the server the bounding-box
 * size, not per-monitor rectangles. Pixels flow and route correctly, but the
 * Windows server sees one big monitor rather than a true multi-monitor surface
 * (no per-monitor taskbar / maximize seams). Real multi-monitor requires the
 * DisplayControl/MS-RDPEDISP multi-rect spec the codec branch sends; that is
 * the follow-up transport work behind this same interface. Search: TRANSPORT-TODO.
 */
export class TdpMonitorSession implements MonitorSession {
  private readonly model: MonitorModel;
  private readonly listeners = new Set<(s: MonitorSessionState) => void>();

  /** Registered canvases by monitor id (null until the host mounts one). */
  private readonly canvases = new Map<number, CanvasRendererRef | null>();
  /** Physical (framebuffer-px) viewport per monitor, recomputed each layout. */
  private viewports: MonitorViewport[] = [];

  /** While true, layout changes stay local — no screen-spec push (see docs). */
  private layoutHold = false;
  /** Signature of the last spec sent, so we never push a redundant resize. */
  private lastSentSig: string | null = null;

  constructor(
    private readonly client: TdpClient,
    private readonly opts: {
      initial: ModelMonitor[];
      maxMonitors: number;
      /** RDP scale percent (e.g. 100, 150, 200). Matches server applyScale. */
      scale: number;
    }
  ) {
    this.model = new MonitorModel(opts.initial, opts.maxMonitors);
    for (const m of opts.initial) this.canvases.set(m.id, null);
    this.wireClient();
    // Compute the initial viewports, but do NOT push a screen spec: the caller
    // hasn't connected yet, and the initial dimensions travel through
    // `client.connect({ screenSpec: getInitialScreenSpec() })`. Seed the dedup
    // signature so the first post-connect layout change isn't a redundant send.
    const { bboxWidth, bboxHeight } = this.recomputeViewports();
    this.lastSentSig = `${bboxWidth}x${bboxHeight}@${this.opts.scale}`;
  }

  /**
   * The screen spec for the initial `client.connect` — the bounding box of the
   * seed monitor layout. (Single-monitor spec; see the TRANSPORT-TODO note.)
   */
  getInitialScreenSpec(): { width: number; height: number; scale: number } {
    const { bboxWidth, bboxHeight } = this.model.computeLayout();
    return { width: bboxWidth, height: bboxHeight, scale: this.opts.scale };
  }

  // --- MonitorSession ------------------------------------------------------

  getState(): MonitorSessionState {
    return this.model.getState();
  }

  subscribe(cb: (state: MonitorSessionState) => void): () => void {
    this.listeners.add(cb);
    return () => this.listeners.delete(cb);
  }

  addMonitor(
    display?: DisplayInfo,
    placement?: { left: number; top: number; width: number; height: number }
  ): void {
    if (this.model.count() >= this.opts.maxMonitors) return;
    const id = this.model.allocateId();
    this.model.upsert({
      id,
      role: 'popup',
      cssWidth: placement?.width ?? (display ? Math.min(1920, display.width) : 1280),
      cssHeight:
        placement?.height ?? (display ? Math.min(1080, display.height) : 720),
      isPrimary: false,
      physical: placement
        ? { left: placement.left, top: placement.top }
        : display
          ? { left: display.left, top: display.top }
          : { left: this.rightEdge(), top: 0 },
      displayId: display?.id ?? null,
      // A popup starts 'pending'; the host flips it 'active' once its window +
      // canvas are registered (registerCanvas), then re-applies the layout.
      status: 'pending',
    });
    this.canvases.set(id, null);
    this.emit();
  }

  removeMonitor(id: number): void {
    const removed = this.model.remove(id);
    if (!removed) return; // main window can't be removed
    this.canvases.delete(id);
    this.applyLayout();
    this.emit();
  }

  setPrimary(id: number): void {
    this.model.setPrimary(id);
    this.applyLayout();
    this.emit();
  }

  setManualOffset(id: number, offset: { x: number; y: number } | null): void {
    this.model.setManualOffset(id, offset);
    this.applyLayout();
    this.emit();
  }

  moveMonitorWindow(id: number, pos: { left: number; top: number }): void {
    // The host performs the real `window.moveTo`; the topology tracker then
    // feeds the settled position back via `updatePhysical`. Update the model
    // optimistically so the layout map tracks the drag.
    this.model.updatePhysical(id, pos);
    this.applyLayout();
    this.emit();
  }

  setMonitorWindowBounds(
    id: number,
    bounds: { left: number; top: number; width: number; height: number }
  ): void {
    this.model.updatePhysical(
      id,
      { left: bounds.left, top: bounds.top },
      undefined,
      { width: bounds.width, height: bounds.height }
    );
    this.applyLayout();
    this.emit();
  }

  beginMonitorInteraction(): void {
    this.layoutHold = true;
  }

  endMonitorInteraction(): void {
    this.layoutHold = false;
    // Commit the final layout once and request repaints to heal anything the
    // suppressed mid-drag resizes left behind.
    this.lastSentSig = null;
    this.applyLayout();
  }

  setArrangement(mode: ArrangementMode): void {
    this.model.setArrangement(mode);
    this.applyLayout();
    this.emit();
  }

  tidy(): void {
    this.model.tidy();
    this.applyLayout();
    this.emit();
  }

  restoreMonitors(): void {
    // Restore is a host concern (it must re-open popup windows from a user
    // gesture); the session exposes the hook but has nothing to do here on its
    // own. The host re-seeds monitors via addMonitor + registerCanvas.
  }

  // --- Host wiring (not part of the interface) -----------------------------

  /**
   * Attach a monitor's live `CanvasRenderer`. Called by the host once the
   * window hosting that monitor has mounted. Flips a pending popup active and
   * re-applies the layout so its viewport starts receiving routed frames.
   */
  registerCanvas(id: number, ref: CanvasRendererRef): void {
    this.canvases.set(id, ref);
    if (this.model.get(id)?.status !== 'active') {
      this.model.setStatus(id, 'active');
    }
    this.applyLayout();
    this.emit();
  }

  /** Detach a monitor's canvas (window closed / unmounted). */
  unregisterCanvas(id: number): void {
    this.canvases.set(id, null);
  }

  /** The underlying model, for hosts that need direct read access. */
  getModel(): MonitorModel {
    return this.model;
  }

  /**
   * A monitor's physical (framebuffer-px) viewport — the slice of the whole
   * desktop it occupies. Hosts use it to size the canvas backing buffer and to
   * offset local mouse coordinates into whole-desktop space. Undefined until
   * the monitor is active and included in the layout.
   */
  getViewport(id: number): PhysRect | undefined {
    return this.viewports.find(v => v.id === id)?.phys;
  }

  // --- Internals -----------------------------------------------------------

  private emit(): void {
    const s = this.getState();
    for (const l of this.listeners) l(s);
  }

  private rightEdge(): number {
    return this.model
      .list()
      .reduce((mx, m) => Math.max(mx, (m.physical?.left ?? 0) + m.cssWidth), 0);
  }

  /**
   * Recompute each monitor's physical viewport from the model layout. Pure
   * (no transport); returns the bounding-box size for the screen spec.
   */
  private recomputeViewports(): { bboxWidth: number; bboxHeight: number } {
    const { monitors, bboxWidth, bboxHeight } = this.model.computeLayout();
    this.viewports = monitors.map(m => ({
      id: m.id,
      phys: physViewport(m, this.opts.scale),
    }));
    return { bboxWidth, bboxHeight };
  }

  /**
   * Recompute viewports and push the bounding-box screen spec to the server
   * (deduped; suppressed mid-drag).
   */
  private applyLayout(): void {
    const { bboxWidth, bboxHeight } = this.recomputeViewports();

    if (this.layoutHold) return;

    // TRANSPORT-TODO: single-monitor ClientScreenSpec — send the bbox size so
    // the server renders one framebuffer spanning the whole arrangement.
    const sig = `${bboxWidth}x${bboxHeight}@${this.opts.scale}`;
    if (sig === this.lastSentSig) return;
    this.lastSentSig = sig;
    this.client.sendClientScreenSpec({
      width: bboxWidth,
      height: bboxHeight,
      scale: this.opts.scale,
    });
  }

  /** Subscribe to the client's paint/pointer streams and fan them out. */
  private wireClient(): void {
    this.client.onBmpFrame(frame => {
      routeBitmapFrame(frame, this.viewports, (id, f) =>
        this.canvases.get(id)?.renderBitmapFrame(f)
      );
    });
    this.client.onPngFrame(frame => {
      // Legacy PNG path: route whole-frame by the monitor containing its
      // top-left (no seam clipping — PNG frames aren't tiled like fast-path).
      const vp = this.viewports.find(
        v =>
          frame.left >= v.phys.x &&
          frame.left < v.phys.x + v.phys.width &&
          frame.top >= v.phys.y &&
          frame.top < v.phys.y + v.phys.height
      );
      if (!vp) return;
      this.canvases.get(vp.id)?.renderPngFrame({
        ...frame,
        left: frame.left - vp.phys.x,
        top: frame.top - vp.phys.y,
        right: frame.right - vp.phys.x,
        bottom: frame.bottom - vp.phys.y,
      });
    });
    // Cursor: mirror onto every canvas (the OS cursor is shared across the
    // virtual desktop; per-monitor hit-testing is a refinement).
    this.client.onPointer(pointer => {
      for (const ref of this.canvases.values()) ref?.setPointer(pointer);
    });
    this.client.onReset(() => {
      for (const ref of this.canvases.values()) ref?.clear();
    });
  }
}
