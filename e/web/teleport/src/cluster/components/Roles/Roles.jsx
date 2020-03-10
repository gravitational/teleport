import React from 'react';
import { values, keyBy } from 'lodash';
import { withState, useAttempt } from 'shared/hooks';
import Roles from 'e-shared/components/Roles';
import service from 'e-teleport/cluster/services/resources';
import { useStoreUser } from 'teleport/teleportContext';

export default withState(() => {
  const [roles, setRoles] = React.useState([]);
  const [attempt, attemptActions] = useAttempt({ isProcessing: true });
  const storeUser = useStoreUser();
  const canCreate = storeUser.getConnectorAccess().create;

  function fetchRoles() {
    return service.fetchRoles().then(response => {
      setRoles(response);
    });
  }

  function onSave(yaml, isNew) {
    return service.upsertRole(yaml, isNew).then(response => {
      const keyedRoles = keyBy(roles, 'id');
      const keyedReceivedRoles = keyBy(response, 'id');
      setRoles(
        values({
          ...keyedRoles,
          ...keyedReceivedRoles,
        })
      );
    });
  }

  function onDelete(role) {
    const { kind, name } = role;
    return service.delete(kind, name).then(() => {
      setRoles(roles.filter(r => r.name !== name));
    });
  }

  React.useEffect(() => {
    attemptActions.do(() => fetchRoles());
  }, []);

  return {
    canCreate,
    roles,
    attempt,
    onSave,
    onDelete,
  };
})(Roles);
