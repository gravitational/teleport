/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react'
import PropTypes from 'prop-types';
import { Box, ButtonSecondary, ButtonWarning, Text } from 'design';
import * as Alerts from 'design/Alert';
import { useAttempt, withState } from 'shared/hooks';
import Dialog, { DialogTitle, DialogHeader, DialogContent, DialogFooter} from 'design/DialogConfirmation';
import * as actions from 'gravity/cluster/flux/users/actions';

export function UserDeleteDialog(props){
  const { user, onClose, onDelete, attempt, attemptActions } = props;

  const onOk = () => {
    attemptActions.do(() => onDelete(user))
      .then(() => onClose());
  };

  const { userId } = user;
  const isDisabled = attempt.isProcessing;

  return (
    <Dialog
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <Box maxWidth="500px">
        <DialogHeader>
          <DialogTitle>Remove User?</DialogTitle>
        </DialogHeader>
        <DialogContent>
          {attempt.isFailed && (
            <Alerts.Danger children={attempt.message} />
          )}
          <Text typography="paragraph">
            You are about to remove {userId}.
            This will revoke the user's access to the current cluster. This action cannot be undone.
          </Text>
        </DialogContent>
        <DialogFooter>
          <ButtonWarning mr="3" disabled={isDisabled} onClick={onOk}>
            I Understand, Remove User
          </ButtonWarning>
          <ButtonSecondary disabled={isDisabled} onClick={onClose}>
            Cancel
          </ButtonSecondary>
        </DialogFooter>
      </Box>
    </Dialog>
  );
}

UserDeleteDialog.propTypes = {
  onClose: PropTypes.func.isRequired,
  onDelete: PropTypes.func.isRequired,
  user: PropTypes.object,
}

function mapState(){
  const [ attempt, attemptActions ] = useAttempt();
  return {
    onDelete: actions.deleteUser,
    attempt,
    attemptActions
  }
}

export default withState(mapState)(UserDeleteDialog)