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

function main(): ModelMonitor {
  return {
    id: 0,
    role: 'main',
    cssWidth: 1920,
    cssHeight: 1080,
    isPrimary: true,
    physical: { left: 0, top: 0 },
    status: 'active',
  };
}

function popup(id: number, left: number): ModelMonitor {
  return {
    id,
    role: 'popup',
    cssWidth: 1920,
    cssHeight: 1080,
    isPrimary: false,
    physical: { left, top: 0 },
    status: 'active',
  };
}

describe('MonitorModel', () => {
  it('reports a single primary main monitor', () => {
    const m = new MonitorModel([main()], 3);
    const s = m.getState();
    expect(s.monitors).toHaveLength(1);
    expect(s.monitors[0]).toMatchObject({ id: 0, role: 'main', isPrimary: true });
    expect(s.bbox).toEqual({ width: 1920, height: 1080 });
    expect(s.arrangement).toBe('auto');
    expect(s.maxMonitors).toBe(3);
  });

  it('places a second display side by side from physical position', () => {
    const m = new MonitorModel([main()], 3);
    m.upsert(popup(1, 1920));
    const s = m.getState();
    expect(s.monitors).toHaveLength(2);
    expect(s.bbox).toEqual({ width: 3840, height: 1080 });
    const m1 = s.monitors.find(x => x.id === 1)!;
    expect(m1.rect).toMatchObject({ x: 1920, y: 0 });
  });

  it('shows a pending monitor in the view but excludes it from the layout', () => {
    const m = new MonitorModel([main()], 3);
    m.upsert({ ...popup(1, 1920), status: 'pending' });
    const s = m.getState();
    expect(s.monitors).toHaveLength(2);
    // bbox unchanged because the pending monitor isn't in the live layout yet.
    expect(s.bbox).toEqual({ width: 1920, height: 1080 });
    expect(s.monitors.find(x => x.id === 1)!.status).toBe('pending');
  });

  it('removes a popup but never the main window', () => {
    const m = new MonitorModel([main(), popup(1, 1920)], 3);
    expect(m.remove(1)).toBeDefined();
    expect(m.has(1)).toBe(false);
    expect(m.remove(0)).toBeUndefined();
    expect(m.has(0)).toBe(true);
  });

  it('moves the primary flag and re-anchors the layout', () => {
    const m = new MonitorModel([main(), popup(1, 1920)], 3);
    m.setPrimary(1);
    const s = m.getState();
    expect(s.monitors.find(x => x.id === 0)!.isPrimary).toBe(false);
    expect(s.monitors.find(x => x.id === 1)!.isPrimary).toBe(true);
    // The bounding box is unchanged; only which monitor Windows treats as
    // origin changes (handled server-side).
    expect(s.bbox).toEqual({ width: 3840, height: 1080 });
  });

  it('manual offset switches arrangement to manual and repositions', () => {
    const m = new MonitorModel([main(), popup(1, 1920)], 3);
    // Drag monitor 1 below the primary instead of beside it.
    m.setManualOffset(1, { x: 0, y: 1080 });
    const s = m.getState();
    expect(s.arrangement).toBe('manual');
    const m1 = s.monitors.find(x => x.id === 1)!;
    expect(m1.manual).toBe(true);
    expect(m1.rect).toMatchObject({ x: 0, y: 1080 });
    expect(s.bbox).toEqual({ width: 1920, height: 2160 });
  });

  it('returning to auto clears manual overrides', () => {
    const m = new MonitorModel([main(), popup(1, 1920)], 3);
    m.setManualOffset(1, { x: 0, y: 1080 });
    m.setArrangement('auto');
    const s = m.getState();
    expect(s.arrangement).toBe('auto');
    expect(s.monitors.find(x => x.id === 1)!.manual).toBe(false);
    // Back to the physical side-by-side arrangement.
    expect(s.bbox).toEqual({ width: 3840, height: 1080 });
  });

  it('allocates monotonically increasing ids above the initial set', () => {
    const m = new MonitorModel([main(), popup(2, 1920)], 3);
    expect(m.allocateId()).toBe(3);
    expect(m.allocateId()).toBe(4);
  });

  it('flags overlaps in the raw arrangement', () => {
    // Two monitors reporting the same physical origin (same display).
    const m = new MonitorModel([main(), popup(1, 0)], 3);
    expect(m.getState().overlaps).toEqual([[0, 1]]);
  });
});
