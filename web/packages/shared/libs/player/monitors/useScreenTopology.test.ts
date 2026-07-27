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

import { findDisplayForPoint, type DisplayInfo } from './useScreenTopology';

function display(p: Partial<DisplayInfo> & { id: string }): DisplayInfo {
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

const screens: DisplayInfo[] = [
  display({ id: 'a', left: 0, top: 0, isPrimary: true }),
  display({ id: 'b', left: 1920, top: 0 }),
];

describe('findDisplayForPoint', () => {
  it('returns the display containing the point', () => {
    expect(findDisplayForPoint(screens, 100, 100)?.id).toBe('a');
    expect(findDisplayForPoint(screens, 2000, 500)?.id).toBe('b');
  });

  it('treats the right edge as belonging to the next display', () => {
    // x=1920 is outside display "a" (0..1919) and inside "b".
    expect(findDisplayForPoint(screens, 1920, 0)?.id).toBe('b');
  });

  it('falls back to the nearest display for a point in a gap', () => {
    const gapped: DisplayInfo[] = [
      display({ id: 'a', left: 0, top: 0 }),
      display({ id: 'b', left: 3000, top: 0 }),
    ];
    // Point at x=2000 is past "a" but before "b"; closer to "a"'s center.
    expect(findDisplayForPoint(gapped, 2000, 500)?.id).toBe('a');
    // Point at x=3500 is inside "b".
    expect(findDisplayForPoint(gapped, 3500, 500)?.id).toBe('b');
  });

  it('returns null when there are no displays', () => {
    expect(findDisplayForPoint([], 0, 0)).toBeNull();
  });
});
