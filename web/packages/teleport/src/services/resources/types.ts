/**
 * Copyright 2021 Gravitational, Inc.
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

export type Resource<T extends Kind> = {
  id: string;
  kind: T;
  name: string;
  // content is config in yaml format.
  content: string;
};

export type KindRole = 'role';
export type KindTrustedCluster = 'trusted_cluster';
export type KindAuthConnectors = 'github' | 'saml' | 'oidc';
export type KindSamlIdpServiceProvider = 'saml_idp_service_provider';
export type Kind =
  | KindRole
  | KindTrustedCluster
  | KindAuthConnectors
  | KindSamlIdpServiceProvider;
