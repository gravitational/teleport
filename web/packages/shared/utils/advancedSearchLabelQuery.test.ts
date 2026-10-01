/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { makeAdvancedSearchQueryForLabel } from './advancedSearchLabelQuery';

const label = { name: 'env', value: 'prod' };
const labelQuery = 'labels["env"] == "prod"';

test('adds the label to an empty query', () => {
  expect(makeAdvancedSearchQueryForLabel(label, {})).toBe(labelQuery);
});

test('adds the label after an existing query and a simple search', () => {
  expect(
    makeAdvancedSearchQueryForLabel(label, {
      query: 'labels["os"] == "linux"',
      search: 'web',
    })
  ).toBe(`labels["os"] == "linux" && search("web") && ${labelQuery}`);
});

test('does not repeat a label that is already in the query', () => {
  const query = `labels["os"] == "linux" && ${labelQuery}`;
  expect(makeAdvancedSearchQueryForLabel(label, { query })).toBe(query);
});

test('does not repeat a label that is the whole query', () => {
  expect(makeAdvancedSearchQueryForLabel(label, { query: labelQuery })).toBe(
    labelQuery
  );
});

test('keeps a simple search when the label is already in the query', () => {
  expect(
    makeAdvancedSearchQueryForLabel(label, { query: labelQuery, search: 'web' })
  ).toBe(`${labelQuery} && search("web")`);
});

test('still adds a label whose value only starts like an existing one', () => {
  const query = 'labels["env"] == "prod-eu"';
  expect(makeAdvancedSearchQueryForLabel(label, { query })).toBe(
    `${query} && ${labelQuery}`
  );
});
