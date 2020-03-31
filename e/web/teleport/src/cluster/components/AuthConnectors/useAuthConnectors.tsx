import { useEffect, useState, useAttempt } from 'shared/hooks';
import { useTeleportE } from 'e-teleport/teleportEContext';
import { Resource } from 'e-teleport/services/resources';

export default function useAuthConnectors() {
  const teleContext = useTeleportE();
  const [items, setItems] = useState<Resource[]>([]);
  const [attempt, attemptActions] = useAttempt({ isProcessing: true });

  function fetchData() {
    return teleContext.resourceService.fetchAuthConnectors().then(response => {
      setItems(response);
    });
  }

  function save(yaml: string, isNew: boolean) {
    return teleContext.resourceService
      .upsertAuthConnector(yaml, isNew)
      .then(fetchData);
  }

  function remove(connector: Resource) {
    const { kind, name } = connector;
    return teleContext.resourceService.delete(kind, name).then(fetchData);
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
