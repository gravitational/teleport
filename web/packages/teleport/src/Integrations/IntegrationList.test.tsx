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

import { render, screen, userEvent } from 'design/utils/testing';

import {
  IntegrationKind,
  integrationService,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

import { IntegrationList } from './IntegrationList';

test('aws oidc row without s3 fields should render tooltip', async () => {
  jest
    .spyOn(integrationService, 'fetchThumbprint')
    .mockResolvedValue('some-thumbprint');

  render(
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
  );

  expect(screen.getByText('aws')).toBeInTheDocument();
  expect(screen.getByText(/running/i)).toBeInTheDocument();
  await userEvent.hover(screen.getByRole('icon'));
  expect(screen.queryByTestId('btn-copy')).not.toBeInTheDocument();

  await userEvent.click(screen.getByText(/generate a new thumbprint/i));
  expect(screen.getByText(/some-thumbprint/i)).toBeInTheDocument();
});

test('aws oidc row with s3 fields should NOT render tooltip', async () => {
  jest
    .spyOn(integrationService, 'fetchThumbprint')
    .mockResolvedValue('some-thumbprint');

  render(
    <IntegrationList
      list={[
        {
          resourceType: 'integration',
          name: 'aws',
          kind: IntegrationKind.AwsOidc,
          statusCode: IntegrationStatusCode.Running,
          spec: {
            roleArn: 'some-role-arn',
            issuerS3Prefix: 'some-prefix',
            issuerS3Bucket: 'some-bucket',
          },
        },
      ]}
    />
  );

  expect(screen.getByText('aws')).toBeInTheDocument();
  expect(screen.getByText(/running/i)).toBeInTheDocument();
  expect(screen.queryByRole('icon')).not.toBeInTheDocument();
});
