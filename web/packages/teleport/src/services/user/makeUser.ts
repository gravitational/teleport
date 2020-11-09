/**
 * Copyright 2020 Gravitational, Inc.
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

import { at } from 'lodash';
import { User } from './types';

export default function makeUser(json): User {
  const [name, roles, authType] = at(json, ['name', 'roles', 'authType']);
  return {
    name,
    roles,
    authType: authType === 'local' ? 'teleport local user' : authType,
    isLocal: authType === 'local',
  };
}

export function makeUsers(json): User[] {
  return json.map(user => makeUser(user));
}
