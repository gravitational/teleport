/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { ButtonSecondary, ButtonWarning, Text, Flex } from 'design';
import { Danger } from 'design/Alert';
import useAttempt from 'shared/hooks/useAttemptNext';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/DialogConfirmation';

import { Lock } from 'teleport/services/locks';

import { Pills } from './Locks';

type Props = {
  onClose(): void;
  onDelete(lockName: string): Promise<void>;
  lock: Lock;
};

export function DeleteLockDialogue(props: Props) {
  const { lock, onClose, onDelete } = props;
  const { attempt, run } = useAttempt('');
  const isDisabled = attempt.status === 'processing';

  function onOk() {
    run(() => onDelete(lock.name));
  }

  return (
    <Dialog disableEscapeKeyDown={false} onClose={onClose} open={true}>
      <DialogHeader>
        <DialogTitle>Delete Lock</DialogTitle>
      </DialogHeader>
      <DialogContent width="540px">
        {attempt.status === 'failed' && <Danger>{attempt.statusText}</Danger>}
        <Flex alignItems="center" flexWrap="wrap">
          <Text typography="paragraph" as="span" mr={1}>
            Are you sure you want to delete lock for{' '}
          </Text>
          <Pills targets={lock.targets} />?
        </Flex>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning mr="3" disabled={isDisabled} onClick={onOk}>
          Yes, Delete Lock
        </ButtonWarning>
        <ButtonSecondary disabled={isDisabled} onClick={onClose}>
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
