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

import { Alert } from 'design/Alert/Alert';
import { ButtonSecondary, ButtonWarning } from 'design/Button/Button';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';
import Flex from 'design/Flex/Flex';

import { LockResourceKind } from 'teleport/LocksV2/NewLock/common';

import { useResourceLock } from './useResourceLock';

/**
 * A confirmation dialog component for unlocking a resource. It uses
 * `useResourceLock` internally to load existing lock state, check permissions
 * and perform the unlock operation. Conditional rendering is used to show and
 * hide the dialog.
 *
 * Example usage:
 * ```tsx
 * const [showUnlockDialog, setShowUnlockDialog] = useState(false);
 * {showUnlockDialog ? (
 *  <ResourceUnlockDialog
 *    targetKind={'user'}
 *    targetName={'example-user'}
 *    onCancel={() => setShowUnlockDialog(false)}
 *    onComplete={() => setShowUnlockDialog(false)}
 *  />
 * ) : undefined}
 * ```
 *
 * @param props
 * @returns
 */
export function ResourceUnlockDialog(props: {
  /** The kind of resource to unlock. */
  targetKind: LockResourceKind;
  /** The name of the resource to unlock. */
  targetName: string;
  /**
   * Called when the user cancels the unlock operation.
   */
  onCancel: () => void;
  /**
   * Called when the user completes the unlock operation.
   */
  onComplete: () => void;
}) {
  const { targetKind, targetName, onCancel, onComplete } = props;

  const { isLoading, canUnlock, unlock, unlockPending, unlockError } =
    useResourceLock({
      targetKind,
      targetName,
    });

  const handleUnlock = async () => {
    try {
      await unlock();
    } catch {
      // Swallow this error - it's handled as `unlockError` above
      return;
    }
    onComplete();
  };

  return (
    <Dialog onClose={onCancel} open={true}>
      <DialogHeader>
        <DialogTitle>Unlock {targetName}?</DialogTitle>
      </DialogHeader>
      <DialogContent>
        <div>
          This will remove the restrictions placed on <code>{targetName}</code>{' '}
          and resume its activity.
        </div>
        {unlockError ? (
          <Alert kind="danger" details={unlockError.message} mt={3} mb={0}>
            Failed to unlock <code>{targetName}</code>
          </Alert>
        ) : undefined}
      </DialogContent>
      <DialogFooter>
        <Flex gap={3}>
          <ButtonWarning
            onClick={handleUnlock}
            disabled={isLoading || !canUnlock || unlockPending}
          >
            Remove Lock
          </ButtonWarning>
          <ButtonSecondary
            disabled={isLoading || unlockPending}
            onClick={onCancel}
          >
            Cancel
          </ButtonSecondary>
        </Flex>
      </DialogFooter>
    </Dialog>
  );
}
