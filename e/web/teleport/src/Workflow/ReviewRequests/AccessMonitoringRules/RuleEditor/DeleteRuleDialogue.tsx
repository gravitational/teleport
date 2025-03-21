import { Alert, ButtonSecondary, ButtonWarning, P1 } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';
import useAttempt from 'shared/hooks/useAttemptNext';

import { accessMonitoringRuleService } from 'e-teleport/services/accessmonitoringrule';
import useStickyClusterId from 'teleport/useStickyClusterId';

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
        <P1>Are you sure you want to delete rule “{name}”?</P1>
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
