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

import cfg, {
  UrlDeployServiceIamConfigureScriptParams,
  UrlAwsOidcConfigureIdp,
} from './config';

test('getDeployServiceIamConfigureScriptPath formatting', async () => {
  const params: UrlDeployServiceIamConfigureScriptParams = {
    integrationName: 'int-name',
    region: 'us-east-1',
    awsOidcRoleArn: 'oidc-arn',
    taskRoleArn: 'task-arn',
  };
  const base =
    'http://localhost/webapi/scripts/integrations/configure/deployservice-iam.sh?';
  const expected = `integrationName=${'int-name'}&awsRegion=${'us-east-1'}&role=${'oidc-arn'}&taskRole=${'task-arn'}`;
  expect(cfg.getDeployServiceIamConfigureScriptUrl(params)).toBe(
    `${base}${expected}`
  );
});

test('getAwsOidcConfigureIdpScriptUrl formatting with s3 fields', async () => {
  const params: UrlAwsOidcConfigureIdp = {
    integrationName: 'int-name',
    roleName: 'role-arn',
    s3Bucket: 's3-bucket',
    s3Prefix: 's3-prefix',
  };
  const base =
    'http://localhost/webapi/scripts/integrations/configure/awsoidc-idp.sh?';
  const expected = `integrationName=int-name&role=role-arn&s3Bucket=s3-bucket&s3Prefix=s3-prefix`;
  expect(cfg.getAwsOidcConfigureIdpScriptUrl(params)).toBe(
    `${base}${expected}`
  );
});

test('getAwsOidcConfigureIdpScriptUrl formatting, without s3 fields', async () => {
  const params: UrlAwsOidcConfigureIdp = {
    integrationName: 'int-name',
    roleName: 'role-arn',
  };
  const base =
    'http://localhost/webapi/scripts/integrations/configure/awsoidc-idp.sh?';
  const expected = `integrationName=int-name&role=role-arn`;
  expect(cfg.getAwsOidcConfigureIdpScriptUrl(params)).toBe(
    `${base}${expected}`
  );
});
