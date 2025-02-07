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
import { addHours } from 'date-fns';

import { render, screen } from 'design/utils/testing';
import { makeSuccessAttempt } from 'shared/hooks/useAsync';

import { AwsOidcDashboard } from 'teleport/Integrations/status/AwsOidc/AwsOidcDashboard';
import { MockAwsOidcStatusProvider } from 'teleport/Integrations/status/AwsOidc/testHelpers/mockAwsOidcStatusProvider';
import {
  IntegrationAwsOidc,
  IntegrationKind,
  IntegrationWithSummary,
} from 'teleport/services/integrations';

test('renders header and stats cards', () => {
  render(
    <MockAwsOidcStatusProvider
      value={{
        integrationAttempt: makeSuccessAttempt<IntegrationAwsOidc>({
          resourceType: 'integration',
          name: 'integration-one',
          kind: IntegrationKind.AwsOidc,
          spec: {
            roleArn: 'arn:aws:iam::111456789011:role/bar',
          },
          statusCode: 1,
        }),
        statsAttempt: makeSuccessAttempt<IntegrationWithSummary>({
          name: 'integration-one',
          subKind: IntegrationKind.AwsOidc,
          awsoidc: {
            roleArn: 'arn:aws:iam::111456789011:role/bar',
          },
          awsec2: {
            rulesCount: 24,
            resourcesFound: 12,
            resourcesEnrollmentFailed: 3,
            resourcesEnrollmentSuccess: 9,
            discoverLastSync: new Date().getTime(),
            ecsDatabaseServiceCount: 0, // irrelevant
          },
          awsrds: {
            rulesCount: 14,
            resourcesFound: 5,
            resourcesEnrollmentFailed: 5,
            resourcesEnrollmentSuccess: 0,
            discoverLastSync: addHours(new Date().getTime(), -4).getTime(),
            ecsDatabaseServiceCount: 8, // relevant
          },
          awseks: {
            rulesCount: 33,
            resourcesFound: 3,
            resourcesEnrollmentFailed: 0,
            resourcesEnrollmentSuccess: 3,
            discoverLastSync: addHours(new Date().getTime(), -48).getTime(),
            ecsDatabaseServiceCount: 0, // irrelevant
          },
        }),
      }}
      path=""
    >
      <AwsOidcDashboard />
    </MockAwsOidcStatusProvider>
  );

  const breadcrumbs = screen.getByTestId('aws-oidc-header');
  expect(within(breadcrumbs).getByText('integration-one')).toBeInTheDocument();

  const title = screen.getByTestId('aws-oidc-title');
  expect(within(title).getByRole('link', { name: 'back' })).toHaveAttribute(
    'href',
    '/web/integrations'
  );
  expect(within(title).getByLabelText('status')).toHaveAttribute(
    'kind',
    'success'
  );
  expect(within(title).getByLabelText('status')).toHaveTextContent('Running');
  expect(within(title).getByText('integration-one')).toBeInTheDocument();

  const ec2 = screen.getByTestId('ec2-stats');
  expect(within(ec2).getByTestId('sync')).toHaveTextContent(
    'Last Sync: 0 seconds ago'
  );
  expect(within(ec2).getByTestId('rules')).toHaveTextContent(
    'Enrollment Rules 24'
  );
  expect(within(ec2).queryByTestId('rds-agents')).not.toBeInTheDocument();
  expect(within(ec2).getByTestId('enrolled')).toHaveTextContent(
    'Enrolled Instances 9'
  );
  expect(within(ec2).getByTestId('failed')).toHaveTextContent(
    'Failed Instances 3'
  );

  const rds = screen.getByTestId('rds-stats');
  expect(within(rds).getByTestId('sync')).toHaveTextContent(
    'Last Sync: 4 hours ago'
  );
  expect(within(rds).getByTestId('rules')).toHaveTextContent(
    'Enrollment Rules 14'
  );
  expect(within(rds).getByTestId('rds-agents')).toHaveTextContent('Agents 8');
  expect(within(rds).getByTestId('enrolled')).toHaveTextContent(
    'Enrolled Databases 0'
  );
  expect(within(rds).getByTestId('failed')).toHaveTextContent(
    'Failed Databases 5'
  );

  const eks = screen.getByTestId('eks-stats');
  expect(within(eks).getByTestId('sync')).toHaveTextContent(
    'Last Sync: 2 days ago'
  );
  expect(within(eks).getByTestId('rules')).toHaveTextContent(
    'Enrollment Rules 33'
  );
  expect(within(eks).queryByTestId('rds-agents')).not.toBeInTheDocument();
  expect(within(eks).getByTestId('enrolled')).toHaveTextContent(
    'Enrolled Clusters 3'
  );
  expect(within(eks).getByTestId('failed')).toHaveTextContent(
    'Failed Clusters 0'
  );
});

test('renders enroll cards', () => {
  const zeroCount = {
    rulesCount: 0,
    resourcesFound: 0,
    resourcesEnrollmentFailed: 0,
    resourcesEnrollmentSuccess: 0,
    discoverLastSync: new Date().getTime(),
    ecsDatabaseServiceCount: 0,
  };

  render(
    <MockAwsOidcStatusProvider
      value={{
        integrationAttempt: makeSuccessAttempt({
          resourceType: 'integration',
          name: 'integration-one',
          kind: IntegrationKind.AwsOidc,
          spec: {
            roleArn: 'arn:aws:iam::111456789011:role/bar',
          },
          statusCode: 1,
        }),
        statsAttempt: makeSuccessAttempt({
          name: 'integration-one',
          subKind: IntegrationKind.AwsOidc,
          awsoidc: {
            roleArn: 'arn:aws:iam::111456789011:role/bar',
          },
          awsec2: zeroCount,
          awsrds: zeroCount,
          awseks: zeroCount,
        }),
      }}
      path=""
    >
      <AwsOidcDashboard />
    </MockAwsOidcStatusProvider>
  );

  expect(
    within(screen.getByTestId('ec2-enroll')).getByRole('link', {
      name: 'Enroll EC2',
    })
  ).toBeInTheDocument();
  expect(
    within(screen.getByTestId('rds-enroll')).getByRole('link', {
      name: 'Enroll RDS',
    })
  ).toBeInTheDocument();
  expect(
    within(screen.getByTestId('eks-enroll')).getByRole('link', {
      name: 'Enroll EKS',
    })
  ).toBeInTheDocument();
});
