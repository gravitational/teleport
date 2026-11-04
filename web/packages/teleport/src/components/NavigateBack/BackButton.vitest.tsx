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

import { createMemoryHistory } from 'history';
import { Router } from 'react-router';
import { expect, test } from 'vitest';

import { render, screen, userEvent } from 'design/utils/testing';

import { BackButton } from './BackButton';

test('navigates to the previous history entry when one exists', async () => {
  const history = createMemoryHistory({
    initialEntries: ['/previous?search=banana', '/current'],
    initialIndex: 1,
  });

  render(
    <Router history={history}>
      <BackButton fallbackRoute="/fallback" />
    </Router>
  );

  await userEvent.click(screen.getByRole('button', { name: 'Back' }));

  expect(history.location.pathname).toBe('/previous');
  expect(history.location.search).toBe('?search=banana');
});

test('navigates to the fallback route when there is no previous history entry', async () => {
  const history = createMemoryHistory({ initialEntries: ['/current'] });
  // react-router v5 assigns every entry a key by default, including the first one.
  history.location.key = undefined;

  render(
    <Router history={history}>
      <BackButton fallbackRoute="/fallback" />
    </Router>
  );

  await userEvent.click(screen.getByRole('button', { name: 'Back' }));

  expect(history.location.pathname).toBe('/fallback');
});
