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

import { Text, ButtonIcon, ButtonWarning } from 'design';
import DialogConfirmation, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/DialogConfirmation';
import { Cross } from 'design/Icon';

const updateSelectedResourceWarning =
  'Resource Access Request cannot be combined with Role Access Request. The current items will be cleared. Are you sure you want to continue?';

export default function ChangeResourceDialog({
  toResource,
  onClose,
  onConfirm,
}: Props) {
  return (
    <DialogConfirmation
      open={!!toResource}
      onClose={onClose}
      dialogCss={() => ({
        maxWidth: '400px',
        width: '100%',
      })}
    >
      <DialogHeader justifyContent="space-between" mb={0}>
        <Text typography="h5" bold style={{ whiteSpace: 'nowrap' }}>
          Remove selected resources?
        </Text>
        <ButtonIcon onClick={onClose} color="text.slightlyMuted">
          <Cross size="small" />
        </ButtonIcon>
      </DialogHeader>
      <DialogContent mb={4}>
        <Text color="text.slightlyMuted" typography="body1">
          {updateSelectedResourceWarning}
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning
          size="large"
          block={true}
          onClick={() => onConfirm(toResource)}
        >
          Confirm
        </ButtonWarning>
      </DialogFooter>
    </DialogConfirmation>
  );
}

type Props = {
  toResource: string;
  onClose: () => void;
  onConfirm: (resource: string) => void;
};
