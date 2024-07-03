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
    'http://localhost/v1/webapi/scripts/integrations/configure/deployservice-iam.sh?';
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
    'http://localhost/v1/webapi/scripts/integrations/configure/awsoidc-idp.sh?';
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
    'http://localhost/v1/webapi/scripts/integrations/configure/awsoidc-idp.sh?';
  const expected = `integrationName=int-name&role=role-arn`;
  expect(cfg.getAwsOidcConfigureIdpScriptUrl(params)).toBe(
    `${base}${expected}`
  );
});
