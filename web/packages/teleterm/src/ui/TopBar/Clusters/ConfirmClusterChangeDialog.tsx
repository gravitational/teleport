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

import { Text, ButtonIcon, ButtonWarning } from 'design';
import DialogConfirmation, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/DialogConfirmation';
import { Close } from 'design/Icon';

const changeSelectedClusterWarning =
  'Resources from different clusters cannot be combined in an access request. Current items selected will be cleared. Are you sure you want to continue?';

export default function ConfirmClusterChangeDialog({
  confirmChangeTo,
  onClose,
  onConfirm,
}: Props) {
  return (
    <DialogConfirmation
      open={!!confirmChangeTo}
      onClose={onClose}
      dialogCss={() => ({
        maxWidth: '400px',
        width: '100%',
      })}
    >
      <DialogHeader justifyContent="space-between" mb={0}>
        <Text typography="h5" bold style={{ whiteSpace: 'nowrap' }}>
          Change clusters?
        </Text>
        <ButtonIcon onClick={onClose} color="text.slightlyMuted">
          <Close fontSize={5} />
        </ButtonIcon>
      </DialogHeader>
      <DialogContent mb={4}>
        <Text color="text.slightlyMuted" typography="body1">
          {changeSelectedClusterWarning}
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning size="large" block={true} onClick={() => onConfirm()}>
          Confirm
        </ButtonWarning>
      </DialogFooter>
    </DialogConfirmation>
  );
}

type Props = {
  confirmChangeTo: string;
  onClose: () => void;
  onConfirm: () => void;
};
