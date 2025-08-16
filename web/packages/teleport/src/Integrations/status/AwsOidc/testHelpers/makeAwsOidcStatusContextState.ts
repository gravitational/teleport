/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { makeSuccessAttempt } from 'shared/hooks/useAsync';

import { AwsOidcStatusContextState } from 'teleport/Integrations/status/AwsOidc/useAwsOidcStatus';
import {
  IntegrationKind,
  ResourceTypeSummary,
} from 'teleport/services/integrations';

export function makeAwsOidcStatusContextState(
  overrides: Partial<AwsOidcStatusContextState> = {}
): AwsOidcStatusContextState {
  return Object.assign(
    {
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
        awsec2: makeResourceTypeSummary(),
        awsrds: makeResourceTypeSummary(),
        awseks: makeResourceTypeSummary(),
      }),
    },
    overrides
  );
}

function makeResourceTypeSummary(
  overrides: Partial<ResourceTypeSummary> = {}
): ResourceTypeSummary {
  return Object.assign(
    {
      rulesCount: 11,
      resourcesFound: 11,
      resourcesEnrollmentFailed: 11,
      resourcesEnrollmentSuccess: 11,
      discoverLastSync: new Date().getTime(),
      ecsDatabaseServiceCount: 11,
      unresolvedUserTasks: 11,
    },
    overrides
  );
}
