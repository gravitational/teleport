/*
Copyright 2019-2020 Gravitational, Inc.

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

import { Cluster } from '../../services/clusters';

export type AuthType = 'local' | 'sso';

export interface UserContext {
  authType: AuthType;
  acl: Acl;
  username: string;
  cluster: Cluster;
  accessStrategy: AccessStrategy;
}

export interface Access {
  list: boolean;
  read: boolean;
  edit: boolean;
  create: boolean;
  remove: boolean;
}

export interface Acl {
  logins: string[];
  authConnectors: Access;
  trustedClusters: Access;
  roles: Access;
  sessions: Access;
  events: Access;
  users: Access;
  tokens: Access;
  appServers: Access;
}

export interface User {
  name: string;
  roles: string[];
  authType?: string;
  isLocal?: boolean;
}

export interface ResetToken {
  value: string;
  username: string;
  expires: Date;
}

export interface AccessRequest {
  id: string;
  state: 'NONE' | 'PENDING' | 'APPROVED' | 'DENIED' | 'APPLIED' | '';
  reason: string;
}

export interface AccessStrategy {
  type: 'optional' | 'always' | 'reason';
  prompt: string;
}

export type ResetPasswordType = 'invite' | 'password';
