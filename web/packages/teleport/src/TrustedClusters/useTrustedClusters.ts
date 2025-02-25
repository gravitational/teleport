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

import { Resource } from 'teleport/services/resources';
import useTeleport from 'teleport/useTeleport';

export default function useTrustedClusters() {
  const teleContext = useTeleport();
  const [items, setItems] = useState<Resource<'trusted_cluster'>[]>([]);
  const { attempt, setAttempt, handleError } = useAttempt('');
  const canCreate = teleContext.storeUser.getTrustedClusterAccess().create;

  function fetchData() {
    setAttempt({ status: 'processing' });
    teleContext.resourceService
      .fetchTrustedClusters()
      .then(response => {
        setItems(response);
        setAttempt({ status: 'success' });
      })
      .catch(handleError);
  }

  function save(name: string, yaml: string, isNew: boolean) {
    if (isNew) {
      return teleContext.resourceService
        .createTrustedCluster(yaml)
        .then(fetchData);
    }
    return teleContext.resourceService
      .updateTrustedCluster(name, yaml)
      .then(fetchData);
  }

  function remove(name: string) {
    return teleContext.resourceService.deleteTrustedCluster(name).then(() => {
      setItems(items.filter(r => r.name !== name));
    });
  }

  useEffect(() => {
    fetchData();
  }, []);

  return {
    canCreate,
    items,
    save,
    remove,
    attempt,
  };
}
