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

import { format } from 'date-fns/format';
import { parseISO } from 'date-fns/parseISO';
import { MouseEventHandler } from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';

import { Alert } from 'design/Alert/Alert';
import {
  ButtonPrimaryBorder,
  ButtonSecondary,
  ButtonWarning,
} from 'design/Button/Button';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';
import Flex from 'design/Flex';
import Text from 'design/Text';

import cfg from 'teleport/config';
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
  /**
   * For testing only: called when the user wants to go to locks.
   */
  onGoToLocksForTesting?: MouseEventHandler<HTMLAnchorElement>;
}) {
  const {
    targetKind,
    targetName,
    onCancel,
    onComplete,
    onGoToLocksForTesting,
  } = props;

  const { isLoading, locks, canUnlock, unlock, unlockPending, unlockError } =
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

  const multipleLocksExist = locks && locks.length > 1;

  return (
    <Dialog onClose={onCancel} open={true}>
      <DialogHeader>
        <DialogTitle>Unlock {targetName}?</DialogTitle>
      </DialogHeader>
      <DialogContent maxWidth="650px">
        {multipleLocksExist ? (
          <div>
            Multiple locks exist on <strong>{targetName}</strong> and they can
            only be removed from the{' '}
            <Link to={cfg.getLocksRoute()} onClick={onGoToLocksForTesting}>
              Session and Identity Locks
            </Link>{' '}
            page.
          </div>
        ) : (
          <div>
            This will remove the restrictions placed on{' '}
            <code>{targetName}</code> and resume its activity.
          </div>
        )}

        {locks ? (
          <>
            <LocksTitle>Lock details:</LocksTitle>

            <LocksContainer>
              {locks.map(lock => (
                <LockItem key={lock.name}>
                  <Text>
                    <strong>Reason</strong>: {lock.message || 'none'}
                  </Text>
                  <Text>
                    <strong>Locked on</strong>:{' '}
                    {lock.createdAt
                      ? format(parseISO(lock.createdAt), 'PP, p z')
                      : 'unknown'}
                  </Text>
                  <Text>
                    <strong>Expires</strong>:{' '}
                    {lock.expires
                      ? format(parseISO(lock.expires), 'PP, p z')
                      : 'never'}
                  </Text>
                </LockItem>
              ))}
            </LocksContainer>
          </>
        ) : undefined}

        {unlockError ? (
          <Alert kind="danger" details={unlockError.message} mt={3} mb={0}>
            Failed to unlock <code>{targetName}</code>
          </Alert>
        ) : undefined}
      </DialogContent>
      <DialogFooter>
        <Flex gap={3}>
          {multipleLocksExist ? (
            <ButtonPrimaryBorder
              as="a"
              href={cfg.getLocksRoute()}
              onClick={onGoToLocksForTesting}
            >
              Go to Session and Identity Locks
            </ButtonPrimaryBorder>
          ) : (
            <ButtonWarning
              onClick={handleUnlock}
              disabled={isLoading || !canUnlock || unlockPending}
            >
              Remove Lock
            </ButtonWarning>
          )}
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

const LocksTitle = styled(Text)`
  font-weight: bold;
  padding: ${({ theme }) => theme.space[3]}px 0;
`;

const LocksContainer = styled(Flex)`
  flex-direction: column;
  gap: ${props => props.theme.space[3]}px;
`;

const LockItem = styled(Flex)`
  flex-direction: column;
  gap: ${props => props.theme.space[2]}px;
  padding: ${({ theme }) => theme.space[2]}px;
  background-color: ${({ theme }) => theme.colors.interactive.tonal.neutral[0]};
  border-left: 4px solid
    ${({ theme }) => theme.colors.interactive.tonal.neutral[2]};
  color: ${({ theme }) => theme.colors.text.slightlyMuted};

  & strong {
    font-weight: bold;
  }
`;
