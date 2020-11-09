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

import { values, keyBy } from 'lodash';
import { useEffect, useState, useAttempt } from 'shared/hooks';
import TeleportContext from 'teleport/teleportContext';
import { Resource } from 'teleport/services/resources';

export default function useRoles(ctx: TeleportContext) {
  const [items, setItems] = useState<Resource[]>([]);
  const [attempt, attemptActions] = useAttempt({ isProcessing: true });
  const canCreate = ctx.storeUser.getRoleAccess().create;

  function fetchData() {
    return ctx.resourceService.fetchRoles().then(received => {
      setItems(received);
    });
  }

  function save(yaml: string, isNew: boolean) {
    return ctx.resourceService.upsertRole(yaml, isNew).then(received => {
      // TODO: we cannot refetch the data right after saving because this backend
      // operation is not atomic.
      setItems(
        values({
          ...keyBy(items, 'id'),
          ...keyBy(received, 'id'),
        })
      );
    });
  }

  function remove(role: Resource) {
    const { kind, name } = role;
    return ctx.resourceService.delete(kind, name).then(() => {
      setItems(items.filter(r => r.name !== name));
    });
  }

  useEffect(() => {
    attemptActions.do(() => fetchData());
  }, []);

  return {
    canCreate,
    items,
    attempt,
    save,
    remove,
  };
}
