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

import React from 'react';
import { ButtonWarning, ButtonSecondary, Flex, Text, Alert } from 'design';
import TextSelectCopy from 'teleport/components/TextSelectCopy';

import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';

import { Attempt } from 'shared/hooks/useAsync';

import RolesRequested from '../RolesRequested';

import type { RequestState } from 'shared/services/accessRequests';

export interface RequestDeleteProps {
  requestId: string;
  requestState: RequestState;
  user: string;
  roles: string[];
  onClose(): void;
  deleteRequestAttempt: Attempt<void>;
  onDelete(): void;
}

export function RequestDelete({
  deleteRequestAttempt,
  user,
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
          <Text typography="body2">
            You are about to delete a request from <strong>{user}</strong> for
            the following roles:
          </Text>
          <RolesRequested roles={roles} />
        </Flex>
        {requestState === 'APPROVED' && (
          <>
            <Text mt={3} mb={2} typography="body2">
              Since this access request has already been approved, deleting the
              request now will NOT remove the user's access to these roles. If
              you would like to lock the user's access to the requested roles,
              you can run:
            </Text>
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
