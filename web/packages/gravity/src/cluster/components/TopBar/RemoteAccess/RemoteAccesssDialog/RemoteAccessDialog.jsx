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
import { Box, ButtonSecondary, ButtonPrimary, Text } from 'design';
import * as Alerts from 'design/Alert';
import { useAttempt, withState } from 'shared/hooks';
import Dialog, { DialogHeader, DialogTitle, DialogContent, DialogFooter} from 'design/DialogConfirmation';

export function RemoteAccessDialog(props){
  const { enabled, onClose, onConfirmed, attempt, attemptActions } = props;
  const onOk = () => {
    attemptActions.do(() => onConfirmed())
      .then(() => onClose());
  };

  return (
    <Dialog
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <Box maxWidth="500px">
        <DialogHeader>
          <DialogTitle> Remote Assistance </DialogTitle>
        </DialogHeader>
        {!enabled && renderEnabled(attempt, onOk, onClose)}
        {enabled && renderDisabled(attempt, onOk, onClose)}
      </Box>
    </Dialog>
  );
}

function renderEnabled(attempt, onOk, onClose){
  const isDisabled = attempt.isProcessing;
  return (
    <>
      <DialogContent>
        {attempt.isFailed && (
          <Alerts.Danger children={attempt.message} />
        )}
        <Text typography="paragraph" color="primary.contrastText">
          Are you sure you want enable remote assistance?
          <Text>Enabling remote assistance will allow vendor team to support your infrastructure.</Text>
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonPrimary mr="3" disabled={isDisabled} onClick={onOk}>
          Enable
        </ButtonPrimary>
        <ButtonSecondary disabled={isDisabled} onClick={onClose}>
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </>
  )
}

function renderDisabled(attempt, onOk, onClose){
  const isDisabled = attempt.isProcessing;
  return (
    <>
      <DialogContent>
        {attempt.isFailed && (
          <Alerts.Danger children={attempt.message} />
        )}
        <Text typography="paragraph" color="primary.contrastText">
          Are you sure you want disable remote assistance?
          <Text>Disabling remote assistance will turn off remote access to your infrastructure for vendor support team.</Text>
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonPrimary mr="3" disabled={isDisabled} onClick={onOk}>
          Disable
        </ButtonPrimary>
        <ButtonSecondary disabled={isDisabled} onClick={onClose}>
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </>
  )
}

RemoteAccessDialog.propTypes = {
  onClose: PropTypes.func.isRequired,
  onConfirmed: PropTypes.func.isRequired,
  enabled: PropTypes.bool.isRequired,
}

function mapState(){
  const [ attempt, attemptActions ] = useAttempt();
  return {
    attempt,
    attemptActions
  }
}

export default withState(mapState)(RemoteAccessDialog)