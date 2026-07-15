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
  IntegrationStatusCode,
  IntegrationWithSummary,
  type Plugin,
} from 'teleport/services/integrations';

import { getStatus, SummaryStatusLabel } from './StatusLabel';

afterEach(() => {
  jest.useRealTimers();
});

test('SummaryStatusLabel shows scanning when no sync timestamps exist', () => {
  const stats = makeSummary(0);

  render(<SummaryStatusLabel summary={stats} />);

  expect(screen.getByText('Scanning')).toBeInTheDocument();
  expect(screen.getByText('(scanning in progress)')).toBeInTheDocument();
});

test('SummaryStatusLabel shows scanning when sync timestamp is stale', () => {
  jest.useFakeTimers().setSystemTime(new Date('2026-02-11T12:00:00Z'));
  const stats = makeSummary(new Date('2026-02-11T11:55:00Z').getTime());

  render(<SummaryStatusLabel summary={stats} />);

  expect(screen.getByText('Scanning')).toBeInTheDocument();
  expect(screen.getByText('(scanning in progress)')).toBeInTheDocument();
});

test('SummaryStatusLabel shows healthy when sync timestamp is recent', () => {
  jest.useFakeTimers().setSystemTime(new Date('2026-02-11T12:00:00Z'));
  const stats = makeSummary(new Date('2026-02-11T11:55:01Z').getTime());

  render(<SummaryStatusLabel summary={stats} />);

  expect(screen.getByText('Healthy')).toBeInTheDocument();
  expect(screen.queryByText('(scanning in progress)')).not.toBeInTheDocument();
});

test('getStatus surfaces backend errorMessage for AWS IC Unauthorized', () => {
  const plugin = makePlugin('aws-identity-center', {
    code: IntegrationStatusCode.Unauthorized,
    lastRun: new Date('2026-03-31T00:00:00Z'),
    errorMessage:
      'AWS Identity Center rejected the SCIM token. Rotate the token to restore access.',
  });

  const { status, label, tooltip } = getStatus(plugin);

  expect(status).toBe('Failed');
  expect(label).toBe('Failed');
  expect(tooltip).toBe(
    'AWS Identity Center rejected the SCIM token. Rotate the token to restore access.'
  );
});

test('getStatus keeps generic tooltip for non-AWS-IC Unauthorized even when errorMessage is set', () => {
  const plugin = makePlugin('slack', {
    code: IntegrationStatusCode.Unauthorized,
    lastRun: new Date('2026-03-31T00:00:00Z'),
    errorMessage: 'some backend error',
  });

  const { tooltip } = getStatus(plugin);

  expect(tooltip).toBe(
    'Integration was denied access. This could be a result of revoked authorization on the 3rd party provider. Try removing and re-connecting the integration.'
  );
});

function makePlugin(kind: Plugin['kind'], status: Plugin['status']): Plugin {
  return {
    resourceType: 'plugin',
    name: `${kind}-plugin`,
    kind,
    details: 'plugin details',
    statusCode: status.code,
    status,
  };
}

function makeSummary(lastSyncMs: number): IntegrationWithSummary {
  return {
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
      discoverLastSync: lastSyncMs,
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
    azurevm: {
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
}
