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

/**
 * type Integration v. type Plugin:
 *
 * Before "integration" resource was made, a "plugin" resource existed.
 * They are essentially the same where plugin resource could've
 * been defined with the integration resource. But it's too late for
 * renames/changes. There are small differences between the two resource,
 * so they are separate types.
 *
 * "integration" resource is supported in both OS and Enterprise
 * while "plugin" resource is only supported in enterprise. Plugin
 * type exists in OS for easier typing when combining the resources
 * into one list.
 */
export type Integration<
  T extends string = 'integration',
  K extends string = IntegrationKind,
  S extends Record<string, any> = IntegrationSpecAwsOidc
> = {
  resourceType: T;
  kind: K;
  spec: S;
  name: string;
  details?: string;
  statusCode: IntegrationStatusCode;
};
export type IntegrationKind = 'aws-oidc';
export type IntegrationSpecAwsOidc = {
  roleArn: string;
};

export enum IntegrationStatusCode {
  Unknown = 0,
  Running = 1,
  OtherError = 2,
  Unauthorized = 3,
  SlackNotInChannel = 10,
}

export function getStatusCodeTitle(code: IntegrationStatusCode): string {
  switch (code) {
    case IntegrationStatusCode.Unknown:
      return 'Unknown';
    case IntegrationStatusCode.Running:
      return 'Running';
    case IntegrationStatusCode.Unauthorized:
      return 'Unauthorized';
    case IntegrationStatusCode.SlackNotInChannel:
      return 'Bot not invited to channel';
    default:
      return 'Unknown error';
  }
}

export function getStatusCodeDescription(
  code: IntegrationStatusCode
): string | null {
  switch (code) {
    case IntegrationStatusCode.Unauthorized:
      return 'The integration was denied access. This could be a result of revoked authorization on the 3rd party provider. Try removing and re-connecting the integration.';

    case IntegrationStatusCode.SlackNotInChannel:
      return 'The Slack integration must be invited to the default channel in order to receive access request notifications.';

    default:
      return null;
  }
}

export type Plugin = Integration<'plugin', PluginKind, PluginSpec>;
export type PluginSpec = Record<string, never>; // currently no 'spec' fields exposed to the frontend
export type PluginKind = 'slack';
