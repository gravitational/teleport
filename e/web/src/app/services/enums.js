/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

export const ResourceEnum = {
  SAML: 'saml',
  OIDC: 'oidc',
  ROLE: 'role',
  AUTH_CONNECTORS: 'auth_connector',
  TRUSTED_CLUSTER: 'trusted_cluster'
}

export const SshBuiltInLoginEnum = {
  ROOT : 'root'
}

export const K8sBuiltInGroupEnum = {
  ADMIN : 'admin',
  VIEW : 'view',
  EDIT : 'edit'
}

export const UserRoleSystemNameEnum = {
  ADMIN : '@teleadmin'
}

export const RestRespCodeEnum = {
  FORBIDDEN : 403
}
