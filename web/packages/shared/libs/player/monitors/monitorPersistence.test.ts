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

import type { ModelMonitor } from './monitorModel';
import {
  clearLayout,
  loadLayout,
  popupCount,
  saveLayout,
  serializeLayout,
  type SavedLayout,
} from './monitorPersistence';

const KEY = 'test:monitors';

function model(p: Partial<ModelMonitor> & { id: number }): ModelMonitor {
  return {
    role: 'popup',
    cssWidth: 1920,
    cssHeight: 1080,
    isPrimary: false,
    status: 'active',
    ...p,
  };
}

beforeEach(() => localStorage.clear());

describe('serializeLayout', () => {
  it('captures each monitor’s physical rect, role, primary, and manual offset', () => {
    const layout = serializeLayout(
      [
        model({
          id: 0,
          role: 'main',
          isPrimary: true,
          cssWidth: 3200,
          cssHeight: 1124,
          physical: { left: 0, top: 0 },
        }),
        model({
          id: 1,
          role: 'popup',
          cssWidth: 1920,
          cssHeight: 1078,
          physical: { left: 6400, top: 0 },
          manualOffset: { x: 3200, y: 0 },
        }),
      ],
      'manual'
    );
    expect(layout).toEqual<SavedLayout>({
      version: 1,
      arrangement: 'manual',
      monitors: [
        {
          role: 'main',
          isPrimary: true,
          physical: { left: 0, top: 0 },
          width: 3200,
          height: 1124,
          manualOffset: null,
        },
        {
          role: 'popup',
          isPrimary: false,
          physical: { left: 6400, top: 0 },
          width: 1920,
          height: 1078,
          manualOffset: { x: 3200, y: 0 },
        },
      ],
    });
  });

  it('records a missing physical position as null', () => {
    const layout = serializeLayout(
      [model({ id: 0, role: 'main', isPrimary: true, physical: undefined })],
      'auto'
    );
    expect(layout.monitors[0].physical).toBeNull();
  });
});

describe('popupCount', () => {
  it('counts only popup monitors (the windows a restore reopens)', () => {
    const layout = serializeLayout(
      [
        model({ id: 0, role: 'main', isPrimary: true }),
        model({ id: 1, role: 'popup' }),
        model({ id: 2, role: 'popup' }),
      ],
      'auto'
    );
    expect(popupCount(layout)).toBe(2);
  });
});

describe('save / load round-trip', () => {
  it('persists and reloads an identical layout', () => {
    const layout = serializeLayout(
      [
        model({ id: 0, role: 'main', isPrimary: true, physical: { left: 0, top: 0 } }),
        // two monitors on one screen: right half.
        model({
          id: 1,
          role: 'popup',
          cssWidth: 960,
          cssHeight: 1080,
          physical: { left: 960, top: 0 },
        }),
      ],
      'auto'
    );
    saveLayout(KEY, layout);
    expect(loadLayout(KEY)).toEqual(layout);
  });

  it('returns null when nothing is saved', () => {
    expect(loadLayout(KEY)).toBeNull();
  });

  it('clearLayout forgets the saved layout', () => {
    saveLayout(KEY, serializeLayout([model({ id: 0, role: 'main' })], 'auto'));
    clearLayout(KEY);
    expect(loadLayout(KEY)).toBeNull();
  });
});

describe('loadLayout validation', () => {
  it('rejects a future/old schema version', () => {
    localStorage.setItem(
      KEY,
      JSON.stringify({ version: 999, arrangement: 'auto', monitors: [] })
    );
    expect(loadLayout(KEY)).toBeNull();
  });

  it('rejects corrupt JSON', () => {
    localStorage.setItem(KEY, '{not json');
    expect(loadLayout(KEY)).toBeNull();
  });

  it('rejects a malformed monitor entry', () => {
    localStorage.setItem(
      KEY,
      JSON.stringify({
        version: 1,
        arrangement: 'auto',
        monitors: [{ role: 'popup', isPrimary: false, width: 'wide' }],
      })
    );
    expect(loadLayout(KEY)).toBeNull();
  });

  it('rejects an invalid arrangement mode', () => {
    localStorage.setItem(
      KEY,
      JSON.stringify({ version: 1, arrangement: 'diagonal', monitors: [] })
    );
    expect(loadLayout(KEY)).toBeNull();
  });
});
