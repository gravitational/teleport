/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { FlatBot } from 'teleport/Bots/types';

export function makeBot(json: any): FlatBot {
  json = json || {};

  return {
    kind: json.kind,
    status: json.status,
    subKind: json.subKind,
    version: json.version,

    name: json.metadata.name,
    namespace: json.metadata.namespace,
    description: json.metadata.description,
    labels: json.metadata.labels,
    revision: json.metadata.revision,

    roles: json.spec.roles,
    traits: json.spec.traits,
  };
}
