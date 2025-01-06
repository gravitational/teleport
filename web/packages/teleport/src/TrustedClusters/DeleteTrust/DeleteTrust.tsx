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

import { ButtonSecondary, ButtonWarning, P1, Text } from 'design';
import * as Alerts from 'design/Alert';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';
import useAttempt from 'shared/hooks/useAttemptNext';

export default function DeleteTrustedClusterDialog(props: Props) {
  const { name, onClose, onDelete } = props;
  const { attempt, run } = useAttempt('');
  const isDisabled = attempt.status === 'processing';

  function onOk() {
    run(() => onDelete()).then(() => onClose());
  }

  return (
    <Dialog disableEscapeKeyDown={false} onClose={onClose} open={true}>
      <DialogHeader>
        <DialogTitle>Remove Trusted Cluster?</DialogTitle>
      </DialogHeader>
      <DialogContent width="540px">
        {attempt.status === 'failed' && (
          <Alerts.Danger>{attempt.statusText}</Alerts.Danger>
        )}
        <P1>
          Are you sure you want to delete trusted cluster{' '}
          <Text as="span" bold color="text.main">
            {name}
          </Text>
          ?
        </P1>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning mr="3" disabled={isDisabled} onClick={onOk}>
          Yes, Remove Trusted Cluster
        </ButtonWarning>
        <ButtonSecondary disabled={isDisabled} onClick={onClose}>
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

type Props = {
  onClose: () => void;
  onDelete: () => Promise<any>;
  name: string;
};
