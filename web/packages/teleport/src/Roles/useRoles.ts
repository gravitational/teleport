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

import type { UrlListRolesParams } from 'teleport/config';
import { RoleWithYaml } from 'teleport/services/resources';
import { yamlService } from 'teleport/services/yaml';
import { YamlSupportedResourceKind } from 'teleport/services/yaml/types';
import TeleportContext from 'teleport/teleportContext';

export function useRoles(ctx: TeleportContext) {
  const rolesAcl = ctx.storeUser.getRoleAccess();

  async function create(role: Partial<RoleWithYaml>) {
    return ctx.resourceService.createRole(await toYaml(role));
  }

  async function update(name: string, role: Partial<RoleWithYaml>) {
    return ctx.resourceService.updateRole(name, await toYaml(role));
  }

  function remove(name: string) {
    return ctx.resourceService.deleteRole(name);
  }

  function fetch(params?: UrlListRolesParams) {
    return ctx.resourceService.fetchRoles(params);
  }

  return {
    fetch,
    create,
    update,
    remove,
    rolesAcl,
  };
}

async function toYaml(role: Partial<RoleWithYaml>): Promise<string> {
  return (
    role.yaml ||
    (await yamlService.stringify(YamlSupportedResourceKind.Role, {
      resource: role.object,
    }))
  );
}

export type State = ReturnType<typeof useRoles>;
