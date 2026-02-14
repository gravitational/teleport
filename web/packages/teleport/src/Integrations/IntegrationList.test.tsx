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

import { render, screen, userEvent } from 'design/utils/testing';

import { IntegrationList } from 'teleport/Integrations/IntegrationList';
import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

const mockedNavigate = jest.fn();

jest.mock('react-router', () => ({
  ...jest.requireActual('react-router'),
  useNavigate: () => mockedNavigate,
}));

beforeEach(() => {
  mockedNavigate.mockReset();
});

test('integration list does not display action menu for aws-oidc, row click navigates', async () => {
  render(
    <MemoryRouter initialEntries={['/integrations']}>
      <Routes>
        <Route
          path="/integrations"
          element={
            <IntegrationList
              list={[
                {
                  resourceType: 'integration',
                  name: 'aws-integration',
                  kind: IntegrationKind.AwsOidc,
                  statusCode: IntegrationStatusCode.Running,
                  spec: { roleArn: '', issuerS3Prefix: '', issuerS3Bucket: '' },
                },
              ]}
            />
          }
        />
      </Routes>
    </MemoryRouter>
  );

  expect(
    screen.queryByRole('button', { name: 'Options' })
  ).not.toBeInTheDocument();
  await userEvent.click(screen.getAllByRole('row')[1]);
  expect(mockedNavigate).toHaveBeenCalledWith(
    '/web/integrations/status/aws-oidc/aws-integration'
  );
});
