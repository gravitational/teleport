/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { render, screen } from 'design/utils/testing';

import {
  IntegrationKind,
  IntegrationWithSummary,
} from 'teleport/services/integrations';

import { SummaryStatusLabel } from './StatusLabel';

test('SummaryStatusLabel shows scanning when no sync timestamps exist', () => {
  const stats: IntegrationWithSummary = {
    name: 'integration-name',
    subKind: IntegrationKind.AwsOidc,
    unresolvedUserTasks: 0,
    userTasks: [],
    awsra: undefined,
    awsoidc: {
      roleArn: 'arn:aws:iam::123456789012:role/example',
    },
    awsec2: {
      rulesCount: 0,
      resourcesFound: 0,
      resourcesEnrollmentFailed: 0,
      resourcesEnrollmentSuccess: 0,
      discoverLastSync: 0,
      ecsDatabaseServiceCount: 0,
      unresolvedUserTasks: 0,
    },
    awsrds: {
      rulesCount: 0,
      resourcesFound: 0,
      resourcesEnrollmentFailed: 0,
      resourcesEnrollmentSuccess: 0,
      discoverLastSync: 0,
      ecsDatabaseServiceCount: 0,
      unresolvedUserTasks: 0,
    },
    awseks: {
      rulesCount: 0,
      resourcesFound: 0,
      resourcesEnrollmentFailed: 0,
      resourcesEnrollmentSuccess: 0,
      discoverLastSync: 0,
      ecsDatabaseServiceCount: 0,
      unresolvedUserTasks: 0,
    },
    rolesAnywhereProfileSync: undefined,
  };

  render(<SummaryStatusLabel summary={stats} />);

  expect(screen.getByText('Scanning')).toBeInTheDocument();
  expect(screen.getByText('(scanning in progress)')).toBeInTheDocument();
});
