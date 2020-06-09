import React from 'react';
import PropTypes from 'prop-types';
import { ButtonSecondary, ButtonWarning, Text } from 'design';
import * as Alerts from 'design/Alert';
import { useAttempt } from 'shared/hooks';
import Dialog, {
  DialogContent,
  DialogFooter,
} from 'design/DialogConfirmation';

export default function DeleteRoleDialog(props) {
  const { name, onClose, onDelete } = props;
  const [attempt, attemptActions] = useAttempt();
  const isDisabled = attempt.isProcessing;

  function onOk() {
    attemptActions.do(() => onDelete()).then(() => onClose());
  }

  return (
    <Dialog disableEscapeKeyDown={false} onClose={onClose} open={true}>
      {attempt.isFailed && <Alerts.Danger>{attempt.message}</Alerts.Danger>}
      <DialogContent width="400px">
        <Text typography="h3">Remove Role?</Text>
        <Text typography="paragraph" mt="2" mb="6">
          Are you sure you want to delete role{' '}
          <Text as="span" bold color="primary.contrastText">
            {name}
          </Text>{' '}
          ?
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning mr="3" disabled={isDisabled} onClick={onOk}>
          Yes, Remove Role
        </ButtonWarning>
        <ButtonSecondary disabled={isDisabled} onClick={onClose}>
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

DeleteRoleDialog.propTypes = {
  onClose: PropTypes.func.isRequired,
  onDelete: PropTypes.func.isRequired,
  name: PropTypes.string.isRequired,
};
