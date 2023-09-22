/**
 * Copyright 2020-2022 Gravitational, Inc.
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

import { User } from './types';

export default function makeUser(json: any): User {
  json = json || {};
  const { name, roles, authType, traits = {} } = json;

  return {
    name,
    roles: roles ? roles.sort() : [],
    authType: authType === 'local' ? 'teleport local user' : authType,
    isLocal: authType === 'local',
    traits: {
      logins: traits.logins || [],
      databaseUsers: traits.databaseUsers || [],
      databaseNames: traits.databaseNames || [],
      kubeUsers: traits.kubeUsers || [],
      kubeGroups: traits.kubeGroups || [],
      windowsLogins: traits.windowsLogins || [],
      awsRoleArns: traits.awsRoleArns || [],
    },
  };
}

export function makeUsers(json): User[] {
  json = json || [];
  return json.map(user => makeUser(user));
}
