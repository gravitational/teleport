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

// IntegrationStatusCode must be in sync with the text values defined
// in the backend as these are used to determine the status color:
// https://github.com/gravitational/teleport.e/blob/1ebe50ce2fe608dc6dd24fef205fb9caaa216a46/lib/web/ui/plugins.go#L51
export type IntegrationStatusCode =
  | 'Unknown'
  | 'Running'
  | 'Unknown error'
  | 'Unauthorized'
  | 'Bot not invited to channel';

export type Plugin = Integration<'plugin', PluginKind, PluginSpec>;
export type PluginSpec = {
  statusDescription?: string;
};
export type PluginKind = 'slack';
