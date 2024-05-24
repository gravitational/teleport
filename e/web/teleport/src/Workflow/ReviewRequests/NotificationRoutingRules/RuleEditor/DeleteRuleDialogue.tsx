import React from 'react';
import { ButtonSecondary, ButtonWarning, Text, Alert } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/DialogConfirmation';
import useAttempt from 'shared/hooks/useAttemptNext';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { accessMonitoringRuleService } from 'e-teleport/services/accessmonitoringrule';

export function DeleteRuleDialogue({
  name,
  onClose,
  onDelete,
}: {
  onClose(): void;
  onDelete(): void;
  name: string;
}) {
  const { clusterId } = useStickyClusterId();
  const { attempt, run } = useAttempt();
  const isDisabled = attempt.status === 'processing';

  function handleDelete() {
    run(() =>
      accessMonitoringRuleService
        .deleteAccessMonitoringRule({
          clusterId,
          name: name,
        })
        .then(onDelete)
    );
  }

  return (
    <Dialog onClose={onClose} open={true}>
      <DialogHeader>
        <DialogTitle>Delete Rule?</DialogTitle>
      </DialogHeader>
      <DialogContent width="450px">
        {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
        <Text typography="paragraph" mb="2">
          Are you sure you want to delete rule "{name}"
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning mr="3" disabled={isDisabled} onClick={handleDelete}>
          Yes, Delete Rule
        </ButtonWarning>
        <ButtonSecondary disabled={isDisabled} onClick={onClose}>
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
