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

import { ResourceIdKind } from 'teleport/services/agents';

/** Available request kinds for resource-based and role-based access requests. */
export type ResourceKind = ResourceIdKind | 'role' | 'resource';

export type ResourceMap = {
  [K in ResourceIdKind | 'role']: Record<string, string>;
};

export function getEmptyResourceState(): ResourceMap {
  return {
    node: {},
    db: {},
    app: {},
    kube_cluster: {},
    user_group: {},
    windows_desktop: {},
    role: {},
  };
}
