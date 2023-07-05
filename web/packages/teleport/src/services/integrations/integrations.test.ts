/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import api from 'teleport/services/api';
import cfg from 'teleport/config';

import { integrationService } from './integrations';
import { IntegrationStatusCode } from './types';

test('fetchIntegration() response (a integration)', async () => {
  // test a valid response
  jest.spyOn(api, 'get').mockResolvedValue(awsOidcIntegration);

  let response = await integrationService.fetchIntegration('integration-name');
  expect(api.get).toHaveBeenCalledWith(
    cfg.getIntegrationsUrl('integration-name')
  );
  expect(response).toEqual({
    kind: 'aws-oidc',
    name: 'aws-oidc-integration',
    resourceType: 'integration',
    spec: {
      roleArn: 'arn-123',
    },
    statusCode: IntegrationStatusCode.Running,
  });

  // test null response
  jest.spyOn(api, 'get').mockResolvedValue(null);

  response = await integrationService.fetchIntegration('integration-name');
  expect(response).toEqual({
    resourceType: 'integration',
    statusCode: IntegrationStatusCode.Running,
    kind: undefined,
    name: undefined,
    spec: {
      roleArn: undefined,
    },
  });
});

test('fetchIntegrations() response (list)', async () => {
  // test a valid response
  jest.spyOn(api, 'get').mockResolvedValue({
    items: [awsOidcIntegration, nonAwsOidcIntegration],
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
        spec: {
          roleArn: 'arn-123',
        },
        statusCode: IntegrationStatusCode.Running,
      },
      {
        kind: 'abc',
        name: 'non-aws-oidc-integration',
        resourceType: 'integration',
        spec: {
          roleArn: undefined,
        },
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
      },
      {
        engine: 'mysql',
        name: 'rds-2',
        uri: 'endpoint-2',
        labels: [],
        status: undefined,
        accountId: undefined,
        resourceId: undefined,
      },
      {
        engine: 'mysql',
        name: 'rds-3',
        uri: 'endpoint-3',
        labels: [],
        status: undefined,
        accountId: undefined,
        resourceId: undefined,
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
    protocol             | expectedEngines               | expectedRdsType
    ${'mysql'}           | ${['mysql', 'mariadb']}       | ${'instance'}
    ${'postgres'}        | ${['postgres']}               | ${'instance'}
    ${'aurora-mysql'}    | ${['aurora', 'aurora-mysql']} | ${'cluster'}
    ${'aurora-postgres'} | ${['aurora-postgresql']}      | ${'cluster'}
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

const nonAwsOidcIntegration = {
  name: 'non-aws-oidc-integration',
  subKind: 'abc',
};
const awsOidcIntegration = {
  name: 'aws-oidc-integration',
  subKind: 'aws-oidc',
  awsoidc: { roleArn: 'arn-123' },
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
