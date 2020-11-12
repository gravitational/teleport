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

export default function useAuthConnectors() {
  const ctx = useTeleport();
  const [items, setItems] = useState<Resource[]>([]);
  const [attempt, attemptActions] = useAttempt({ isProcessing: true });

  function fetchData() {
    return ctx.resourceService.fetchAuthConnectors().then(response => {
      setItems(response);
    });
  }

  function save(yaml: string, isNew: boolean) {
    return ctx.resourceService.upsertAuthConnector(yaml, isNew).then(fetchData);
  }

  function remove(connector: Resource) {
    const { kind, name } = connector;
    return ctx.resourceService.delete(kind, name).then(fetchData);
  }

  useEffect(() => {
    attemptActions.do(() => fetchData());
  }, []);

  return {
    items,
    attempt,
    save,
    remove,
  };
}

export type State = ReturnType<typeof useAuthConnectors>;
