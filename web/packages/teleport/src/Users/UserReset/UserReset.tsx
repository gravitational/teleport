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
import { ButtonPrimary, ButtonSecondary, Text, Alert } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';
import { useAttemptNext } from 'shared/hooks';
import UserTokenLink from './../UserTokenLink';
import { ResetToken } from 'teleport/services/user';

export default function Container(props: Props) {
  const dialog = useDialog(props);
  return <UserReset {...dialog} />;
}

export function UserReset({
  username,
  onReset,
  onClose,
  attempt,
  token,
}: ReturnType<typeof useDialog>) {
  if (attempt.status === 'success') {
    return <UserTokenLink onClose={onClose} token={token} asInvite={false} />;
  }

  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '500px', width: '100%' })}
      disableEscapeKeyDown={false}
      onClose={close}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>Reset User Password?</DialogTitle>
      </DialogHeader>
      <DialogContent>
        {attempt.status === 'failed' && (
          <Alert kind="danger" children={attempt.statusText} />
        )}
        <Text mb={4} mt={1}>
          You are about to reset password for user
          <Text bold as="span">
            {` ${username} `}
          </Text>
          . This will generate a temporary URL which can be used to set up a new
          password.
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonPrimary
          mr="3"
          disabled={attempt.status === 'processing'}
          onClick={onReset}
        >
          Generate reset url
        </ButtonPrimary>
        <ButtonSecondary onClick={onClose}>Cancel</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

function useDialog(props: Props) {
  const { attempt, run } = useAttemptNext();
  const [token, setToken] = React.useState<ResetToken>(null);

  function onReset() {
    run(() => props.onReset(props.username).then(setToken));
  }

  return {
    username: props.username,
    onClose: props.onClose,
    token,
    onReset,
    attempt,
  };
}

type Props = {
  username: string;
  onClose(): void;
  onReset(username: string): Promise<any>;
};
