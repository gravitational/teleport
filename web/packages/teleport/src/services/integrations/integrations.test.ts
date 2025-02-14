/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import cfg from 'teleport/config';
import { AwsResource } from 'teleport/Integrations/status/AwsOidc/StatCard';
import api from 'teleport/services/api';

import { integrationService } from './integrations';
import {
  IntegrationAudience,
  IntegrationAwsOidc,
  IntegrationStatusCode,
} from './types';

beforeEach(() => {
  jest.resetAllMocks();
});

test('fetch a single integration: fetchIntegration()', async () => {
  // test a valid response
  jest.spyOn(api, 'get').mockResolvedValue(awsOidcIntegration);

  let response =
    await integrationService.fetchIntegration<IntegrationAwsOidc>(
      'integration-name'
    );
  expect(api.get).toHaveBeenCalledWith(
    cfg.getIntegrationsUrl('integration-name')
  );
  expect(response).toEqual({
    kind: 'aws-oidc',
    name: 'aws-oidc-integration',
    resourceType: 'integration',
    details:
      'Enroll EC2, RDS and EKS resources or enable Web/CLI access to your AWS Account.',
    spec: {
      roleArn: 'arn-123',
      origin: undefined,
    },
    statusCode: IntegrationStatusCode.Running,
  });

  // test null response
  jest.spyOn(api, 'get').mockResolvedValue(null);

  response =
    await integrationService.fetchIntegration<IntegrationAwsOidc>(
      'integration-name'
    );
  expect(response).toEqual({
    resourceType: 'integration',
    statusCode: IntegrationStatusCode.Running,
    kind: undefined,
    name: undefined,
  });
});

test('fetch integration list: fetchIntegrations()', async () => {
  // test a valid response
  jest.spyOn(api, 'get').mockResolvedValue({
    items: [
      awsOidcIntegration,
      awsOidcIntegrationWithAudience,
      githubIntegration,
      nonAwsOidcIntegration,
    ],
    nextKey: 'some-key',
  });

  let response = await integrationService.fetchIntegrations();
  expect(api.get).toHaveBeenCalledWith(cfg.getIntegrationsUrl());
  expect(response).toEqual({
    nextKey: 'some-key',
    items: [
      {
        kind: 'aws-oidc',
        name: 'aws-oidc-integration',
        resourceType: 'integration',
        details:
          'Enroll EC2, RDS and EKS resources or enable Web/CLI access to your AWS Account.',
        spec: {
          roleArn: 'arn-123',
        },
        statusCode: IntegrationStatusCode.Running,
      },
      {
        kind: 'aws-oidc',
        name: 'aws-oidc-integration2',
        resourceType: 'integration',
        details:
          'Enroll EC2, RDS and EKS resources or enable Web/CLI access to your AWS Account.',
        spec: {
          roleArn: 'arn-12345',
          audience: 'aws-identity-center',
        },
        statusCode: IntegrationStatusCode.Running,
      },
      {
        kind: 'github',
        name: 'github-my-org',
        resourceType: 'integration',
        details: 'GitHub repository access for organization "my-org"',
        spec: { organization: 'my-org' },
        statusCode: IntegrationStatusCode.Running,
      },
      {
        kind: 'abc',
        name: 'non-aws-oidc-integration',
        resourceType: 'integration',
        statusCode: IntegrationStatusCode.Running,
      },
    ],
  });

  // test null response
  jest.spyOn(api, 'get').mockResolvedValue(null);

  response = await integrationService.fetchIntegrations();
  expect(response).toEqual({
    items: [],
    nextKey: undefined,
  });
});

test('fetchAwsDatabases response', async () => {
  // test a valid response
  jest
    .spyOn(api, 'post')
    .mockResolvedValue({ databases: mockAwsDbs, nextToken: 'next-token' });

  let response = await integrationService.fetchAwsRdsDatabases(
    'integration-name',
    'mysql',
    { region: 'us-east-1', nextToken: 'next-token' }
  );

  expect(response).toEqual({
    databases: [
      {
        engine: 'postgres',
        name: 'rds-1',
        uri: 'endpoint-1',
        status: 'Available',
        labels: [{ name: 'env', value: 'prod' }],
        accountId: 'account-id-1',
        resourceId: 'resource-id-1',
        vpcId: 'vpc-123',
        subnets: [],
        securityGroups: [],
      },
      {
        engine: 'mysql',
        name: 'rds-2',
        uri: 'endpoint-2',
        labels: [],
        status: undefined,
        accountId: undefined,
        resourceId: undefined,
        vpcId: undefined,
        subnets: [],
        securityGroups: [],
      },
      {
        engine: 'mysql',
        name: 'rds-3',
        uri: 'endpoint-3',
        labels: [],
        status: undefined,
        accountId: undefined,
        resourceId: undefined,
        vpcId: undefined,
        subnets: [],
        securityGroups: [],
      },
    ],
    nextToken: 'next-token',
  });

  // test null response
  jest.spyOn(api, 'post').mockResolvedValue(null);

  response = await integrationService.fetchAwsRdsDatabases(
    'integration-name',
    'mysql',
    {} as any
  );
  expect(response).toEqual({
    databases: [],
    nextToken: undefined,
  });
});

describe('fetchAwsDatabases() request body formatting', () => {
  test.each`
    protocol             | expectedEngines          | expectedRdsType
    ${'mysql'}           | ${['mysql', 'mariadb']}  | ${'instance'}
    ${'postgres'}        | ${['postgres']}          | ${'instance'}
    ${'aurora-mysql'}    | ${['aurora-mysql']}      | ${'cluster'}
    ${'aurora-postgres'} | ${['aurora-postgresql']} | ${'cluster'}
  `(
    'format protocol $protocol',
    async ({ protocol, expectedEngines, expectedRdsType }) => {
      jest.spyOn(api, 'post').mockResolvedValue({ databases: [] }); // not testing response here.

      await integrationService.fetchAwsRdsDatabases(protocol, protocol, {
        region: 'us-east-1',
        nextToken: 'next-token',
      });

      expect(api.post).toHaveBeenCalledWith(
        `/v1/webapi/sites/localhost/integrations/aws-oidc/${protocol}/databases`,
        {
          rdsType: expectedRdsType,
          engines: expectedEngines,
          region: 'us-east-1',
          nextToken: 'next-token',
        }
      );
    }
  );
});

test('fetch integration rules: fetchIntegrationRules()', async () => {
  // test a valid response
  jest.spyOn(api, 'get').mockResolvedValue({
    rules: [
      {
        resourceType: 'eks',
        region: 'us-west-2',
        labelMatcher: [{ name: 'env', value: 'dev' }],
        discoveryConfig: 'cfg',
        lastSync: 1733782634,
      },
    ],
    nextKey: 'some-key',
  });

  let response = await integrationService.fetchIntegrationRules(
    'name',
    AwsResource.eks
  );
  expect(api.get).toHaveBeenCalledWith(
    cfg.getIntegrationRulesUrl('name', AwsResource.eks)
  );
  expect(response).toEqual({
    nextKey: 'some-key',
    rules: [
      {
        resourceType: 'eks',
        region: 'us-west-2',
        labelMatcher: [{ name: 'env', value: 'dev' }],
        discoveryConfig: 'cfg',
        lastSync: 1733782634,
      },
    ],
  });

  // test null response
  jest.spyOn(api, 'get').mockResolvedValue(null);

  response = await integrationService.fetchIntegrationRules(
    'name',
    AwsResource.eks
  );
  expect(response).toEqual({
    nextKey: undefined,
    rules: [],
  });
});

const nonAwsOidcIntegration = {
  name: 'non-aws-oidc-integration',
  subKind: 'abc',
};

const awsOidcIntegration = {
  name: 'aws-oidc-integration',
  subKind: 'aws-oidc',
  awsoidc: { roleArn: 'arn-123' },
};

const awsOidcIntegrationWithAudience = {
  name: 'aws-oidc-integration2',
  subKind: 'aws-oidc',
  awsoidc: {
    roleArn: 'arn-12345',
    audience: IntegrationAudience.AwsIdentityCenter,
  },
};
const githubIntegration = {
  name: 'github-my-org',
  subKind: 'github',
  github: {
    organization: 'my-org',
  },
};

const mockAwsDbs = [
  {
    protocol: 'postgres',
    name: 'rds-1',
    uri: 'endpoint-1',
    labels: [{ name: 'env', value: 'prod' }],
    aws: {
      status: 'Available',
      account_id: 'account-id-1',
      rds: {
        resource_id: 'resource-id-1',
        vpc_id: 'vpc-123',
      },
    },
  },
  // Test with empty aws fields.
  {
    protocol: 'mysql',
    name: 'rds-2',
    uri: 'endpoint-2',
    aws: {},
  },
  // Test without aws field.
  {
    protocol: 'mysql',
    name: 'rds-3',
    uri: 'endpoint-3',
  },
];
