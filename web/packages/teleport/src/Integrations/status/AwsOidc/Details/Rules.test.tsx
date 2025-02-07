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

import { render, screen, waitFor } from 'design/utils/testing';

import { Route } from 'teleport/components/Router';
import cfg from 'teleport/config';
import { Rules } from 'teleport/Integrations/status/AwsOidc/Details/Rules';
import { integrationService } from 'teleport/services/integrations';

import { makeIntegrationDiscoveryRule } from '../testHelpers/makeIntegrationDiscoveryRule';

test('renders region & labels from response', async () => {
  jest.spyOn(integrationService, 'fetchIntegrationRules').mockResolvedValue({
    rules: [
      makeIntegrationDiscoveryRule({
        region: 'us-west-2',
        labelMatcher: [
          { name: 'env', value: 'prod' },
          { name: 'key', value: '123' },
        ],
      }),
      makeIntegrationDiscoveryRule({
        region: 'us-east-2',
        labelMatcher: [{ name: 'env', value: 'stage' }],
      }),
      makeIntegrationDiscoveryRule({
        region: 'us-west-1',
        labelMatcher: [{ name: 'env', value: 'test' }],
      }),
      makeIntegrationDiscoveryRule({
        region: 'us-east-1',
        labelMatcher: [{ name: 'env', value: 'dev' }],
      }),
    ],
    nextKey: '',
  });
  render(
    <MemoryRouter
      initialEntries={[
        `/web/integrations/status/aws-oidc/some-name/resources/eks`,
      ]}
    >
      <Route
        path={cfg.routes.integrationStatusResources}
        render={() => <Rules />}
      />
    </MemoryRouter>
  );

  await waitFor(() => {
    expect(screen.getByText('env:prod')).toBeInTheDocument();
  });

  expect(getTableCellContents()).toEqual({
    header: ['Region', 'Labels'],
    rows: [
      ['us-west-2', 'env:prodkey:123'],
      ['us-east-2', 'env:stage'],
      ['us-west-1', 'env:test'],
      ['us-east-1', 'env:dev'],
    ],
  });

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
