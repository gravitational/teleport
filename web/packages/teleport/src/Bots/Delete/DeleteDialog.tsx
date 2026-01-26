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

import { useMutation } from '@tanstack/react-query';

import { Alert } from 'design/Alert/Alert';
import {
  ButtonSecondary,
  ButtonWarning,
  ButtonWarningBorder,
} from 'design/Button/Button';
import { Dialog } from 'design/Dialog/Dialog';
import DialogContent from 'design/Dialog/DialogContent';
import DialogFooter from 'design/Dialog/DialogFooter';
import DialogHeader from 'design/Dialog/DialogHeader';
import DialogTitle from 'design/Dialog/DialogTitle';
import Flex from 'design/Flex';
import { P } from 'design/Text/Text';
import { wait } from 'shared/utils/wait';

import { deleteBot } from 'teleport/services/bot/bot';

export function DeleteDialog(props: {
  botName: string;
  /**
   * Called when the user cancels the delete operation.
   */
  onCancel: () => void;
  /**
   * Called when the user completes the delete operation.
   */
  onComplete: () => void;
  showLockAlternative?: boolean;
  /**
   * Called when the user requests to lock the bot instead of deleting it.
   */
  onLockRequest?: () => void;
  canLockBot?: boolean;
}) {
  const {
    botName,
    onCancel,
    onComplete,
    showLockAlternative = true,
    onLockRequest,
    canLockBot,
  } = props;

  const { mutateAsync, isPending, error } = useMutation({
    async mutationFn(variables: Parameters<typeof deleteBot>[0]) {
      const result = await deleteBot(variables);

      // Adds a delay to allow the delete event to propagate to the backend
      // cache before indicating that the operation is complete. In case a
      // refresh is triggered immediately.
      //
      // TODO(nicholasmarais1158): Use Tanstack Query to fetch the bot list,
      // then update the query's cache here to avoid needing this delay.
      await wait(1000);

      return result;
    },
  });

  const handleDelete = async () => {
    try {
      await mutateAsync({ botName });
    } catch {
      // Swallow this error - it's handled as `error` above
      return;
    }
    onComplete();
  };

  return (
    <Dialog onClose={onCancel} open={true}>
      <DialogHeader>
        <DialogTitle>Delete {botName}?</DialogTitle>
      </DialogHeader>
      <DialogContent maxWidth={540}>
        <div>
          <P>
            Deleting a bot is permanent and cannot be undone. Bot instances
            remain active until their issued credentials expire.
            {showLockAlternative
              ? ''
              : ' To terminate active instances immediately, lock the bot before deleting it.'}
          </P>
          {showLockAlternative ? (
            <P>
              Alternatively, you can lock a bot to stop all of its activity
              immediately.
            </P>
          ) : undefined}
        </div>
        {error ? (
          <Alert kind="danger" details={error.message} mt={3} mb={0}>
            Failed to delete <code>{botName}</code>
          </Alert>
        ) : undefined}
      </DialogContent>
      <DialogFooter>
        <Flex gap={3}>
          <ButtonWarning disabled={isPending} onClick={handleDelete}>
            Delete Bot
          </ButtonWarning>
          <ButtonSecondary disabled={isPending} onClick={onCancel} mr="auto">
            Cancel
          </ButtonSecondary>
          {showLockAlternative ? (
            <ButtonWarningBorder
              disabled={!canLockBot || isPending}
              onClick={onLockRequest}
            >
              Lock Bot
            </ButtonWarningBorder>
          ) : undefined}
        </Flex>
      </DialogFooter>
    </Dialog>
  );
}
