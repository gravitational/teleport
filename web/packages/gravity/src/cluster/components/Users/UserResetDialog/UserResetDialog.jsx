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
import { ButtonPrimary, ButtonSecondary, Text } from 'design';
import * as Alerts from 'design/Alert';
import { useAttempt, useState, withState } from 'shared/hooks';
import CmdText from 'gravity/components/CmdText';
import Dialog, { DialogContent, DialogFooter} from 'design/DialogConfirmation';
import * as actions from 'gravity/cluster/flux/users/actions';

export function UserResetDialog(props){
  const { userId, onClose, onReset, attempt, attemptActions, resetLink, setResetLink } = props;
  const { isSuccess, isProcessing } = attempt;

  const onOk = () => {
    attemptActions.start();
    onReset(userId)
      .done(userToken => {
        setResetLink(userToken.url);
        attemptActions.stop();
      })
      .fail(err => {
        attemptActions.error(err);
      });
  };

  return (
    <Dialog
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <DialogContent minHeight="200px" width="450PX">
        {attempt.isFailed && (
          <Alerts.Danger children={attempt.message} />
        )}
        {renderContent(userId, isSuccess, resetLink)}
      </DialogContent>
      <DialogFooter>
        {renderButtons(isProcessing, isSuccess, onOk, onClose)}
      </DialogFooter>
    </Dialog>
  );
}

UserResetDialog.propTypes = {
  attempt: PropTypes.object.isRequired,
  attemptActions: PropTypes.object.isRequired,
  onClose: PropTypes.func.isRequired,
  onReset: PropTypes.func.isRequired,
  resetLink: PropTypes.string,
  setResetLink: PropTypes.func.isRequired,
  userId: PropTypes.string.isRequired,
}

function renderContent(userId, isSuccess, resetLink){
  if (isSuccess) {
    return <CmdText cmd={resetLink} />
  }

  return (
    <div>
      <Text typography="h2">Reset User Password?</Text>
      <Text typography="paragraph" mt="2">
        You are about to reset the user {userId} password.
        This will generate a new invitation URL.
        Share it with a user so they can select a new password.
      </Text>
    </div>
  )
}

function renderButtons(isProcessing, isSuccess, onOk, onClose){
  if(isSuccess) {
    return (
      <ButtonSecondary onClick={onClose}>
        Close
      </ButtonSecondary>
    )
  }

  return (
    <>
      <ButtonPrimary mr="3" disabled={isProcessing} onClick={onOk}>
        Generate Reset URL
      </ButtonPrimary>
      <ButtonSecondary disabled={isProcessing} onClick={onClose}>
        Cancel
      </ButtonSecondary>
    </>
  )
}

function mapState(){
  const [ attempt, attemptActions ] = useAttempt();
  const [ resetLink, setResetLink ] = useState();
  return {
    onReset: actions.resetUser,
    attempt,
    attemptActions,
    resetLink,
    setResetLink
  }
}

export default withState(mapState)(UserResetDialog);