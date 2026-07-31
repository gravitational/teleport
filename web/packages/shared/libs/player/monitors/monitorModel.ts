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
  computeRdpLayout,
  deriveRawLayout,
  detectOverlaps,
  type LayoutResult,
  type MonitorPlacement,
} from './monitorLayout';
import type {
  ArrangementMode,
  ManagedMonitorView,
  MonitorSessionState,
  MonitorStatus,
} from './monitorSession';

/** Internal mutable record for one monitor. Geometry is CSS px. */
export interface ModelMonitor {
  id: number;
  role: 'main' | 'popup';
  cssWidth: number;
  cssHeight: number;
  isPrimary: boolean;
  physical?: { left: number; top: number };
  displayId?: string | null;
  manualOffset?: { x: number; y: number };
  status: MonitorStatus;
}

/**
 * Pure, DOM-free state machine for the monitor set. Owns the monitor records
 * and arrangement mode; derives the RDP layout + UI view via the layout module.
 * Both the codec-test controller and the Storybook mock drive this so the
 * arrangement logic is tested once, in isolation.
 */
export class MonitorModel {
  private monitors = new Map<number, ModelMonitor>();
  private arrangement: ArrangementMode = 'auto';
  private idCounter: number;
  readonly maxMonitors: number;

  constructor(initial: ModelMonitor[], maxMonitors: number) {
    for (const m of initial) this.monitors.set(m.id, { ...m });
    this.maxMonitors = maxMonitors;
    this.idCounter = initial.reduce((mx, m) => Math.max(mx, m.id), -1) + 1;
  }

  list(): ModelMonitor[] {
    return [...this.monitors.values()].sort((a, b) => a.id - b.id);
  }
  get(id: number): ModelMonitor | undefined {
    return this.monitors.get(id);
  }
  has(id: number): boolean {
    return this.monitors.has(id);
  }
  count(): number {
    return this.monitors.size;
  }
  allocateId(): number {
    return this.idCounter++;
  }
  getArrangement(): ArrangementMode {
    return this.arrangement;
  }

  upsert(m: ModelMonitor): void {
    this.monitors.set(m.id, { ...m });
  }
  remove(id: number): ModelMonitor | undefined {
    const m = this.monitors.get(id);
    if (m?.role === 'main') return undefined; // the main window can't be removed
    this.monitors.delete(id);
    return m;
  }
  setStatus(id: number, status: MonitorStatus): void {
    const m = this.monitors.get(id);
    if (m) m.status = status;
  }
  updatePhysical(
    id: number,
    physical: { left: number; top: number },
    displayId?: string | null,
    size?: { width: number; height: number }
  ): void {
    const m = this.monitors.get(id);
    if (!m) return;
    m.physical = physical;
    if (displayId !== undefined) m.displayId = displayId;
    if (size) {
      m.cssWidth = size.width;
      m.cssHeight = size.height;
    }
  }
  setPrimary(id: number): void {
    if (!this.monitors.has(id)) return;
    for (const m of this.monitors.values()) m.isPrimary = m.id === id;
  }
  setManualOffset(id: number, offset: { x: number; y: number } | null): void {
    const m = this.monitors.get(id);
    if (!m) return;
    m.manualOffset = offset ?? undefined;
    if (offset) this.arrangement = 'manual';
  }
  setArrangement(mode: ArrangementMode): void {
    this.arrangement = mode;
    if (mode === 'auto') {
      for (const m of this.monitors.values()) m.manualOffset = undefined;
      return;
    }
    // Freeze the current derived offsets as manual overrides so subsequent
    // physical moves don't shift the layout.
    const { monitors: layout } = this.computeLayout();
    const primary = layout.find(m => m.isPrimary) ?? layout[0];
    if (!primary) return;
    for (const vm of layout) {
      const mm = this.monitors.get(vm.id);
      if (mm && !mm.isPrimary) {
        mm.manualOffset = { x: vm.x - primary.x, y: vm.y - primary.y };
      }
    }
  }
  tidy(): void {
    this.arrangement = 'auto';
    for (const m of this.monitors.values()) m.manualOffset = undefined;
  }

  /** Placements for the monitors that are part of the live layout (active). */
  private placements(): MonitorPlacement[] {
    return this.list()
      .filter(m => m.status === 'active')
      .map(m => ({
        id: m.id,
        cssWidth: m.cssWidth,
        cssHeight: m.cssHeight,
        isPrimary: m.isPrimary,
        physical: m.physical,
        manualOffset:
          this.arrangement === 'manual' ? m.manualOffset : undefined,
      }));
  }

  computeLayout(): LayoutResult {
    return computeRdpLayout(this.placements());
  }

  getState(): MonitorSessionState {
    const placements = this.placements();
    const { monitors: layout, bboxWidth, bboxHeight } =
      computeRdpLayout(placements);
    const overlaps = detectOverlaps(deriveRawLayout(placements));
    const rectById = new Map(layout.map(m => [m.id, m]));
    const monitors: ManagedMonitorView[] = this.list().map(m => {
      const r = rectById.get(m.id);
      return {
        id: m.id,
        role: m.role,
        isPrimary: m.isPrimary,
        status: m.status,
        cssWidth: m.cssWidth,
        cssHeight: m.cssHeight,
        physical: m.physical,
        displayId: m.displayId,
        manual: !!m.manualOffset,
        rect: r
          ? { x: r.x, y: r.y, width: r.width, height: r.height }
          : { x: 0, y: 0, width: m.cssWidth, height: m.cssHeight },
      };
    });
    return {
      monitors,
      bbox: { width: bboxWidth, height: bboxHeight },
      arrangement: this.arrangement,
      maxMonitors: this.maxMonitors,
      overlaps,
      // The model has no restore concept; the session controller overrides this
      // with the count of saved popups still pending restore.
      restorable: 0,
    };
  }
}
