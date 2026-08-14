import { Alert, ButtonSecondary, ButtonWarning, P1 } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';
import useAttempt from 'shared/hooks/useAttemptNext';

import { ExternalAuditStorageOpType } from 'teleport/Integrations/Operations/useIntegrationOperation';

export function ExternalAuditStorageDelete(props: Props) {
  const { onClose, onDelete } = props;
  const { attempt, run } = useAttempt();
  const isDisabled = attempt.status === 'processing';

  function onOk() {
    run(() => onDelete()).then(ok => ok && onClose());
  }

  return (
    <Dialog disableEscapeKeyDown={false} onClose={onClose} open={true}>
      <DialogHeader>
        <DialogTitle>Remove External Audit Storage integration?</DialogTitle>
      </DialogHeader>
      <DialogContent width="450px">
        {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
        <P1>Are you sure you want to delete this integration?</P1>
        <P1>
          Teleport will not remove the infrastructure created during
          integration.
        </P1>
        {props.opType === 'cluster' && (
          <P1 bold>
            Any audit logs and session recordings created when the integration
            was active will be inaccessible to Teleport.
          </P1>
        )}
      </DialogContent>
      <DialogFooter>
        <ButtonWarning mr="3" disabled={isDisabled} onClick={onOk}>
          Yes
        </ButtonWarning>
        <ButtonSecondary disabled={isDisabled} onClick={onClose}>
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

type Props = {
  onClose(): void;
  onDelete(): Promise<any>;
  opType: ExternalAuditStorageOpType;
};
