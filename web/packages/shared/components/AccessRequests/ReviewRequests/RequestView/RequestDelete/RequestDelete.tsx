/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { Alert, ButtonSecondary, ButtonWarning, Flex } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import { P } from 'design/Text/Text';
import { TextSelectCopy } from 'shared/components/TextSelectCopy';
import { UserDisplayName } from 'shared/components/UserDisplayName';
import { Attempt } from 'shared/hooks/useAsync';
import type { RequestState, UserDisplay } from 'shared/services/accessRequests';

import RolesRequested from '../RolesRequested';

export interface RequestDeleteProps {
  requestId: string;
  requestState: RequestState;
  user: string;
  userDisplay?: UserDisplay;
  roles: string[];
  onClose(): void;
  deleteRequestAttempt: Attempt<void>;
  onDelete(): void;
}

export function RequestDelete({
  deleteRequestAttempt,
  user,
  userDisplay,
  roles,
  requestId,
  requestState,
  onClose,
  onDelete,
}: RequestDeleteProps) {
  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '550px', width: '100%' })}
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>Delete Request?</DialogTitle>
      </DialogHeader>
      <DialogContent>
        {deleteRequestAttempt.status === 'error' && (
          <Alert kind="danger" children={deleteRequestAttempt.statusText} />
        )}
        <Flex flexWrap="wrap" gap={1} alignItems="baseline">
          <P>
            You are about to delete a request from{' '}
            <strong>
              <UserDisplayName
                username={user}
                primaryText={userDisplay?.primary}
                primaryTextProps={{ style: { font: 'inherit' } }}
                layout="inline"
              />
            </strong>{' '}
            for the following roles:
          </P>
          <RolesRequested roles={roles} />
        </Flex>
        {requestState === 'APPROVED' && (
          <>
            <P mt={3} mb={2}>
              Since this access request has already been approved, deleting the
              request now will NOT remove the user's access to these roles. If
              you would like to lock the user's access to the requested roles,
              you can run:
            </P>
            <TextSelectCopy
              mt={2}
              text={`tctl lock --access-request ${requestId}`}
            />
          </>
        )}
      </DialogContent>
      <DialogFooter>
        <ButtonWarning
          mr="3"
          disabled={deleteRequestAttempt.status === 'processing'}
          onClick={onDelete}
        >
          Delete Request
        </ButtonWarning>
        <ButtonSecondary
          disabled={deleteRequestAttempt.status === 'processing'}
          onClick={onClose}
        >
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
