/**
 * Copyright 2020 Gravitational, Inc.
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
import { ButtonWarning, ButtonSecondary, Text, Alert } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';
import { useAttemptNext } from 'shared/hooks';

export default function Container(props: Props) {
  const dialog = useDialog(props);
  return <UserDelete {...dialog} />;
}

export function UserDelete({
  username,
  onDelete,
  onClose,
  attempt,
}: ReturnType<typeof useDialog>) {
  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '500px', width: '100%' })}
      disableEscapeKeyDown={false}
      onClose={close}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>Delete User?</DialogTitle>
      </DialogHeader>
      <DialogContent>
        {attempt.status === 'failed' && (
          <Alert kind="danger" children={attempt.statusText} />
        )}
        <Text mb={4} mt={1}>
          You are about to delete user
          <Text bold as="span">
            {` ${username}`}
          </Text>
          . This will revoke the user's access to this cluster.
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning
          mr="3"
          disabled={attempt.status === 'processing'}
          onClick={onDelete}
        >
          I understand, delete user
        </ButtonWarning>
        <ButtonSecondary onClick={onClose}>Cancel</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

function useDialog(props: Props) {
  const { attempt, setAttempt } = useAttemptNext();
  function onDelete() {
    setAttempt({ status: 'processing' });
    props
      .onDelete(props.username)
      .then(() => {
        setAttempt({ status: 'success' });
        props.onClose();
      })
      .catch((err: Error) => {
        setAttempt({ status: 'failed', statusText: err.message });
      });
  }

  return {
    username: props.username,
    onClose: props.onClose,
    onDelete,
    attempt,
  };
}

type Props = {
  username: string;
  onClose(): void;
  onDelete(username: string): Promise<any>;
};
