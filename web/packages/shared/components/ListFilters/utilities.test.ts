/*
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

import { applyFilters, FilterMap } from '.';

type Item = { id: number; tag: string };

type Values = {
  Tag?: string;
  ID?: number;
};

const items: Item[] = [
  { id: 1, tag: 'a' },
  { id: 2, tag: 'b' },
  { id: 3, tag: 'a' },
];

describe('applyFilters', () => {
  it('applies a single filter to the list', () => {
    const filters: FilterMap<Item, Values> = {
      Tag: {
        options: [],
        selected: ['a'],
        apply: (l, s) => l.filter(i => s.includes(i.tag)),
      },
    };

    expect(applyFilters(items, filters)).toEqual([items[0], items[2]]);
  });

  it('applies a multiple filters to the list', () => {
    const filters: FilterMap<Item, Values> = {
      Tag: {
        options: [],
        selected: ['a'],
        apply: (l, s) => l.filter(i => s.includes(i.tag)),
      },
      ID: {
        options: [],
        selected: [3],
        apply: (l, s) => l.filter(i => s.includes(i.id)),
      },
    };

    expect(applyFilters(items, filters)).toEqual([items[2]]);
  });

  it('skips filters with no selected values', () => {
    const filters: FilterMap<Item, Values> = {
      Tag: {
        options: [],
        selected: ['a'],
        apply: (l, s) => l.filter(i => s.includes(i.tag)),
      },
      ID: {
        options: [],
        selected: [],
        apply: jest.fn(),
      },
    };

    applyFilters(items, filters);

    expect(filters.ID.apply).not.toHaveBeenCalled();
  });
});
