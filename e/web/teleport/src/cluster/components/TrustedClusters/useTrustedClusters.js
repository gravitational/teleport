import React from 'react';
import { useAttempt } from 'shared/hooks';
import service from 'e-teleport/cluster/services/resources';
import { useStoreUser } from 'teleport/teleport';

export default function useTrustedClusters() {
  const [trustedClusters, setTrustedClusters] = React.useState([]);
  const [attempt, attemptActions] = useAttempt({ isProcessing: true });
  const storeUser = useStoreUser();
  const canCreate = storeUser.getConnectorAccess().create;

  function fetchTrustedClusters() {
    return service.fetchTrustedClusters().then(response => {
      setTrustedClusters(response);
    });
  }

  function onSave(yaml, isNew) {
    return service.upsertTrustedCluster(yaml, isNew).then(fetchTrustedClusters);
  }

  function onDelete(connector) {
    const { kind, name } = connector;
    return service.delete(kind, name).then(fetchTrustedClusters);
  }

  React.useEffect(() => {
    attemptActions.do(() => fetchTrustedClusters());
  }, []);

  return {
    canCreate,
    trustedClusters,
    attempt,
    onSave,
    onDelete,
  };
}
