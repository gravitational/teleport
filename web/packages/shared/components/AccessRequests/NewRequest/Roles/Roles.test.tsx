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

import { screen, within } from '@testing-library/react';

import { render } from 'design/utils/testing';

import { Roles } from './Roles';

test('renders requestable roles with a request option', async () => {
  render(
    <Roles
      requestable={['editor', 'access']}
      requested={new Set()}
      onToggleRole={() => {}}
    />
  );

  const row = await screen.findByRole('row', {
    name: /editor/i,
  });

  expect(
    await within(row).findByRole('button', {
      name: /request access/i,
    })
  ).toBeVisible();
});

test('renders requested roles with a remove option and requestable roles with an add option', async () => {
  render(
    <Roles
      requestable={['editor', 'access']}
      requested={new Set(['editor'])}
      onToggleRole={() => {}}
    />
  );

  const rowEditor = await screen.findByRole('row', {
    name: /editor/i,
  });
  expect(
    await within(rowEditor).findByRole('button', {
      name: /remove/i,
    })
  ).toBeVisible();

  const rowAccess = await screen.findByRole('row', {
    name: /access/i,
  });
  expect(
    await within(rowAccess).findByRole('button', {
      name: /add to request/i,
    })
  ).toBeVisible();
});
