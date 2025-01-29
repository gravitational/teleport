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

import { addHours } from 'date-fns';

import { AwsOidcDashboard } from 'teleport/Integrations/status/AwsOidc/AwsOidcDashboard';
import { MockAwsOidcStatusProvider } from 'teleport/Integrations/status/AwsOidc/testHelpers/mockAwsOidcStatusProvider';
import { AwsOidcStatusContextState } from 'teleport/Integrations/status/AwsOidc/useAwsOidcStatus';
import {
  IntegrationKind,
  ResourceTypeSummary,
} from 'teleport/services/integrations';

export default {
  title: 'Teleport/Integrations/AwsOidc',
};

// Loaded dashboard with data for each aws resource and a navigation header
export function Dashboard() {
  return (
    <MockAwsOidcStatusProvider value={makeAwsOidcStatusContextState()}>
      <AwsOidcDashboard />
    </MockAwsOidcStatusProvider>
  );
}

// Loaded dashboard with missing data for each aws resource and a navigation header
export function DashboardMissingData() {
  const state = makeAwsOidcStatusContextState();
  state.statsAttempt.data.awseks = undefined;
  state.statsAttempt.data.awsrds = undefined;
  state.statsAttempt.data.awsec2 = undefined;
  return (
    <MockAwsOidcStatusProvider value={state}>
      <AwsOidcDashboard />
    </MockAwsOidcStatusProvider>
  );
}

// Loading screen
export function StatsProcessing() {
  const props = makeAwsOidcStatusContextState({
    statsAttempt: { status: 'processing', data: null, statusText: '' },
  });
  return (
    <MockAwsOidcStatusProvider value={props}>
      <AwsOidcDashboard />
    </MockAwsOidcStatusProvider>
  );
}

// No header, no loading indicator
export function IntegrationProcessing() {
  const props = makeAwsOidcStatusContextState({
    integrationAttempt: {
      status: 'processing',
      data: null,
      statusText: '',
    },
  });
  return (
    <MockAwsOidcStatusProvider value={props}>
      <AwsOidcDashboard />
    </MockAwsOidcStatusProvider>
  );
}

// Loaded error message
export function StatsFailed() {
  const props = makeAwsOidcStatusContextState({
    statsAttempt: {
      status: 'error',
      data: null,
      statusText: 'failed to get stats',
      error: {},
    },
  });
  return (
    <MockAwsOidcStatusProvider value={props}>
      <AwsOidcDashboard />
    </MockAwsOidcStatusProvider>
  );
}

// Loaded dashboard with data for each aws resource but no navigation header
export function IntegrationFailed() {
  const props = makeAwsOidcStatusContextState({
    integrationAttempt: {
      status: 'error',
      data: null,
      statusText: 'failed  to get integration',
      error: {},
    },
  });
  return (
    <MockAwsOidcStatusProvider value={props}>
      <AwsOidcDashboard />
    </MockAwsOidcStatusProvider>
  );
}

// Blank screen
export function StatsNoData() {
  const props = makeAwsOidcStatusContextState({
    statsAttempt: { status: 'success', data: null, statusText: '' },
  });
  return (
    <MockAwsOidcStatusProvider value={props}>
      <AwsOidcDashboard />
    </MockAwsOidcStatusProvider>
  );
}

// No header, no loading indicator
export function IntegrationNoData() {
  const props = makeAwsOidcStatusContextState({
    integrationAttempt: {
      status: 'success',
      data: null,
      statusText: '',
    },
  });
  return (
    <MockAwsOidcStatusProvider value={props}>
      <AwsOidcDashboard />
    </MockAwsOidcStatusProvider>
  );
}

function makeAwsOidcStatusContextState(
  overrides: Partial<AwsOidcStatusContextState> = {}
): AwsOidcStatusContextState {
  return Object.assign(
    {
      integrationAttempt: {
        status: 'success',
        statusText: '',
        data: {
          resourceType: 'integration',
          name: 'integration-one',
          kind: IntegrationKind.AwsOidc,
          spec: {
            roleArn: 'arn:aws:iam::111456789011:role/bar',
          },
          statusCode: 1,
        },
      },
      statsAttempt: {
        status: 'success',
        statusText: '',
        data: {
          name: 'integration-one',
          subKind: IntegrationKind.AwsOidc,
          awsoidc: {
            roleArn: 'arn:aws:iam::111456789011:role/bar',
          },
          awsec2: makeResourceTypeSummary(),
          awsrds: makeResourceTypeSummary(),
          awseks: makeResourceTypeSummary(),
        },
      },
    },
    overrides
  );
}

function makeResourceTypeSummary(
  overrides: Partial<ResourceTypeSummary> = {}
): ResourceTypeSummary {
  return Object.assign(
    {
      rulesCount: Math.floor(Math.random() * 100),
      resourcesFound: Math.floor(Math.random() * 100),
      resourcesEnrollmentFailed: Math.floor(Math.random() * 100),
      resourcesEnrollmentSuccess: Math.floor(Math.random() * 100),
      discoverLastSync: addHours(
        new Date().getTime(),
        -Math.floor(Math.random() * 100)
      ),
      ecsDatabaseServiceCount: Math.floor(Math.random() * 100),
    },
    overrides
  );
}
