import React from 'react';
import { ButtonSecondary, ButtonWarning, Text } from 'design';
import * as Alerts from 'design/Alert';
import Dialog, { DialogContent, DialogFooter } from 'design/DialogConfirmation';
import useAttempt from 'shared/hooks/useAttemptNext';

export default function RemoveDialog(props: Props) {
  const { name, onCancel, onRemove } = props;
  const { attempt, handleError, setAttempt } = useAttempt('');

  function onConfirm() {
    setAttempt({ status: 'processing' });
    onRemove().catch(handleError);
  }

  return (
    <Dialog disableEscapeKeyDown={false} onClose={onCancel} open={true}>
      {attempt.status == 'failed' && (
        <Alerts.Danger>{attempt.statusText}</Alerts.Danger>
      )}
      <DialogContent width="400px">
        <Text typography="h2">Remove Device</Text>
        <Text typography="paragraph" mt="2" mb="6">
          Are you sure you want to remove device{' '}
          <Text as="span" bold color="primary.contrastText">
            {name}
          </Text>{' '}
          ?
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning
          mr="3"
          disabled={attempt.status === 'processing'}
          onClick={onConfirm}
        >
          Remove
        </ButtonWarning>
        <ButtonSecondary
          disabled={attempt.status === 'processing'}
          onClick={onCancel}
        >
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

type Props = {
  onCancel: () => void;
  onRemove: () => Promise<any>;
  name: string;
};
