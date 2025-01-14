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

import { Alert, ButtonSecondary, ButtonWarning, Text } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
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
        {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
        <Text mb={4}>
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
