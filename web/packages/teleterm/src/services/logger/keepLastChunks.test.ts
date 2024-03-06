/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { KeepLastChunks } from './keepLastChunks';

describe('keepLastChunks', () => {
  it('keeps last n chunks', () => {
    const stream = new KeepLastChunks(3);

    ['foo', 'bar', 'lorem', 'ipsum', 'dolor'].forEach(string => {
      stream.write(string);
    });
    stream.end();

    expect(stream.getChunks()).toEqual(['lorem', 'ipsum', 'dolor']);
  });
});
