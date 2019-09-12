import React from 'react'
import PropTypes from 'prop-types';
import * as Alerts from 'design/Alert';
import { withState, useAttempt } from 'shared/hooks';
import { ButtonSecondary, ButtonPrimary, Text } from 'design';
import Dialog, { DialogHeader, DialogTitle, DialogContent, DialogFooter} from 'design/DialogConfirmation';
import { unlinkCluster } from 'e-gravity/hub/flux/actions';

export function ClusterDisconnectDialog(props){
  const { onClose, cluster, attempt, attemptActions, onDisconnect } = props;
  const isDisabled = attempt.isProcessing;
  const siteId = cluster.id;

  function onOk(){
    attemptActions
    .do(() => {
      return onDisconnect();
    })
    .then(() => onClose());
  }

  return (
    <Dialog disableEscapeKeyDown={isDisabled} onClose={onClose} open={true}>
      <DialogHeader>
        <DialogTitle>UNLINK A CLUSTER</DialogTitle>
      </DialogHeader>
      <DialogContent maxWidth="600px">
        {attempt.isFailed && <Alerts.Danger children={attempt.message} />}
        <Text typography="paragraph" color="primary.contrastText">
          You are about to permanently disconnect{" "}
          <Text as="span" bold>
            {siteId}
          </Text>{" "}
          cluster from the Hub.
          <Text>
            It will continue to work as usual in standalone mode.
          </Text>
          <Text> This operation cannot be undone. Are you sure? </Text>
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonPrimary mr="3" disabled={isDisabled} onClick={onOk}>
          Disconnect
        </ButtonPrimary>
        <ButtonSecondary disabled={isDisabled} onClick={onClose}>
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

ClusterDisconnectDialog.propTypes = {
  cluster: PropTypes.object.isRequired,
  attempt: PropTypes.object.isRequired,
  attemptActions: PropTypes.object.isRequired,
  onClose: PropTypes.func.isRequired,
  onDisconnect: PropTypes.func.isRequired,
}

function mapState(props) {
  const [ attempt, attemptActions ] = useAttempt();
  return {
    attempt,
    attemptActions,
    onDisconnect: () => unlinkCluster(props.cluster.id)
  }
}

export default withState(mapState)(ClusterDisconnectDialog);