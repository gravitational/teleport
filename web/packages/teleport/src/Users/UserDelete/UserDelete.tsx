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

import { useMutation, useQueryClient } from '@tanstack/react-query';

import { Alert, ButtonSecondary, ButtonWarning, Text } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';

import { ResourcesResponse } from 'teleport/services/agents';
import userService, { User } from 'teleport/services/user';
import { GetUsersQueryKey } from 'teleport/services/user/hooks';

interface UserDeleteProps {
  username: string;
  onClose(): void;
  modifyFetchedData: React.Dispatch<
    React.SetStateAction<ResourcesResponse<User>>
  >;
}

export function UserDelete({
  username,
  onClose,
  modifyFetchedData,
}: UserDeleteProps) {
  const queryClient = useQueryClient();

  const deleteUser = useMutation({
    mutationFn: userService.deleteUser,
    onSuccess: (_, name) => {
      queryClient.setQueryData(GetUsersQueryKey, previous => {
        if (!previous) {
          return [];
        }

        return previous.filter(user => user.name !== name);
      });
    },
  });

  async function handleDelete() {
    await deleteUser.mutateAsync(username);

    modifyFetchedData(p => {
      p.agents = p.agents.filter(user => user.name !== username);
      return p;
    });

    onClose();
  }

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
        {deleteUser.isError && <Alert children={deleteUser.error.message} />}
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
          disabled={deleteUser.isPending}
          onClick={handleDelete}
        >
          I understand, delete user
        </ButtonWarning>
        <ButtonSecondary onClick={onClose}>Cancel</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
