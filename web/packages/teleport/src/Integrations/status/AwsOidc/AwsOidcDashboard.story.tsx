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

import React from 'react';

import { AwsOidcDashboard } from 'teleport/Integrations/status/AwsOidc/AwsOidcDashboard';
import { MockAwsOidcStatusProvider } from 'teleport/Integrations/status/AwsOidc/testHelpers/mockAwsOidcStatusProvider';
import { IntegrationKind } from 'teleport/services/integrations';

export default {
  title: 'Teleport/Integrations/AwsOidc',
};

export function Dashboard() {
  return (
    <MockAwsOidcStatusProvider
      value={{
        attempt: {
          status: 'success',
          data: {
            resourceType: 'integration',
            name: 'integration-one',
            kind: IntegrationKind.AwsOidc,
            spec: {
              roleArn: 'arn:aws:iam::111456789011:role/bar',
            },
            statusCode: 1,
          },
          statusText: '',
        },
      }}
    >
      <AwsOidcDashboard />
    </MockAwsOidcStatusProvider>
  );
}
