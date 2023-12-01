/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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

import { ResetToken } from 'teleport/services/user';

import UserTokenLink from './../UserTokenLink';

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
        <DialogTitle>Reset User Authentication?</DialogTitle>
      </DialogHeader>
      <DialogContent>
        {attempt.status === 'failed' && (
          <Alert kind="danger" children={attempt.statusText} />
        )}
        <Text mb={4} mt={1}>
          You are about to reset authentication for user
          <Text bold as="span">
            {` ${username} `}
          </Text>
          . This will generate a temporary URL which can be used to set up new
          authentication.
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
