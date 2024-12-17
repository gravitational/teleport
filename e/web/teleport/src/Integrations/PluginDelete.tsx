import { ButtonSecondary, ButtonWarning, Alert, P1 } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/DialogConfirmation';
import useAttempt from 'shared/hooks/useAttemptNext';
import { PluginKind } from 'teleport/services/integrations';

export function PluginDelete(props: Props) {
  const { onClose, onDelete } = props;
  const { attempt, run } = useAttempt();
  const isDisabled = attempt.status === 'processing';

  function onOk() {
    run(() => onDelete()).then(ok => ok && onClose());
  }

  return (
    <Dialog disableEscapeKeyDown={false} onClose={onClose} open={true}>
      <DialogHeader>
        <DialogTitle>Remove Plugin?</DialogTitle>
      </DialogHeader>
      <DialogContent width="450px">
        {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
        <P1>Are you sure you want to delete the {props.pluginKind} plugin?</P1>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning mr="3" disabled={isDisabled} onClick={onOk}>
          Yes, Remove Plugin
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
