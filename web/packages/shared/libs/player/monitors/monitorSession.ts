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

import type { DisplayInfo } from './useScreenTopology';

/**
 * Transport-agnostic control surface for the live monitor set. The codec-test
 * harness implements this over its SharedWorker decoder; production
 * `DesktopSession` can implement the same interface over `TdpClient`, so the UI
 * and the physical-tracking hook graduate unchanged.
 */
export interface MonitorSession {
  getState(): MonitorSessionState;
  /** Subscribe to state changes; returns an unsubscribe. */
  subscribe(cb: (state: MonitorSessionState) => void): () => void;
  /**
   * Open a new monitor window, optionally targeting a physical display.
   * `placement` (FRAME bounds, virtual-screen CSS px — e.g. a half/quarter
   * zone picked on the layout map) overrides the automatic free-strip
   * placement.
   */
  addMonitor(
    display?: DisplayInfo,
    placement?: { left: number; top: number; width: number; height: number }
  ): void;
  /** Close a monitor window. The main window (role 'main') cannot be removed. */
  removeMonitor(id: number): void;
  /** Make a monitor the primary; Windows places the taskbar / origin there. */
  setPrimary(id: number): void;
  /**
   * Pin a monitor at a manual offset (CSS px, relative to primary). Pass null
   * to release it back to auto-from-physical placement.
   */
  setManualOffset(id: number, offset: { x: number; y: number } | null): void;
  /**
   * Physically move a monitor's OS window so its CONTENT (viewport) top-left
   * lands at the given virtual-screen position (CSS px). Only popups can be
   * moved — `moveTo` works on script-opened windows; the main browser window
   * is a no-op. The position tracker feeds the move back into the layout.
   */
  moveMonitorWindow(id: number, pos: { left: number; top: number }): void;
  /**
   * Move AND resize a monitor's OS window to the given FRAME bounds
   * (virtual-screen CSS px) — the apply step of an OS-style snap zone
   * (halves/quarters). Popups only, like `moveMonitorWindow`.
   */
  setMonitorWindowBounds(
    id: number,
    bounds: { left: number; top: number; width: number; height: number }
  ): void;
  /**
   * Bracket an interactive drag/resize from the layout panel. While active,
   * layout changes apply locally (model + canvas viewports) but the monitor
   * layout is NOT pushed to the server — a mid-drag stream of DisplayControl
   * resizes makes Windows reset graphics continuously (black rects, torn
   * paint). `endMonitorInteraction` commits the final layout once and
   * requests repaints to heal anything the resets left behind.
   */
  beginMonitorInteraction(): void;
  endMonitorInteraction(): void;
  /** 'auto' tracks physical window positions; 'manual' freezes the layout. */
  setArrangement(mode: ArrangementMode): void;
  /** Re-derive a clean layout from current physical positions (drops overrides). */
  tidy(): void;
  /**
   * Reopen the monitors saved from a previous session (one-click restore). Must
   * be invoked from a user gesture so the popup `window.open`s aren't blocked.
   * No-op when there is nothing to restore.
   */
  restoreMonitors(): void;
}

export type ArrangementMode = 'auto' | 'manual';

export type MonitorStatus = 'active' | 'pending' | 'blocked';

/** One monitor as seen by the management UI. Geometry is CSS px. */
export interface ManagedMonitorView {
  id: number;
  role: 'main' | 'popup';
  isPrimary: boolean;
  status: MonitorStatus;
  /** Window inner size (the RDP monitor's logical resolution). */
  cssWidth: number;
  cssHeight: number;
  /** Window top-left in OS virtual-screen coords, when known. */
  physical?: { left: number; top: number };
  /** Which physical display this window currently sits on, when known. */
  displayId?: string | null;
  /** True when the user has pinned this monitor with a manual offset. */
  manual: boolean;
  /** Computed position in the bbox-relative virtual desktop (CSS px). */
  rect: { x: number; y: number; width: number; height: number };
}

export interface MonitorSessionState {
  monitors: ManagedMonitorView[];
  bbox: { width: number; height: number };
  arrangement: ArrangementMode;
  maxMonitors: number;
  /** Overlapping id pairs in the raw arrangement (normalized away before send). */
  overlaps: Array<[number, number]>;
  /**
   * Number of saved popup monitors available to restore this session — drives
   * the taskbar's "Restore N monitors" button. 0 once restored, or once the
   * user changes the layout themselves (the offer is then forgotten).
   */
  restorable: number;
}
