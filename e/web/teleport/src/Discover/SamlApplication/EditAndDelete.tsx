import { Box, Indicator } from 'design';

import { DiscoverUpdate } from 'e-teleport/Discover';
import { Delete } from 'e-teleport/SamlApplication/components/Delete';
import { Edit } from 'e-teleport/SamlApplication/components/Edit';
import ErrorMessage from 'teleport/components/AgentErrorMessage';
import { useSamlAppAction } from 'teleport/SamlApplications/useSamlAppActions';

/**
 * SamlAppEditAndDelete is used in Saml app edit and delete actions which
 * are triggered from the unified resources page.
 */
export function SamlAppEditAndDelete() {
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

  function EditContent() {
    switch (fetchSamlResourceAttempt.status) {
      case '':
      case 'processing':
        return (
          <Box textAlign="center" m={10}>
            <Indicator />
          </Box>
        );
      case 'error':
        return <ErrorMessage message={fetchSamlResourceAttempt.statusText} />;
      case 'success':
        return (
          <DiscoverUpdate
            resourceSpec={resourceSpec}
            agentMeta={fetchSamlResourceAttempt.data}
          />
        );
    }
  }

  return (
    <>
      {openEdit && (
        <Edit open={openEdit} onClose={clearAction} Content={EditContent} />
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
