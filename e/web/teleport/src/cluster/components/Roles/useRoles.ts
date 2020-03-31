import { values, keyBy } from 'lodash';
import { useEffect, useState, useAttempt } from 'shared/hooks';
import { useTeleportE } from 'e-teleport/teleportEContext';
import { Resource } from 'e-teleport/services/resources';

export default function useRoles() {
  const teleContext = useTeleportE();
  const [items, setItems] = useState<Resource[]>([]);
  const [attempt, attemptActions] = useAttempt({ isProcessing: true });
  const canCreate = teleContext.storeUser.getRoleAccess().create;

  function fetchData() {
    return teleContext.resourceService.fetchRoles().then(received => {
      setItems(received);
    });
  }

  function save(yaml: string, isNew: boolean) {
    return teleContext.resourceService
      .upsertRole(yaml, isNew)
      .then(received => {
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
    attempt,
    save,
    remove,
  };
}
