/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { DbMeta } from 'teleport/Discover/useDiscover';
import { IamPolicyStatus } from 'teleport/services/databases';
import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';
import { DiscoverGuideId } from 'teleport/services/userPreferences/discoverPreference';

import { DATABASES } from '../SelectResource/resources';

export const resourceSpecAwsRdsPostgres = DATABASES.find(
  d => d.id === DiscoverGuideId.DatabaseAwsRdsPostgres
);

export const resourceSpecAwsRdsAuroraMysql = DATABASES.find(
  d => d.id === DiscoverGuideId.DatabaseAwsRdsAuroraMysql
);

export const resourceSpecAwsRdsMySql = DATABASES.find(
  d => d.id === DiscoverGuideId.DatabaseAwsRdsMysqlMariaDb
);

export const resourceSpecSelfHostedPostgres = DATABASES.find(
  d => d.id === DiscoverGuideId.DatabasePostgres
);

export const resourceSpecSelfHostedMysql = DATABASES.find(
  d => d.id === DiscoverGuideId.DatabaseMysql
);

export function getSelectedAwsPostgresDbMeta(): DbMeta {
  return {
    resourceName: 'db-name',
    awsRegion: 'us-east-1',
    agentMatcherLabels: [],
    db: {
      aws: {
        iamPolicyStatus: IamPolicyStatus.Unspecified,
        rds: {
          region: 'us-east-1',
          vpcId: 'test-vpc',
          securityGroups: ['sg-1', 'sg-2'],
          resourceId: 'some-rds-resource-id',
          subnets: [],
        },
      },
      kind: 'db',
      name: 'some-db-name',
      description: 'some-description',
      type: 'rds',
      protocol: 'postgres',
      labels: [],
      hostname: 'some-db-hostname',
      users: ['staticUser1', 'staticUser2', '*'],
      names: ['staticName1', 'staticName2', '*'],
    },
    selectedAwsRdsDb: {
      region: 'us-east-1',
      engine: 'postgres',
      name: 'rds-1',
      uri: 'example.abc123.ca-central-1.rds.amazonaws.com:3306',
      resourceId: 'some-rds-resource-id',
      accountId: '1234',
      labels: [],
      subnets: [],
      vpcId: '',
      securityGroups: ['sg-1', 'sg-2'],
      status: 'available',
    },
    awsIntegration: {
      kind: IntegrationKind.AwsOidc,
      name: 'test-integration',
      resourceType: 'integration',
      spec: {
        roleArn: 'arn:aws:iam::123456789012:role/test-role-arn',
        issuerS3Bucket: '',
        issuerS3Prefix: '',
      },
      statusCode: IntegrationStatusCode.Running,
    },
  };
}
