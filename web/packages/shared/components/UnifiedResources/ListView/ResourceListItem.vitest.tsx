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

import { expect, test } from 'vitest';

import { makeGridTemplate } from './ResourceListItem';

test.each([
  {
    name: 'shows all columns',
    options: { checkbox: true, pin: true, scope: true },
    template: {
      columns: '22px 24px 36px 2fr 1.5fr 1fr 1fr 32px min-content',
      areas:
        "'checkbox pin icon name scope type address labels-btn warning-icon button' " +
        "'. . labels labels labels labels labels labels labels labels'",
    },
  },
  {
    name: 'hides the checkbox',
    options: { checkbox: false, pin: true, scope: true },
    template: {
      columns: '24px 36px 2fr 1.5fr 1fr 1fr 32px min-content',
      areas:
        "'pin icon name scope type address labels-btn warning-icon button' " +
        "'. labels labels labels labels labels labels labels labels'",
    },
  },
  {
    name: 'hides the pin',
    options: { checkbox: true, pin: false, scope: true },
    template: {
      columns: '22px 36px 2fr 1.5fr 1fr 1fr 32px min-content',
      areas:
        "'checkbox icon name scope type address labels-btn warning-icon button' " +
        "'. labels labels labels labels labels labels labels labels'",
    },
  },
  {
    name: 'hides the scope',
    options: { checkbox: true, pin: true, scope: false },
    template: {
      columns: '22px 24px 36px 2fr 1fr 1fr 32px min-content',
      areas:
        "'checkbox pin icon name type address labels-btn warning-icon button' " +
        "'. . labels labels labels labels labels labels labels'",
    },
  },
  {
    name: 'hides all possible sections',
    options: { checkbox: false, pin: false, scope: false },
    template: {
      columns: '36px 2fr 1fr 1fr 32px min-content',
      areas:
        "'icon name type address labels-btn warning-icon button' " +
        "'labels labels labels labels labels labels labels'",
    },
  },
])('makeGridTemplate $name', ({ options, template }) => {
  expect(makeGridTemplate(options)).toEqual(template);
});
