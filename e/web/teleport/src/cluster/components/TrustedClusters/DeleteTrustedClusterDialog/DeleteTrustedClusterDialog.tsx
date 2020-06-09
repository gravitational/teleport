import React from 'react';
import { Box, ButtonSecondary, ButtonWarning, Text } from 'design';
import * as Alerts from 'design/Alert';
import { useAttempt } from 'shared/hooks';
import Dialog, { DialogContent, DialogFooter } from 'design/DialogConfirmation';

export default function DeleteTrustedClusterDialog(props: Props) {
  const { name, onClose, onDelete } = props;
  const [attempt, attempActions] = useAttempt({ isProcessing: false });
  const isDisabled = attempt.isProcessing;

  function onOk() {
    attempActions.do(() => onDelete()).then(() => onClose());
  }

  return (
    <Dialog disableEscapeKeyDown={false} onClose={onClose} open={true}>
      <Box width="540px">
        {attempt.isFailed && <Alerts.Danger>{attempt.message}</Alerts.Danger>}
        <DialogContent>
          <Text typography="h3">Remove Trusted Cluster?</Text>
          <Text typography="paragraph" mt="2" mb="6">
            Are you sure you want to delete trusted cluster{' '}
            <Text as="span" bold color="primary.contrastText">
              {name}
            </Text>
            ?
          </Text>
        </DialogContent>
        <DialogFooter>
          <ButtonWarning mr="3" disabled={isDisabled} onClick={onOk}>
            Yes, Remove Trusted Cluster
          </ButtonWarning>
          <ButtonSecondary disabled={isDisabled} onClick={onClose}>
            Cancel
          </ButtonSecondary>
        </DialogFooter>
      </Box>
    </Dialog>
  );
}

type Props = {
  onClose: () => void;
  onDelete: () => Promise<any>;
  name: string;
};
