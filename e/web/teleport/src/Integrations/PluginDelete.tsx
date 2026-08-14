import { Alert, ButtonSecondary, ButtonWarning, P1 } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';
import useAttempt from 'shared/hooks/useAttemptNext';

import { PluginKind } from 'teleport/services/integrations';

export function PluginDelete(props: Props) {
  const { onClose, onDelete } = props;
  const { attempt, run } = useAttempt();
  const isDisabled = attempt.status === 'processing';
  const prettyPluginKind =
    props.pluginKind === 'okta'
      ? 'Okta Integration'
      : `${props.pluginKind} Plugin`;

  function onOk() {
    run(() => onDelete()).then(ok => ok && onClose());
  }

  return (
    <Dialog onClose={onClose} open={true}>
      <DialogHeader>
        <DialogTitle>Delete {prettyPluginKind}?</DialogTitle>
      </DialogHeader>
      <DialogContent width="450px">
        {attempt.status === 'failed' && (
          <Alert kind="outline-danger">{attempt.statusText}</Alert>
        )}
        <P1>Are you sure you want to delete the {prettyPluginKind}?</P1>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning mr="3" disabled={isDisabled} onClick={onOk}>
          Yes, delete
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
  onDelete(): Promise<void>;
  pluginKind: PluginKind;
};
