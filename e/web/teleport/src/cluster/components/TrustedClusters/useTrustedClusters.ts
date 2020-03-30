import { useEffect, useState, useAttempt } from 'shared/hooks';
import { useTeleportE } from 'e-teleport/teleportEContext';
import { Resource } from 'e-teleport/services/resources';

export default function useTrustedClusters() {
  const teleContext = useTeleportE();
  const [items, setItems] = useState<Resource[]>([]);
  const [attempt, attemptActions] = useAttempt({ isProcessing: true });
  const canCreate = teleContext.storeUser.getTrustedClusterAccess().create;

  function fetchData() {
    return teleContext.resourceService.fetchTrustedClusters().then(response => {
      setItems(response);
    });
  }

  function save(yaml: string, isNew: boolean) {
    return teleContext.resourceService
      .upsertTrustedCluster(yaml, isNew)
      .then(fetchData);
  }

  function remove(trustedCluster: Resource) {
    const { kind, name } = trustedCluster;
    return teleContext.resourceService.delete(kind, name).then(() => {
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
