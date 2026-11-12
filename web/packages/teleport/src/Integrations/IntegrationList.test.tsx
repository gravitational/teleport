/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { MemoryRouter, Route, Routes } from 'react-router';

import { CurrentPath, render, screen, userEvent } from 'design/utils/testing';

import {
  IntegrationList,
  type IntegrationLike,
} from 'teleport/Integrations/IntegrationList';
import {
  IntegrationKind,
  IntegrationStatusCode,
  type Plugin,
} from 'teleport/services/integrations';

function renderList(row: IntegrationLike) {
  render(
    <MemoryRouter initialEntries={['/integrations']}>
      <Routes>
        <Route
          path="/integrations"
          element={<IntegrationList list={[row]} />}
        />
        <Route path="*" element={<CurrentPath />} />
      </Routes>
    </MemoryRouter>
  );
}

test('aws-oidc row opens its status dashboard without an action menu', async () => {
  renderList({
    resourceType: 'integration',
    name: 'aws-integration',
    kind: IntegrationKind.AwsOidc,
    statusCode: IntegrationStatusCode.Running,
  });

  expect(
    screen.queryByRole('button', { name: 'Options' })
  ).not.toBeInTheDocument();
  await userEvent.click(screen.getAllByRole('row')[1]);
  expect(screen.getByTestId('current-path')).toHaveTextContent(
    '/web/integrations/status/aws-oidc/aws-integration'
  );
});

test('terraform-managed azure-oidc row opens its overview without an action menu', async () => {
  const row = {
    resourceType: 'integration',
    name: 'azure-integration',
    kind: IntegrationKind.AzureOidc,
    statusCode: IntegrationStatusCode.Running,
    isManagedByTerraform: true,
  } as const;
  renderList(row);

  expect(
    screen.queryByRole('button', { name: 'Options' })
  ).not.toBeInTheDocument();
  await userEvent.click(screen.getAllByRole('row')[1]);
  expect(screen.getByTestId('current-path')).toHaveTextContent(
    '/web/integrations/overview/azure-oidc/azure-integration'
  );
});

test('azure-oidc row without terraform keeps its action menu and does not navigate', async () => {
  renderList({
    resourceType: 'integration',
    name: 'azure-integration',
    kind: IntegrationKind.AzureOidc,
    statusCode: IntegrationStatusCode.Running,
  });

  expect(screen.getByRole('button', { name: 'Options' })).toBeInTheDocument();
  await userEvent.click(screen.getAllByRole('row')[1]);
  expect(screen.queryByTestId('current-path')).not.toBeInTheDocument();
});

test('integration list details prefer AWS IC status error message over details', () => {
  const plugin: Plugin = {
    resourceType: 'plugin',
    name: 'aws-ic-plugin',
    kind: 'aws-identity-center',
    details: 'fallback details',
    statusCode: IntegrationStatusCode.Unauthorized,
    status: {
      code: IntegrationStatusCode.Unauthorized,
      lastRun: new Date('2026-03-31T00:00:00Z'),
      errorMessage: 'scim token rejected',
    },
  };

  render(
    <MemoryRouter>
      <IntegrationList list={[plugin]} />
    </MemoryRouter>
  );

  expect(screen.getByText('scim token rejected')).toBeInTheDocument();
  expect(screen.queryByText('fallback details')).not.toBeInTheDocument();
});

test('integration list details ignore status error message for non-AWS-IC plugins', () => {
  const plugin: Plugin = {
    resourceType: 'plugin',
    name: 'okta-plugin',
    kind: 'okta',
    details: 'fallback details',
    statusCode: IntegrationStatusCode.OtherError,
    status: {
      code: IntegrationStatusCode.OtherError,
      lastRun: new Date('2026-03-31T00:00:00Z'),
      errorMessage: 'sync failed',
    },
  };

  render(
    <MemoryRouter>
      <IntegrationList list={[plugin]} />
    </MemoryRouter>
  );

  expect(screen.getByText('fallback details')).toBeInTheDocument();
  expect(screen.queryByText('sync failed')).not.toBeInTheDocument();
});
