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

import { MemoryRouter } from 'react-router';

import { render, screen } from 'design/utils/testing';

import { IntegrationsAddButton } from './IntegrationsAddButton';

test('is disables if all permissions are missing', async () => {
  render(
    <MemoryRouter>
      <IntegrationsAddButton
        requiredPermissions={[
          { value: false, label: 'permissions.1' },
          { value: false, label: 'permissions.2' },
        ]}
      />
    </MemoryRouter>
  );

  expect(
    screen.getByTitle('You do not have access to add new integrations')
  ).toBeInTheDocument();
});

test('is enabled if at least one permission is true', async () => {
  render(
    <MemoryRouter>
      <IntegrationsAddButton
        requiredPermissions={[
          { value: false, label: 'permissions.1' },
          { value: true, label: 'permissions.2' },
        ]}
      />
    </MemoryRouter>
  );

  expect(screen.getByText('Enroll New Integration')).toBeEnabled();
});
