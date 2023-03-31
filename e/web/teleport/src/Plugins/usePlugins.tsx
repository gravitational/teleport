import { useEffect, useState } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';

import useTeleport from 'e-teleport/useTeleportE';

import { Plugin } from '../services/plugins';

export function usePlugins() {
  const ctx = useTeleport();
  const [items, setItems] = useState<Plugin[]>([]);
  const { attempt, run } = useAttempt('processing');
  const [operation, setOperation] = useState({
    type: 'none',
  } as Operation);

  function fetchData() {
    return ctx.pluginsService.fetchPlugins().then(response => {
      setItems(response);
    });
  }

  useEffect(() => {
    run(() => fetchData());
  }, []);

  function onCancelDelete() {
    setOperation({ type: 'none' });
  }

  function onDelete(plugin: Plugin) {
    return ctx.pluginsService.deletePlugin(plugin.name).then(() => {
      const updatedItems = items.filter(p => p.name !== plugin.name);
      setItems(updatedItems);
    });
  }

  function onStartDelete(plugin: Plugin) {
    setOperation({ type: 'delete', plugin });
  }

  return {
    items,
    attempt,
    run,
    operation,
    onCancelDelete,
    onDelete,
    onStartDelete,
  };
}

export type State = ReturnType<typeof usePlugins>;

type Operation = { type: 'delete'; plugin: Plugin } | { type: 'none' };
