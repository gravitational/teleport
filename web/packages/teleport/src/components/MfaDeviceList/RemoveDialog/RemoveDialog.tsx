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

import { ButtonSecondary, ButtonWarning, H2, P1, Text } from 'design';
import { Danger } from 'design/Alert';
import Dialog, { DialogContent, DialogFooter } from 'design/DialogConfirmation';
import useAttempt from 'shared/hooks/useAttemptNext';

export default function RemoveDialog(props: Props) {
  const { name, onClose, onRemove } = props;
  const { attempt, handleError, setAttempt } = useAttempt('');

  function onConfirm() {
    setAttempt({ status: 'processing' });
    onRemove().catch(handleError);
  }

  return (
    <Dialog disableEscapeKeyDown={false} onClose={onClose} open={true}>
      <DialogContent width="400px">
        <H2 mb={4}>Remove Device</H2>
        {attempt.status == 'failed' && (
          <Danger mb={2}>{attempt.statusText}</Danger>
        )}
        <P1>
          Are you sure you want to remove device{' '}
          <Text as="span" bold color="text.main">
            {name}
          </Text>{' '}
          ?
        </P1>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning
          mr="3"
          disabled={attempt.status === 'processing'}
          onClick={onConfirm}
        >
          Remove
        </ButtonWarning>
        <ButtonSecondary
          disabled={attempt.status === 'processing'}
          onClick={onClose}
        >
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

type Props = {
  onClose: () => void;
  onRemove: () => Promise<any>;
  name: string;
};
