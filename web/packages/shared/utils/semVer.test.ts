/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { compareSemVers } from './semVer';

test('compareSemVers', () => {
  expect(['3.0.0', '1.0.0', '2.0.0'].sort(compareSemVers)).toEqual([
    '1.0.0',
    '2.0.0',
    '3.0.0',
  ]);

  expect(['3.1.0', '3.2.0', '3.1.1'].sort(compareSemVers)).toEqual([
    '3.1.0',
    '3.1.1',
    '3.2.0',
  ]);

  expect(['10.0.1', '10.0.2', '2.0.0'].sort(compareSemVers)).toEqual([
    '2.0.0',
    '10.0.1',
    '10.0.2',
  ]);

  expect(['10.1.0', '11.1.0', '5.10.10'].sort(compareSemVers)).toEqual([
    '5.10.10',
    '10.1.0',
    '11.1.0',
  ]);

  expect(compareSemVers('1.0.0', '1.0.0')).toBe(0);
});
