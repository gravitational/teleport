/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
