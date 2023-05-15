/*
Copyright 2021 Gravitational, Inc.

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

import React from 'react';
import { ButtonSecondary, ButtonWarning, Text } from 'design';
import { Danger } from 'design/Alert';
import Dialog, { DialogContent, DialogFooter } from 'design/DialogConfirmation';
import useAttempt from 'shared/hooks/useAttemptNext';

export default function RemoveDialog(props: Props) {
  const { name, onClose, onRemove } = props;
  const { attempt, handleError, setAttempt } = useAttempt('');

  function onConfirm() {
    setAttempt({ status: 'processing' });
    onRemove().catch(handleError);
  }

  return (
    <Dialog disableEscapeKeyDown={false} onClose={onClose} open={true}>
      <DialogContent width="400px">
        <Text typography="h2" mb={2}>
          Remove Device
        </Text>
        {attempt.status == 'failed' && (
          <Danger mb={2}>{attempt.statusText}</Danger>
        )}
        <Text typography="paragraph" mb="6">
          Are you sure you want to remove device{' '}
          <Text as="span" bold color="text.main">
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
          onClick={onClose}
        >
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

type Props = {
  onClose: () => void;
  onRemove: () => Promise<any>;
  name: string;
};
