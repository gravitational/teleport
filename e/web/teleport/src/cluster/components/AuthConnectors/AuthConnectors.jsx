import React from 'react';
import { withState, useAttempt } from 'shared/hooks';
import AuthConnectors from 'e-shared/components/AuthConnectors';
import service from 'e-teleport/cluster/services/resources';
import { useStoreUser } from 'teleport/teleportContext';

export default withState(() => {
  const [connectors, setConnectors] = React.useState([]);
  const [attempt, attemptActions] = useAttempt({ isProcessing: true });
  const storeUser = useStoreUser();
  const canCreate = storeUser.getConnectorAccess().create;

  function fetchProviders() {
    return service.fetchAuthConnectors().then(response => {
      setConnectors(response);
    });
  }

  function onSave(yaml, isNew) {
    return service.upsertAuthConnector(yaml, isNew).then(fetchProviders);
  }

  function onDelete(connector) {
    const { kind, name } = connector;
    return service.delete(kind, name).then(fetchProviders);
  }

  React.useEffect(() => {
    attemptActions.do(() => fetchProviders());
  }, []);

  return {
    canCreate,
    connectors,
    attempt,
    onSave,
    onDelete,
  };
})(AuthConnectors);
