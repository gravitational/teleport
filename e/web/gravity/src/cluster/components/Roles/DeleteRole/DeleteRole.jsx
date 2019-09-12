import React from 'react'
import PropTypes from 'prop-types';
import { ButtonSecondary, ButtonPrimary, Text } from 'design';
import * as Alerts from 'design/Alert';
import { useAttempt, withState } from 'shared/hooks';
import Dialog, { DialogHeader, DialogContent, DialogFooter} from 'design/DialogConfirmation';
import * as actions from 'e-gravity/cluster/flux/roles/actions';

export function DeleteRoleDialog(props){
  const { role, onClose, onDelete } = props;
  if(!role){
    return null;
  }

  const [ attempt, attemptActions ] = useAttempt();

  const onOk = () => {
    attemptActions.do(() => onDelete(role))
      .then(() => onClose());
  };

  const { name } = role;
  const isDisabled = attempt.isProcessing;

  return (
    <Dialog
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      {attempt.isFailed &&  (
        <DialogHeader mb="0">
          <Alerts.Danger mb="0">
            {attempt.message}
          </Alerts.Danger>
        </DialogHeader>
      )}
      <DialogContent width="400px">
        <Text typography="h2">Remove Role?</Text>
        <Text typography="paragraph" mt="2" mb="6">
          Are you sure you want to delete role <Text as="span" bold color="primary.contrastText">{name}</Text> ?
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
    </Dialog>
  );
}

DeleteRoleDialog.propTypes = {
  onClose: PropTypes.func.isRequired,
  onDelete: PropTypes.func.isRequired,
  role: PropTypes.object,
}

function mapState(){
  return {
    onDelete: actions.deleteRole
  }
}

export default withState(mapState)(DeleteRoleDialog)