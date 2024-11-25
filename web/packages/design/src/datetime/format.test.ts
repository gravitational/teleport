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

import { dateTimeShortFormat, displayDate, displayDateTime } from './format';

const testDate = new Date('2022-01-28T16:00:44.309Z');

test('displayDate', () => {
  const output = displayDate(testDate);

  expect(output).toBe('2022-01-28');
});

test('displayDateTime', () => {
  const output = displayDateTime(testDate);

  expect(output).toBe('2022-01-28 16:00:44');
});

test('dateTimeShortFormat', () => {
  expect(dateTimeShortFormat(testDate)).toEqual('4:00 PM');
});
