/*
Copyright 2019-2021 Gravitational, Inc.

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
import { ButtonSecondary, ButtonWarning, Text, Alert } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/DialogConfirmation';
import useAttempt from 'shared/hooks/useAttemptNext';

import { State as ResourceState } from 'teleport/components/useResources';

export default function DeleteRoleDialog(props: Props) {
  const { name, onClose, onDelete } = props;
  const { attempt, run } = useAttempt();
  const isDisabled = attempt.status === 'processing';

  function onOk() {
    run(() => onDelete()).then(ok => ok && onClose());
  }

  return (
    <Dialog disableEscapeKeyDown={false} onClose={onClose} open={true}>
      <DialogHeader>
        <DialogTitle>Remove Role?</DialogTitle>
      </DialogHeader>
      <DialogContent width="450px">
        {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
        <Text typography="paragraph" mb="6">
          Are you sure you want to delete role{' '}
          <Text as="span" bold color="text.contrast">
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

type Props = {
  name: string;
  onClose: ResourceState['disregard'];
  onDelete(): Promise<any>;
};
