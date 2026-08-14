import { useMutation } from '@tanstack/react-query';

import { Alert, ButtonPrimary, ButtonSecondary } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import { getErrMessage } from 'shared/utils/errorType';

import { pluginsService } from 'e-teleport/services/plugins';
import { PluginKind } from 'teleport/services/integrations';

export const CleanupDialogue = ({
  onClose,
  pluginKind,
}: {
  onClose(): void;
  pluginKind: PluginKind;
}) => {
  const cleanUp = useMutation({
    mutationFn: pluginsService.cleanupPlugin,
    onSuccess: onClose,
  });

  function cleanUpPlugin() {
    cleanUp.mutate(pluginKind);
  }

  return (
    <Dialog open={true}>
      <DialogHeader>
        <DialogTitle>Cleanup Required</DialogTitle>
      </DialogHeader>
      {cleanUp.isError && (
        <Alert kind="danger" mb={3} mt={3}>
          {getErrMessage(cleanUp.error)}
        </Alert>
      )}
      <DialogContent width="400px">
        Teleport has detected artifacts from a previous, disconnected Okta
        integration that could interfere with a new integration. <br />
        <br />
        Click "Clean Up" to delete Okta-generated access lists and roles from
        your Teleport cluster before proceeding. <br />
        <br />
        If you previously assigned owners to access lists, you will need to
        reassign them after setup.
      </DialogContent>
      <DialogFooter>
        <ButtonPrimary
          mr={4}
          width="45%"
          disabled={cleanUp.isPending}
          onClick={cleanUpPlugin}
        >
          Clean Up
        </ButtonPrimary>
        <ButtonSecondary
          width="45%"
          disabled={cleanUp.isPending}
          onClick={onClose}
        >
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
};
