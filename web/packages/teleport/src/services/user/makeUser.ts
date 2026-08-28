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

import { User } from './types';

export default function makeUser(json: any): User {
  json = json || {};
  const { name, roles, authType, origin, traits = {}, allTraits, isBot } = json;

  return {
    name,
    roles: roles ? roles.sort() : [],
    authType: authType === 'local' ? 'teleport local user' : authType,
    isLocal: authType === 'local',
    isBot,
    origin: origin ? origin : '',
    traits: {
      logins: traits.logins || [],
      databaseUsers: traits.databaseUsers || [],
      databaseNames: traits.databaseNames || [],
      kubeUsers: traits.kubeUsers || [],
      kubeGroups: traits.kubeGroups || [],
      windowsLogins: traits.windowsLogins || [],
      awsRoleArns: traits.awsRoleArns || [],
    },
    allTraits: makeTraits(allTraits),
  };
}

export function makeUsers(json): User[] {
  json = json || [];
  return json.map(user => makeUser(user));
}

export function makeTraits(traits: Record<string, string[]>) {
  traits = traits || {};

  const traitKeys = Object.keys(traits);
  traitKeys.forEach(k => {
    if (!traits[k]) {
      traits[k] = [];
    }
  });

  return traits;
}
