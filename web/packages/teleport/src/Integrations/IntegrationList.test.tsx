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

import React from 'react';

import { fireEvent, render, screen } from 'design/utils/testing';

import { MemoryRouter } from 'react-router';

import { IntegrationList } from 'teleport/Integrations/IntegrationList';
import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

test('integration list shows edit and view action menu for aws-oidc', () => {
  render(
    <MemoryRouter>
      <IntegrationList
        list={[
          {
            resourceType: 'integration',
            name: 'aws',
            kind: IntegrationKind.AwsOidc,
            statusCode: IntegrationStatusCode.Running,
            spec: { roleArn: '', issuerS3Prefix: '', issuerS3Bucket: '' },
          },
        ]}
      />
    </MemoryRouter>
  );

  fireEvent.click(screen.getByRole('button', { name: 'Options' }));
  expect(screen.getByText('View Status')).toBeInTheDocument();
  expect(screen.getByText('View Status')).toHaveAttribute(
    'href',
    '/web/integrations/status/aws-oidc/aws'
  );
  expect(screen.getByText('Edit...')).toBeInTheDocument();
  expect(screen.getByText('Delete...')).toBeInTheDocument();
});
