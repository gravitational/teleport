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

import { formatDuration } from './formatDuration';

describe('formatDuration', () => {
  it.each`
    seconds  | expected
    ${0}     | ${'0s'}
    ${1}     | ${'1s'}
    ${1.5}   | ${'1s'}
    ${60}    | ${'1m'}
    ${3600}  | ${'1h'}
    ${86400} | ${'24h'}
    ${12345} | ${'3h25m45s'}
  `('formats $seconds seconds as $expected', ({ seconds, expected }) => {
    expect(formatDuration({ seconds })).toBe(expected);
  });

  it.each`
    seconds  | expected
    ${0}     | ${'0s'}
    ${1}     | ${'1s'}
    ${12345} | ${'3h|25m|45s'}
  `('formats $seconds seconds as $expected', ({ seconds, expected }) => {
    expect(formatDuration({ seconds }, { separator: '|' })).toBe(expected);
  });
});
