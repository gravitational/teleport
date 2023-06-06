/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import DialogConfirmation, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/DialogConfirmation';
import { ButtonIcon, ButtonPrimary, ButtonSecondary, Text } from 'design';
import { Close } from 'design/Icon';

interface DocumentsReopenProps {
  onCancel(): void;

  onConfirm(): void;
}

export function DocumentsReopen(props: DocumentsReopenProps) {
  return (
    <DialogConfirmation
      open={true}
      onClose={props.onCancel}
      dialogCss={() => ({
        maxWidth: '400px',
        width: '100%',
      })}
    >
      <form
        onSubmit={e => {
          e.preventDefault();
          props.onConfirm();
        }}
      >
        <DialogHeader
          justifyContent="space-between"
          mb={0}
          alignItems="baseline"
        >
          <Text typography="h4" bold>
            Reopen previous session
          </Text>
          <ButtonIcon
            type="button"
            onClick={props.onCancel}
            color="text.slightlyMuted"
          >
            <Close fontSize={5} />
          </ButtonIcon>
        </DialogHeader>
        <DialogContent mb={4}>
          <Text typography="body1" color="text.slightlyMuted">
            Do you want to reopen tabs from the previous session?
          </Text>
        </DialogContent>
        <DialogFooter>
          <ButtonPrimary autoFocus mr={3} type="submit">
            Reopen
          </ButtonPrimary>
          <ButtonSecondary type="button" onClick={props.onCancel}>
            Start new session
          </ButtonSecondary>
        </DialogFooter>
      </form>
    </DialogConfirmation>
  );
}
