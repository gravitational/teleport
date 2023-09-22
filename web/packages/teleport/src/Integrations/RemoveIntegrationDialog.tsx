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
import { ButtonSecondary, ButtonWarning, Text, Alert } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/DialogConfirmation';
import useAttempt from 'shared/hooks/useAttemptNext';

type Props = {
  close(): void;
  remove(): Promise<void>;
  name: string;
};

export function DeleteIntegrationDialog(props: Props) {
  const { close, remove } = props;
  const { attempt, run } = useAttempt();
  const isDisabled = attempt.status === 'processing';

  function onOk() {
    run(() => remove());
  }

  return (
    <Dialog disableEscapeKeyDown={false} onClose={close} open={true}>
      <DialogHeader>
        <DialogTitle>Delete Integration?</DialogTitle>
      </DialogHeader>
      <DialogContent width="450px">
        {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
        <Text typography="paragraph" mb="6">
          Are you sure you want to delete integration{' '}
          <Text as="span" bold color="text.contrast">
            {props.name}
          </Text>{' '}
          ?
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning mr="3" disabled={isDisabled} onClick={onOk}>
          Yes, Delete Integration
        </ButtonWarning>
        <ButtonSecondary disabled={isDisabled} onClick={close}>
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
