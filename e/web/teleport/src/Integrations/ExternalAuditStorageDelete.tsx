import React from 'react';
import { ButtonSecondary, ButtonWarning, Text, Alert } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
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
        <Text typography="paragraph" mb="2">
          Are you sure you want to delete this integration?
        </Text>
        <Text typography="paragraph" mb="2">
          Teleport will not remove the infrastructure created during
          integration.
        </Text>
        {props.opType === 'cluster' && (
          <Text bold typography="paragraph">
            Any audit logs and session recordings created when the integration
            was active will be inaccessible to Teleport.
          </Text>
        )}
        <Text></Text>
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
