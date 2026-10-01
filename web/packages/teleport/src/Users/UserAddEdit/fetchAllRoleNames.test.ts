/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { fetchAllRoleNames } from './fetchAllRoleNames';

function pagesOf(...pages: string[][]) {
  return jest.fn(async ({ startKey }: { startKey: string }) => {
    const index = startKey ? Number(startKey) : 0;
    return {
      items: pages[index].map(name => ({ name })),
      startKey: index + 1 < pages.length ? String(index + 1) : '',
    };
  });
}

test('returns the roles of a single page', async () => {
  const fetchPage = pagesOf(['a', 'b']);
  expect(await fetchAllRoleNames(fetchPage, '')).toEqual(['a', 'b']);
  expect(fetchPage).toHaveBeenCalledTimes(1);
});

test('follows the next key through every page, in order', async () => {
  const fetchPage = pagesOf(['a', 'b'], ['c'], ['d', 'e']);
  expect(await fetchAllRoleNames(fetchPage, '')).toEqual([
    'a',
    'b',
    'c',
    'd',
    'e',
  ]);
  expect(fetchPage.mock.calls.map(([params]) => params.startKey)).toEqual([
    '',
    '1',
    '2',
  ]);
});

test('sends the search term and page size with every request', async () => {
  const fetchPage = pagesOf(['a'], ['b']);
  await fetchAllRoleNames(fetchPage, 'dev');
  for (const [params] of fetchPage.mock.calls) {
    expect(params).toMatchObject({ search: 'dev', limit: 50 });
  }
});

test('returns nothing when no role matches', async () => {
  expect(await fetchAllRoleNames(pagesOf([]), 'zzz')).toEqual([]);
});

test('stops after a fixed number of pages if the server never ends', async () => {
  const fetchPage = jest.fn(async () => ({
    items: [{ name: 'r' }],
    startKey: 'more',
  }));
  const names = await fetchAllRoleNames(fetchPage, '');
  expect(fetchPage).toHaveBeenCalledTimes(40);
  expect(names).toHaveLength(40);
});

test('passes a fetch failure on to the caller', async () => {
  const fetchPage = jest.fn(async () => {
    throw new Error('boom');
  });
  await expect(fetchAllRoleNames(fetchPage, '')).rejects.toThrow('boom');
});
