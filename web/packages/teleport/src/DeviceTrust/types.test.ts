/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { DeviceOrigin, deviceSource, DeviceSource } from './types';

describe('deviceSource', () => {
  const tests: Array<{
    name: string;
    source: DeviceSource | undefined;
    want: string;
  }> = [
    {
      name: 'default name for origin',
      source: { origin: DeviceOrigin.Intune, name: 'intune' },
      want: 'Intune',
    },
    {
      name: 'custom name',
      source: { origin: DeviceOrigin.Jamf, name: 'cool jamf' },
      want: 'cool jamf',
    },
    {
      name: 'no source',
      source: undefined,
      want: '',
    },
    {
      name: 'unsupported origin',
      source: { origin: 1337 as DeviceOrigin, name: 'even cooler jamf' },
      // Show the name instead of something like "unknown" as name is required and likely more
      // informative than displaying "unknown".
      want: 'even cooler jamf',
    },
  ];

  test.each(tests)('$name', ({ source, want }) => {
    expect(deviceSource(source)).toEqual(want);
  });
});
