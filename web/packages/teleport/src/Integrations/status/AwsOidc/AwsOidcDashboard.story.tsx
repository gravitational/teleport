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

import {
  makeErrorAttempt,
  makeProcessingAttempt,
  makeSuccessAttempt,
} from 'shared/hooks/useAsync';

import cfg from 'teleport/config';
import { AwsOidcDashboard } from 'teleport/Integrations/status/AwsOidc/AwsOidcDashboard';
import { MockAwsOidcStatusProvider } from 'teleport/Integrations/status/AwsOidc/testHelpers/mockAwsOidcStatusProvider';
import { IntegrationKind } from 'teleport/services/integrations';

import { makeAwsOidcStatusContextState } from './testHelpers/makeAwsOidcStatusContextState';

export default {
  title: 'Teleport/Integrations/AwsOidc',
};

const setup = {
  path: cfg.routes.integrationStatus,
  initialEntries: [
    cfg.getIntegrationStatusRoute(IntegrationKind.AwsOidc, 'oidc-int'),
  ],
};

// Loaded dashboard with data for each aws resource and a navigation header
export function Dashboard() {
  return (
    <MockAwsOidcStatusProvider
      {...setup}
      value={makeAwsOidcStatusContextState()}
    >
      <AwsOidcDashboard />
    </MockAwsOidcStatusProvider>
  );
}

// Loaded dashboard with missing data for each aws resource and a navigation header
export function DashboardMissingData() {
  const state = makeAwsOidcStatusContextState();
  state.statsAttempt.data = undefined;
  return (
    <MockAwsOidcStatusProvider {...setup} value={state}>
      <AwsOidcDashboard />
    </MockAwsOidcStatusProvider>
  );
}

// Loaded dashboard with missing data for each aws resource and a navigation header
export function DashboardMissingSummary() {
  const state = makeAwsOidcStatusContextState();
  state.statsAttempt.data.awseks = undefined;
  state.statsAttempt.data.awsrds = undefined;
  state.statsAttempt.data.awsec2 = undefined;
  return (
    <MockAwsOidcStatusProvider {...setup} value={state}>
      <AwsOidcDashboard />
    </MockAwsOidcStatusProvider>
  );
}

// Loading screen
export function StatsProcessing() {
  const props = makeAwsOidcStatusContextState({
    statsAttempt: makeProcessingAttempt(),
  });
  return (
    <MockAwsOidcStatusProvider {...setup} value={props}>
      <AwsOidcDashboard />
    </MockAwsOidcStatusProvider>
  );
}

// No header, no loading indicator
export function IntegrationProcessing() {
  const props = makeAwsOidcStatusContextState({
    integrationAttempt: makeProcessingAttempt(),
  });
  return (
    <MockAwsOidcStatusProvider {...setup} value={props}>
      <AwsOidcDashboard />
    </MockAwsOidcStatusProvider>
  );
}

// Loaded error message
export function StatsFailed() {
  const props = makeAwsOidcStatusContextState({
    statsAttempt: makeErrorAttempt(new Error('failed  to get stats')),
  });
  return (
    <MockAwsOidcStatusProvider {...setup} value={props}>
      <AwsOidcDashboard />
    </MockAwsOidcStatusProvider>
  );
}

// Loaded error message
export function IntegrationFailed() {
  const props = makeAwsOidcStatusContextState({
    integrationAttempt: makeErrorAttempt(
      new Error('failed  to get integration')
    ),
  });
  return (
    <MockAwsOidcStatusProvider {...setup} value={props}>
      <AwsOidcDashboard />
    </MockAwsOidcStatusProvider>
  );
}

// Loaded error message
export function BothFailed() {
  const props = makeAwsOidcStatusContextState({
    statsAttempt: makeErrorAttempt(new Error('failed  to get stats')),
    integrationAttempt: makeErrorAttempt(
      new Error('failed  to get integration')
    ),
  });
  return (
    <MockAwsOidcStatusProvider {...setup} value={props}>
      <AwsOidcDashboard />
    </MockAwsOidcStatusProvider>
  );
}

// Blank screen
export function StatsNoData() {
  const props = makeAwsOidcStatusContextState({
    statsAttempt: makeSuccessAttempt(null),
  });
  return (
    <MockAwsOidcStatusProvider {...setup} value={props}>
      <AwsOidcDashboard />
    </MockAwsOidcStatusProvider>
  );
}

// No header, no loading indicator
export function IntegrationNoData() {
  const props = makeAwsOidcStatusContextState({
    integrationAttempt: makeSuccessAttempt(null),
  });
  return (
    <MockAwsOidcStatusProvider {...setup} value={props}>
      <AwsOidcDashboard />
    </MockAwsOidcStatusProvider>
  );
}
