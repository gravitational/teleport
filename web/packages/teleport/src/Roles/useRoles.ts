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

import TeleportContext from 'teleport/teleportContext';

import type { UrlListRolesParams } from 'teleport/config';

export function useRoles(ctx: TeleportContext) {
  function save(name: string, yaml: string, isNew: boolean) {
    if (isNew) {
      return ctx.resourceService.createRole(yaml);
    }

    return ctx.resourceService.updateRole(name, yaml);
  }

  function remove(name: string) {
    return ctx.resourceService.deleteRole(name);
  }

  function fetch(params?: UrlListRolesParams) {
    return ctx.resourceService.fetchRoles(params);
  }

  return {
    fetch,
    save,
    remove,
  };
}

export type State = ReturnType<typeof useRoles>;
