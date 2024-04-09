/*
Copyright 2019-2021 Gravitational, Inc.

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
