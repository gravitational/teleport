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
  function save(name: string, yaml: string, isNew: boolean) {
    if (isNew) {
      return ctx.resourceService.createRole(yaml).then(result => {
        setItems([result, ...items]);
      });
    }

    return ctx.resourceService.updateRole(name, yaml).then(result => {
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
