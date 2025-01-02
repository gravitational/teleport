import { Alert, ButtonPrimary, ButtonSecondary } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import useAttempt from 'shared/hooks/useAttemptNext';

import { pluginsService } from 'e-teleport/services/plugins';
import { PluginKind } from 'teleport/services/integrations';

export const CleanupDialogue = ({
  onClose,
  pluginKind,
}: {
  onClose(): void;
  pluginKind: PluginKind;
}) => {
  const { attempt, run } = useAttempt('');

  function cleanUpPlugin() {
    run(() => pluginsService.cleanupPlugin(pluginKind).then(onClose));
  }
  return (
    <Dialog open={true}>
      <DialogHeader>
        <DialogTitle>Cleanup Required</DialogTitle>
      </DialogHeader>
      {attempt.status === 'failed' && (
        <Alert kind="danger" children={attempt.statusText} mb={3} mt={3} />
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
          disabled={attempt.status === 'processing'}
          onClick={cleanUpPlugin}
        >
          Clean Up
        </ButtonPrimary>
        <ButtonSecondary
          width="45%"
          disabled={attempt.status === 'processing'}
          onClick={onClose}
        >
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
};
