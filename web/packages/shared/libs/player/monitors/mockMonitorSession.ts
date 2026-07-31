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

import { MonitorModel, type ModelMonitor } from './monitorModel';
import type { MonitorSession, MonitorSessionState } from './monitorSession';
import {
  findDisplayForPoint,
  type DisplayInfo,
  type ScreenTopology,
  type TopologyPermission,
} from './useScreenTopology';

/**
 * In-memory `MonitorSession` used by Storybook (and available to tests). Wraps
 * a `MonitorModel` and synthesizes physical placement for added monitors so the
 * panel behaves end-to-end without a live decoder.
 */
export function createMockMonitorSession(
  initial: ModelMonitor[],
  maxMonitors = 4
): MonitorSession {
  const model = new MonitorModel(initial, maxMonitors);
  const listeners = new Set<(s: MonitorSessionState) => void>();
  const emit = () => {
    const s = model.getState();
    for (const l of listeners) l(s);
  };
  const rightEdge = () =>
    model
      .list()
      .reduce(
        (mx, m) => Math.max(mx, (m.physical?.left ?? 0) + m.cssWidth),
        0
      );

  return {
    getState: () => model.getState(),
    subscribe(cb) {
      listeners.add(cb);
      return () => listeners.delete(cb);
    },
    addMonitor(
      display?: DisplayInfo,
      placement?: { left: number; top: number; width: number; height: number }
    ) {
      if (model.count() >= maxMonitors) return;
      if (placement) {
        const id = model.allocateId();
        model.upsert({
          id,
          role: 'popup',
          cssWidth: placement.width,
          cssHeight: placement.height,
          isPrimary: false,
          physical: { left: placement.left, top: placement.top },
          displayId: display?.id ?? null,
          status: 'active',
        });
        emit();
        return;
      }
      const id = model.allocateId();
      model.upsert({
        id,
        role: 'popup',
        cssWidth: display ? Math.min(1920, display.width) : 1280,
        cssHeight: display ? Math.min(1080, display.height) : 720,
        isPrimary: false,
        physical: display
          ? { left: display.left, top: display.top }
          : { left: rightEdge(), top: 0 },
        displayId: display?.id ?? null,
        status: 'active',
      });
      emit();
    },
    removeMonitor(id) {
      model.remove(id);
      emit();
    },
    setPrimary(id) {
      model.setPrimary(id);
      emit();
    },
    setManualOffset(id, offset) {
      model.setManualOffset(id, offset);
      emit();
    },
    moveMonitorWindow(id, pos) {
      // No real OS window in the mock — apply the move to the model directly,
      // as the topology tracker would after a real `moveTo`.
      model.updatePhysical(id, pos);
      emit();
    },
    setMonitorWindowBounds(id, bounds) {
      // Frame == content in the mock (no chrome).
      model.updatePhysical(
        id,
        { left: bounds.left, top: bounds.top },
        undefined,
        { width: bounds.width, height: bounds.height }
      );
      emit();
    },
    beginMonitorInteraction() {
      // No server to shield in the mock.
    },
    endMonitorInteraction() {},
    setArrangement(mode) {
      model.setArrangement(mode);
      emit();
    },
    tidy() {
      model.tidy();
      emit();
    },
    restoreMonitors() {
      // The mock has no prior-session layout to restore.
    },
  };
}

/** Static `ScreenTopology` for stories/tests. */
export function createMockTopology(
  screens: DisplayInfo[],
  permission: TopologyPermission = 'granted'
): ScreenTopology {
  return {
    supported: permission !== 'unsupported',
    permission,
    screens: permission === 'granted' ? screens : [],
    requestPermission: async () => true,
    displayForPoint: (left, top) => findDisplayForPoint(screens, left, top),
    trackWindow: () => () => undefined,
  };
}

export function mockDisplay(p: Partial<DisplayInfo> & { id: string }): DisplayInfo {
  return {
    label: p.id,
    left: 0,
    top: 0,
    width: 1920,
    height: 1080,
    availLeft: 0,
    availTop: 0,
    isPrimary: false,
    isInternal: false,
    devicePixelRatio: 1,
    ...p,
  };
}
