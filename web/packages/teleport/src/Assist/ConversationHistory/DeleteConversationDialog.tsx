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

import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';

import { ButtonPrimary, ButtonSecondary } from 'design';
import { Danger } from 'design/Alert';

interface DeleteConversationDialogProps {
  conversationTitle: string;
  onDelete: () => void;
  onClose: () => void;
  error?: string;
  disabled: boolean;
}

export function DeleteConversationDialog(props: DeleteConversationDialogProps) {
  // Remove the ending period from the conversation title to avoid double periods
  const conversationTitle = props.conversationTitle.endsWith('.')
    ? props.conversationTitle.slice(0, -1)
    : props.conversationTitle;

  return (
    <Dialog open={true}>
      <DialogHeader>
        <DialogTitle>Are you sure?</DialogTitle>
      </DialogHeader>
      {props.error && <Danger>{props.error}</Danger>}
      <DialogContent width="400px">
        <p style={{ margin: 0 }}>
          You are about to delete the conversation{' '}
          <strong>{conversationTitle}</strong>.
        </p>
        <p style={{ marginBottom: 0 }}>
          You will not be able to access the conversation afterwards.
        </p>
      </DialogContent>
      <DialogFooter>
        <ButtonPrimary
          disabled={props.disabled}
          mr="3"
          onClick={props.onDelete}
        >
          Delete
        </ButtonPrimary>
        <ButtonSecondary disabled={props.disabled} onClick={props.onClose}>
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
