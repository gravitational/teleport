/*
Copyright 2020 Gravitational, Inc.

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

import { useEffect, useState, useAttempt } from 'shared/hooks';

import { Resource } from 'teleport/services/resources';
import useTeleport from 'teleport/useTeleport';

export default function useTrustedClusters() {
  const teleContext = useTeleport();
  const [items, setItems] = useState<Resource<'trusted_cluster'>[]>([]);
  const [attempt, attemptActions] = useAttempt({ isProcessing: true });
  const canCreate = teleContext.storeUser.getTrustedClusterAccess().create;

  function fetchData() {
    return teleContext.resourceService.fetchTrustedClusters().then(response => {
      setItems(response);
    });
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
    attemptActions.do(() => fetchData());
  }, []);

  return {
    canCreate,
    items,
    save,
    remove,
    ...attempt,
  };
}
