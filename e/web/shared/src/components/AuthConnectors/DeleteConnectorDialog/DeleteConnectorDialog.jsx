import React from 'react';
import PropTypes from 'prop-types';
import { Box, ButtonWarning, ButtonSecondary, Text } from 'design';
import * as Alerts from 'design/Alert';
import { useAttempt } from 'shared/hooks';
import Dialog, {
  DialogContent,
  DialogFooter,
} from 'design/DialogConfirmation';

export default function DeleteConnectorDialog(props) {
  const { name, onClose, onDelete } = props;
  const [attempt, attempActions] = useAttempt();
  const isDisabled = attempt.isProcessing;

  function onOk() {
    attempActions.do(() => onDelete()).then(() => onClose());
  }

  return (
    <Dialog disableEscapeKeyDown={false} onClose={onClose} open={true}>
      <Box width="540px">
        {attempt.isFailed && <Alerts.Danger>{attempt.message}</Alerts.Danger>}
        <DialogContent>
          <Text typography="h2">Remove Connector?</Text>
          <Text typography="paragraph" mt="2" mb="6">
            Are you sure you want to delete connector{' '}
            <Text as="span" bold color="primary.contrastText">
              {name}
            </Text>
            ?
          </Text>
        </DialogContent>
        <DialogFooter>
          <ButtonWarning mr="3" disabled={isDisabled} onClick={onOk}>
            DELETE
          </ButtonWarning>
          <ButtonSecondary disabled={isDisabled} onClick={onClose}>
            Cancel
          </ButtonSecondary>
        </DialogFooter>
      </Box>
    </Dialog>
  );
}

DeleteConnectorDialog.propTypes = {
  onClose: PropTypes.func.isRequired,
  onDelete: PropTypes.func.isRequired,
  name: PropTypes.string.isRequired,
};
