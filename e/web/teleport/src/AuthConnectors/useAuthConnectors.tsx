import { useEffect, useState } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import { Resource, KindAuthConnectors } from 'teleport/services/resources';

import useTeleport from 'e-teleport/useTeleportE';

export default function useAuthConnectors() {
  const ctx = useTeleport();
  const [items, setItems] = useState<Resource<KindAuthConnectors>[]>([]);
  const { attempt, run } = useAttempt('processing');

  function fetchData() {
    return ctx.resourceService.fetchAuthConnectors().then(response => {
      setItems(response);
    });
  }

  function save(yaml: string, isNew: boolean, kind: KindAuthConnectors) {
    if (isNew) {
      return ctx.resourceService.createConnector(kind, yaml).then(fetchData);
    }

    return ctx.resourceService.updateConnector(kind, yaml).then(fetchData);
  }

  function remove(connector: Resource<KindAuthConnectors>) {
    const { kind, name } = connector;
    return ctx.resourceService.deleteConnector(kind, name).then(fetchData);
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

export type State = ReturnType<typeof useAuthConnectors>;
