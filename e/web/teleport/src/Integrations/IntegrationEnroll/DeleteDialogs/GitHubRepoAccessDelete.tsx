import { Alert, Box, ButtonSecondary, ButtonWarning, Mark, P1 } from 'design';
import { Warning } from 'design/Alert';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';
import { useAsync } from 'shared/hooks/useAsync';

import { DeleteRequestOptions } from 'teleport/Integrations/Operations/IntegrationOperations';
import { IntegrationGitHub } from 'teleport/services/integrations';

export type Props = {
  onClose(): void;
  onRemove(opt?: DeleteRequestOptions): void;
  integration: IntegrationGitHub;
};

export function GitHubRepoAccessDelete({
  onClose,
  onRemove,
  integration,
}: Props) {
  const [deleteIntegrationAttempt, deleteIntegration] = useAsync(async () =>
    onRemove({ deleteAssociatedResources: true })
  );

  const isDisabled = deleteIntegrationAttempt.status === 'processing';
  const isError = deleteIntegrationAttempt.status === 'error';

  return (
    <Dialog onClose={onClose} open={true}>
      <DialogHeader>
        <DialogTitle>Remove GitHub Integration</DialogTitle>
      </DialogHeader>
      <DialogContent width="450px">
        <P1>
          Are you sure you want to delete integration{' '}
          <Mark>{integration.name}</Mark>?
        </P1>
        <Box mt={3}>
          {isError ? (
            <Alert mb={0}>{deleteIntegrationAttempt.statusText}</Alert>
          ) : (
            <Warning mb={0}>
              Teleport will also remove Git servers that serviced this
              integration.
            </Warning>
          )}
        </Box>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning
          mr="3"
          width="105px"
          disabled={isDisabled}
          onClick={deleteIntegration}
        >
          {isError ? 'Retry' : 'Delete'}
        </ButtonWarning>
        <ButtonSecondary disabled={isDisabled} onClick={onClose}>
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
