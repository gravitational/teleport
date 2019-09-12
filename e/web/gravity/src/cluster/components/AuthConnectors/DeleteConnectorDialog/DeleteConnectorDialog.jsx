import React from 'react';
import PropTypes from 'prop-types';
import { Box, ButtonSecondary, ButtonPrimary, Text } from 'design';
import * as Alerts from 'design/Alert';
import Dialog, {
  DialogHeader,
  DialogContent,
  DialogFooter,
} from 'design/DialogConfirmation';
import { useAttempt, withState } from 'shared/hooks';
import { deleteAuthProvider } from 'e-gravity/cluster/flux/authConnectors/actions';

export function DeleteConnectorDialog(props) {
  const { connector, onClose, onDelete } = props;
  if (!connector) {
    return null;
  }

  // build state
  const [attempt, attempActions] = useAttempt();

  const onOk = () => {
    attempActions.do(() => onDelete(connector)).then(() => onClose());
  };

  const { name } = connector;
  const isDisabled = attempt.isProcessing;

  return (
    <Dialog disableEscapeKeyDown={false} onClose={onClose} open={true}>
      <Box width="540px">
        {attempt.isFailed && (
          <DialogHeader mb="0">
            <Alerts.Danger mb="0">{attempt.message}</Alerts.Danger>
          </DialogHeader>
        )}
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
          <ButtonPrimary mr="3" disabled={isDisabled} onClick={onOk}>
            DELETE
          </ButtonPrimary>
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
  connector: PropTypes.object,
};

export default withState(() => {
  return {
    onDelete: deleteAuthProvider,
  };
})(DeleteConnectorDialog);
