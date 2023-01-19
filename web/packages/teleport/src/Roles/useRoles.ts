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

import { useEffect, useState } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';

import TeleportContext from 'teleport/teleportContext';
import { Resource, KindRole } from 'teleport/services/resources';

export default function useRoles(ctx: TeleportContext) {
  const [items, setItems] = useState<Resource<KindRole>[]>([]);
  const { attempt, run } = useAttempt('processing');

  function fetchData() {
    return ctx.resourceService.fetchRoles().then(received => {
      setItems(received);
    });
  }

  // TODO: we cannot refetch the data right after saving because this backend
  // operation is not atomic.
  function save(yaml: string, isNew: boolean) {
    if (isNew) {
      return ctx.resourceService.createRole(yaml).then(result => {
        setItems([result, ...items]);
      });
    }

    return ctx.resourceService.updateRole(yaml).then(result => {
      setItems([result, ...items.filter(r => r.name !== result.name)]);
    });
  }

  function remove(name: string) {
    return ctx.resourceService.deleteRole(name).then(() => {
      setItems(items.filter(r => r.name !== name));
    });
  }

  useEffect(() => {
    run(() => fetchData());
  }, []);

  return {
    items,
    attempt,
    save,
    remove,
  };
}

export type State = ReturnType<typeof useRoles>;
