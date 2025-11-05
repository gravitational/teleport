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
import { http, HttpResponse } from 'msw';

import cfg from 'teleport/config';
import { AwsResource } from 'teleport/Integrations/status/AwsOidc/Cards/StatCard';
import { Details } from 'teleport/Integrations/status/AwsOidc/Details/Details';
import { MockAwsOidcStatusProvider } from 'teleport/Integrations/status/AwsOidc/testHelpers/mockAwsOidcStatusProvider';
import { IntegrationKind } from 'teleport/services/integrations';

import { makeAwsOidcStatusContextState } from '../testHelpers/makeAwsOidcStatusContextState';
import { makeIntegrationDiscoveryRule } from '../testHelpers/makeIntegrationDiscoveryRule';

export default {
  title: 'Teleport/Integrations/AwsOidc/Details',
};

const integrationName = 'integration-story';

// Empty ec2 details table
export function EC2Empty() {
  return (
    <MockAwsOidcStatusProvider
      value={makeAwsOidcStatusContextState()}
      initialEntries={[getPath(AwsResource.ec2)]}
      path={cfg.routes.integrationStatusResources}
    >
      <Details />
    </MockAwsOidcStatusProvider>
  );
}

// Populated ec2 details table
export function EC2() {
  return (
    <MockAwsOidcStatusProvider
      value={makeAwsOidcStatusContextState()}
      initialEntries={[getPath(AwsResource.ec2)]}
      path={cfg.routes.integrationStatusResources}
    >
      <Details />
    </MockAwsOidcStatusProvider>
  );
}

EC2.parameters = {
  msw: {
    handlers: [
      http.get(
        cfg.getIntegrationRulesUrl(integrationName, AwsResource.ec2),
        () => {
          return HttpResponse.json({
            rules: rules,
            nextKey: '1',
          });
        }
      ),
    ],
  },
};

// Empty eks details table
export function EKSEmpty() {
  return (
    <MockAwsOidcStatusProvider
      value={makeAwsOidcStatusContextState()}
      initialEntries={[getPath(AwsResource.eks)]}
      path={cfg.routes.integrationStatusResources}
    >
      <Details />
    </MockAwsOidcStatusProvider>
  );
}

// Populated eks details table
export function EKS() {
  return (
    <MockAwsOidcStatusProvider
      value={makeAwsOidcStatusContextState()}
      initialEntries={[getPath(AwsResource.eks)]}
      path={cfg.routes.integrationStatusResources}
    >
      <Details />
    </MockAwsOidcStatusProvider>
  );
}

EKS.parameters = {
  msw: {
    handlers: [
      http.get(
        cfg.getIntegrationRulesUrl(integrationName, AwsResource.eks),
        () => {
          return HttpResponse.json({
            rules: rules,
            nextKey: '1',
          });
        }
      ),
    ],
  },
};

// Empty rds details table
export function RDSEmpty() {
  return (
    <MockAwsOidcStatusProvider
      value={makeAwsOidcStatusContextState()}
      initialEntries={[getPath(AwsResource.rds)]}
      path={cfg.routes.integrationStatusResources}
    >
      <Details />
    </MockAwsOidcStatusProvider>
  );
}

// Populated eks details table
export function RDS() {
  return (
    <MockAwsOidcStatusProvider
      value={makeAwsOidcStatusContextState()}
      initialEntries={[getPath(AwsResource.rds)]}
      path={cfg.routes.integrationStatusResources}
    >
      <Details />
    </MockAwsOidcStatusProvider>
  );
}

RDS.parameters = {
  msw: {
    handlers: [
      http.get(
        cfg.getIntegrationRulesUrl(integrationName, AwsResource.rds),
        () => {
          return HttpResponse.json({
            rules: rules,
            nextKey: '1',
          });
        }
      ),
      http.post(
        cfg.getAwsOidcDatabaseServices(integrationName, AwsResource.rds, []),
        () => {
          return HttpResponse.json({
            services: [
              {
                name: 'dev-db',
                matchingLabels: [{ name: 'region', value: 'us-west-2' }],
              },
              {
                name: 'dev-db',
                matchingLabels: [
                  { name: 'region', value: 'us-west-1' },
                  { name: '*', value: '*' },
                ],
              },
              {
                name: 'staging-db',
                matchingLabels: [{ name: '*', value: '*' }],
              },
            ],
          });
        }
      ),
    ],
  },
};

function getPath(resource: AwsResource) {
  return cfg.getIntegrationStatusResourcesRoute(
    IntegrationKind.AwsOidc,
    integrationName,
    resource
  );
}

const rules = [
  makeIntegrationDiscoveryRule({
    region: 'us-west-2',
    labelMatcher: [
      { name: 'env', value: 'prod' },
      { name: 'key', value: '123' },
    ],
  }),
  makeIntegrationDiscoveryRule({
    region: 'us-west-2',
    labelMatcher: [
      { name: 'env', value: 'prod' },
      { name: 'key', value: '123' },
    ],
  }),
  makeIntegrationDiscoveryRule({
    region: 'us-west-2',
    labelMatcher: [
      { name: 'env', value: 'prod' },
      { name: 'key', value: '123' },
    ],
  }),
];
