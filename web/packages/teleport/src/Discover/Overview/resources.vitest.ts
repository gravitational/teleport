/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { expect, test } from 'vitest';

import { AwsResource } from 'teleport/Integrations/status/AwsOidc/Cards/StatCard';
import type { UnifiedResourceKind } from 'teleport/services/agents';
import {
  AzureResource,
  type IntegrationDiscoveryRule,
} from 'teleport/services/integrations';

import { getListResourceRoute } from './resources';

const integrationName = 'aws-integration';
const accountId = '123456789012';

const cases: {
  resource: AwsResource;
  resourceKind: UnifiedResourceKind;
  rules: IntegrationDiscoveryRule[];
  expected: string;
}[] = [
  {
    resource: AwsResource.ec2,
    resourceKind: 'node',
    rules: [makeRule('ec2', 'us-east-1')],
    expected:
      'exists(labels["teleport.dev/instance-id"])' +
      ` && labels["teleport.dev/account-id"] == "${accountId}"`,
  },
  {
    resource: AwsResource.eks,
    resourceKind: 'kube_cluster',
    rules: [makeRule('eks', 'us-east-1'), makeRule('eks', 'us-west-2')],
    expected:
      'labels["teleport.dev/cloud"] == "AWS"' +
      ` && labels["account-id"] == "${accountId}"` +
      ' && (labels["region"] == "us-east-1" || labels["region"] == "us-west-2")',
  },
  {
    resource: AwsResource.rds,
    resourceKind: 'db',
    rules: [makeRule('rds', 'us-east-1')],
    expected: `labels["teleport.internal/discovery-integration-name"] == "${integrationName}"`,
  },
];
test.each(cases)(
  'filtering by $resource returns expected query and kind',
  ({ resource, resourceKind, rules, expected }) => {
    const route = getListResourceRoute({
      resource,
      integrationName,
      accountId,
      isOrganization: false,
      rules,
    });

    expect(getRouteQuery(route)).toEqual(expected);
    expect(getRouteKinds(route)).toEqual([resourceKind]);
  }
);

test('filtering EKS by wildcard region produces "exists" region filter', () => {
  const query = getRouteQuery(
    getListResourceRoute({
      resource: AwsResource.eks,
      integrationName,
      accountId,
      isOrganization: false,
      rules: [makeRule('eks', '*')],
    })
  );

  expect(query).toEqual(
    'labels["teleport.dev/cloud"] == "AWS"' +
      ` && labels["account-id"] == "${accountId}"` +
      ' && exists(labels["region"])'
  );
});

test('filtering EKS without rules omits the region filter', () => {
  const query = getRouteQuery(
    getListResourceRoute({
      resource: AwsResource.eks,
      integrationName,
      accountId,
      isOrganization: false,
      rules: [],
    })
  );

  expect(query).toBe(
    `labels["teleport.dev/cloud"] == "AWS" && labels["account-id"] == "${accountId}"`
  );
});

test.each([
  {
    resource: AwsResource.ec2,
    expected:
      'exists(labels["teleport.dev/instance-id"]) && exists(labels["teleport.dev/account-id"])',
  },
  {
    resource: AwsResource.eks,
    expected:
      'labels["teleport.dev/cloud"] == "AWS" && exists(labels["account-id"])',
  },
])(
  'filtering $resource by AWS organization does not scope to the account or region',
  ({ resource, expected }) => {
    const query = getRouteQuery(
      getListResourceRoute({
        resource,
        integrationName,
        accountId,
        isOrganization: true,
        rules: [makeRule(resource, 'us-east-1')],
      })
    );

    expect(query).toEqual(expected);
    expect(query).not.toContain(accountId);
  }
);

test('filtering by Azure with all values defined (no wildcard)', () => {
  const route = getListResourceRoute({
    resource: AzureResource.vm,
    rules: [
      {
        ...makeRule('azure'),
        region: 'eastus',
        subscriptions: ['subscription-1'],
        resourceGroups: ['production'],
      },
      {
        ...makeRule('azure'),
        region: 'westus2',
        subscriptions: ['subscription-1'],
        resourceGroups: ['production'],
      },
    ],
  });
  const query = getRouteQuery(route);

  expect(query).toContain(
    'labels["teleport.internal/subscription-id"] == "subscription-1"'
  );
  expect(query).toContain(
    '(labels["teleport.internal/region"] == "eastus" || labels["teleport.internal/region"] == "westus2")'
  );
  expect(query).toContain(
    'labels["teleport.internal/resource-group"] == "production"'
  );
  expect(query).toContain('exists(labels["teleport.internal/vm-id"])');

  expect(query).not.toContain('teleport.dev/');
  expect(getRouteKinds(route)).toEqual(['node']);
});

test('filtering by wildcard Azure discovery fields produces "exists" filter', () => {
  const query = getRouteQuery(
    getListResourceRoute({
      resource: AzureResource.vm,
      rules: [
        {
          ...makeRule('azure'),
          region: '*',
          subscriptions: ['*'],
          resourceGroups: ['*'],
        },
      ],
    })
  );

  expect(query).toContain(
    'exists(labels["teleport.internal/subscription-id"])'
  );
  expect(query).toContain('exists(labels["teleport.internal/region"])');
  expect(query).toContain('exists(labels["teleport.internal/resource-group"])');
  expect(query).not.toContain('teleport.dev/');
});

function getRouteQuery(route: string): string {
  return new URLSearchParams(route.split('?')[1]).get('query') ?? '';
}

function getRouteKinds(route: string): string[] {
  return new URLSearchParams(route.split('?')[1]).getAll('kinds');
}

function makeRule(
  resourceType: string,
  region = 'us-east-1'
): IntegrationDiscoveryRule {
  return {
    resourceType,
    region,
    labelMatcher: [],
    discoveryConfig: 'discovery-config',
    lastSync: 0,
  };
}
