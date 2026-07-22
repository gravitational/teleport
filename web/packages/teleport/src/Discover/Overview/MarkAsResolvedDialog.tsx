/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { Alert, Text } from 'design';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';

export function MarkAsResolvedDialog(props: {
  isProcessing: boolean;
  onCancel: () => void;
  onConfirm: () => void;
  confirmDisabled?: boolean;
}) {
  const { isProcessing, onCancel, onConfirm, confirmDisabled } = props;

  return (
    <Dialog onClose={onCancel} open={true}>
      <DialogHeader>
        <DialogTitle>Mark as Resolved</DialogTitle>
      </DialogHeader>
      <DialogContent width="450px">
        <Alert kind="warning" mb={3}>
          These issues will reappear if the underlying problems are not fixed.
        </Alert>
        <Text typography="body3">
          Teleport will automatically retry enrolling these resources in the
          next discovery scan.
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonPrimary
          mr="3"
          disabled={isProcessing || confirmDisabled}
          onClick={onConfirm}
        >
          Mark as Resolved
        </ButtonPrimary>
        <ButtonSecondary disabled={isProcessing} onClick={onCancel}>
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
