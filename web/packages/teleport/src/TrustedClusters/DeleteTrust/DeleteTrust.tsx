/*
Copyright 2020 Gravitational, Inc.

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
import * as Alerts from 'design/Alert';
import { useAttempt } from 'shared/hooks';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/DialogConfirmation';

export default function DeleteTrustedClusterDialog(props: Props) {
  const { name, onClose, onDelete } = props;
  const [attempt, attempActions] = useAttempt({ isProcessing: false });
  const isDisabled = attempt.isProcessing;

  function onOk() {
    attempActions.do(() => onDelete()).then(() => onClose());
  }

  return (
    <Dialog disableEscapeKeyDown={false} onClose={onClose} open={true}>
      <DialogHeader>
        <DialogTitle>Remove Trusted Cluster?</DialogTitle>
      </DialogHeader>
      <DialogContent width="540px">
        {attempt.isFailed && <Alerts.Danger>{attempt.message}</Alerts.Danger>}
        <Text typography="paragraph" mb="6">
          Are you sure you want to delete trusted cluster{' '}
          <Text as="span" bold color="text.contrast">
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
    </Dialog>
  );
}

type Props = {
  onClose: () => void;
  onDelete: () => Promise<any>;
  name: string;
};
