import { useSamlAppAction } from 'teleport/SamlApplications/useSamlAppActions';

import { Delete } from './Delete';
import { Edit } from './Edit';

export function SamlAppActionsComponent() {
  const {
    currentAction,
    fetchSamlResourceAttempt,
    resourceSpec,
    onDelete,
    deleteSamlAppAttempt,
    clearAction,
  } = useSamlAppAction();

  const openEdit = currentAction === 'edit';
  const openDelete = currentAction === 'delete';

  return (
    <>
      {openEdit && (
        <Edit
          open={openEdit}
          onClose={clearAction}
          resourceSpec={resourceSpec}
          agentMeta={fetchSamlResourceAttempt.data}
          attempt={fetchSamlResourceAttempt}
        />
      )}
      {openDelete && (
        <Delete
          open={openDelete}
          onClose={clearAction}
          appName={resourceSpec.name}
          attempt={deleteSamlAppAttempt}
          onDelete={onDelete}
        />
      )}
    </>
  );
}
