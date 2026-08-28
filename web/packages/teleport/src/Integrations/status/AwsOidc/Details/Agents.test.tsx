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

import { within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';

import { render, screen, userEvent, waitFor } from 'design/utils/testing';

import { Route } from 'teleport/components/Router';
import cfg from 'teleport/config';
import { Agents } from 'teleport/Integrations/status/AwsOidc/Details/Agents';
import { integrationService } from 'teleport/services/integrations';

test('renders service name & labels from response', async () => {
  jest.spyOn(window, 'open').mockImplementation();

  jest
    .spyOn(integrationService, 'fetchAwsOidcDatabaseServices')
    .mockResolvedValue({
      services: [
        {
          name: 'dev-db',
          matchingLabels: [{ name: 'region', value: 'us-west-2' }],
          dashboardUrl: 'some-aws-url',
          validTeleportConfig: true,
        },
        {
          name: 'dev-db',
          matchingLabels: [
            { name: 'region', value: 'us-west-1' },
            { name: '*', value: '*' },
          ],
          dashboardUrl: 'some-aws-url',
          validTeleportConfig: true,
        },
        {
          name: 'staging-db',
          matchingLabels: [{ name: '*', value: '*' }],
          dashboardUrl: 'some-aws-url',
          validTeleportConfig: true,
        },
      ],
    });
  render(
    <MemoryRouter
      initialEntries={[
        `/web/integrations/status/aws-oidc/some-name/resources/rds?tab=agents`,
      ]}
    >
      <Route
        path={cfg.routes.integrationStatusResources}
        render={() => <Agents />}
      />
    </MemoryRouter>
  );

  await waitFor(() => {
    screen.getAllByText('dev-db');
  });

  expect(getTableCellContents()).toEqual({
    header: ['Service Name', 'Labels'],
    rows: [
      ['dev-db', 'region:us-west-2'],
      ['dev-db', 'region:us-west-1*:*'],
      ['staging-db', '*:*'],
    ],
  });

  await userEvent.click(screen.getAllByRole('row')[1]);
  expect(window.open).toHaveBeenCalledWith('some-aws-url', '_blank');

  jest.clearAllMocks();
});

function getTableCellContents() {
  const [header, ...rows] = screen.getAllByRole('row');
  return {
    header: within(header)
      .getAllByRole('columnheader')
      .map(cell => cell.textContent),
    rows: rows.map(row =>
      within(row)
        .getAllByRole('cell')
        .map(cell => cell.textContent)
    ),
  };
}
