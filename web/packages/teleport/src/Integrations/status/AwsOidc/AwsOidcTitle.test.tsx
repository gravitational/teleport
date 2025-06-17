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
import { within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';

import { render, screen } from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { AwsOidcTitle } from 'teleport/Integrations/status/AwsOidc/AwsOidcTitle';
import { AwsResource } from 'teleport/Integrations/status/AwsOidc/StatCard';
import {
  IntegrationAwsOidc,
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

const testIntegration: IntegrationAwsOidc = {
  kind: IntegrationKind.AwsOidc,
  name: 'some-name',
  resourceType: 'integration',
  spec: {
    roleArn: '',
    issuerS3Bucket: '',
    issuerS3Prefix: '',
  },
  statusCode: IntegrationStatusCode.Running,
};

test('renders with resource', () => {
  render(
    <MemoryRouter>
      <InfoGuidePanelProvider>
        <AwsOidcTitle
          integration={testIntegration}
          resource={AwsResource.ec2}
        />
      </InfoGuidePanelProvider>
    </MemoryRouter>
  );

  expect(screen.getByRole('link', { name: 'back' })).toHaveAttribute(
    'href',
    '/web/integrations/status/aws-oidc/some-name'
  );
  expect(screen.getByText('EC2')).toBeInTheDocument();
  expect(screen.queryByText('some-name')).not.toBeInTheDocument();
  expect(
    within(screen.getByLabelText('status')).getByText('Running')
  ).toBeInTheDocument();
});

test('renders without resource', () => {
  render(
    <MemoryRouter>
      <InfoGuidePanelProvider>
        <AwsOidcTitle integration={testIntegration} />
      </InfoGuidePanelProvider>
    </MemoryRouter>
  );

  expect(screen.getByRole('link', { name: 'back' })).toHaveAttribute(
    'href',
    '/web/integrations'
  );
  expect(screen.getByText('some-name')).toBeInTheDocument();
  expect(
    within(screen.getByLabelText('status')).getByText('Running')
  ).toBeInTheDocument();
});

test('renders tasks', () => {
  render(
    <MemoryRouter>
      <InfoGuidePanelProvider>
        <AwsOidcTitle integration={testIntegration} tasks={true} />
      </InfoGuidePanelProvider>
    </MemoryRouter>
  );

  expect(screen.getByRole('link', { name: 'back' })).toHaveAttribute(
    'href',
    '/web/integrations/status/aws-oidc/some-name'
  );
  expect(screen.getByText('Pending Tasks')).toBeInTheDocument();
  expect(
    within(screen.getByLabelText('status')).getByText('Running')
  ).toBeInTheDocument();
});
