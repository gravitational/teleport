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

import { useState } from 'react';

import { Alert } from 'design/Alert/Alert';
import { ButtonSecondary, ButtonWarning } from 'design/Button/Button';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';
import Flex from 'design/Flex/Flex';
import Text from 'design/Text/Text';
import FieldInput from 'shared/components/FieldInput/FieldInput';
import { FieldTextArea } from 'shared/components/FieldTextArea/FieldTextArea';
import { Validation } from 'shared/components/Validation/Validation';

import { LockResourceKind } from 'teleport/LocksV2/NewLock/common';
import { Lock } from 'teleport/services/locks';

import { useResourceLock } from './useResourceLock';

/**
 * A confirmation dialog component for locking a resource. It uses
 * `useResourceLock` internally to load existing lock state, check permissions
 * and perform the lock operation. An optional message and/or TTL is captured.
 * Conditional rendering is used to show and hide the dialog.
 *
 * Example usage:
 * ```tsx
 * const [showLockDialog, setShowLockDialog] = useState(false);
 * {showLockDialog ? (
 *  <ResourceLockDialog
 *    targetKind={'user'}
 *    targetName={'example-user'}
 *    onCancel={() => setShowLockDialog(false)}
 *    onComplete={() => setShowLockDialog(false)}
 *  />
 * ) : undefined}
 * ```
 *
 * @param props
 * @returns
 */
export function ResourceLockDialog(props: {
  /** The kind of resource to lock. */
  targetKind: LockResourceKind;
  /** The name of the resource to lock. */
  targetName: string;
  /**
   * Called when the user cancels the lock operation.
   */
  onCancel: () => void;
  /**
   * Called when the user completes the lock operation.
   * @param newLock the newly created lock
   */
  onComplete: (newLock: Lock) => void;
}) {
  const { targetKind, targetName, onCancel, onComplete } = props;

  const [message, setMessage] = useState('');
  const [ttl, setTtl] = useState('');

  const { isLoading, canLock, lock, lockPending, lockError } = useResourceLock({
    targetKind,
    targetName,
  });

  const handleLock = async () => {
    let newLock: Lock | undefined = undefined;
    try {
      newLock = await lock(message, ttl);
    } catch {
      // Swallow this error - it's handled as `lockError` above
      return;
    }
    onComplete(newLock);
  };

  return (
    <Dialog onClose={onCancel} open={true}>
      <DialogHeader>
        <DialogTitle>Lock {targetName}?</DialogTitle>
      </DialogHeader>
      <Validation>
        {({ validator }) => (
          <form
            onSubmit={e => {
              e.preventDefault();
              if (validator.validate()) {
                handleLock();
              }
            }}
          >
            <DialogContent maxWidth="650px">
              <Text mb={3}>
                Locking a resource will terminate all of its connections and
                reject any new API requests.
              </Text>
              <FieldTextArea
                label="Reason"
                placeholder="Going down for maintenance"
                value={message}
                readonly={isLoading || lockPending}
                onChange={e => setMessage(e.target.value)}
              />
              <FieldInput
                label="Expiry"
                value={ttl}
                readonly={isLoading || lockPending}
                onChange={e => setTtl(e.target.value)}
                helperText={
                  'A duration string such as 12h, 2h 45m, 43200s. Valid time units are h, m and s.'
                }
              />
              {lockError ? (
                <Alert kind="danger" details={lockError.message} mt={3} mb={0}>
                  Failed to lock <code>{targetName}</code>
                </Alert>
              ) : undefined}
            </DialogContent>
          </form>
        )}
      </Validation>
      <DialogFooter>
        <Flex gap={3}>
          <ButtonWarning
            onClick={handleLock}
            disabled={isLoading || !canLock || lockPending}
          >
            Create Lock
          </ButtonWarning>
          <ButtonSecondary
            disabled={isLoading || lockPending}
            onClick={onCancel}
          >
            Cancel
          </ButtonSecondary>
        </Flex>
      </DialogFooter>
    </Dialog>
  );
}
