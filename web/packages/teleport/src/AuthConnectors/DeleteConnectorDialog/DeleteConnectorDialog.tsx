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

import React from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import { ButtonWarning, ButtonSecondary, Text, Alert, P1 } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';

import { State as ResourceState } from 'teleport/components/useResources';

export default function DeleteConnectorDialog(props: Props) {
  const { name, onClose, onDelete } = props;
  const { attempt, run } = useAttempt();
  const isDisabled = attempt.status === 'processing';

  function onOk() {
    run(() => onDelete()).then(ok => ok && onClose());
  }

  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '500px', width: '100%' })}
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>Remove Connector?</DialogTitle>
      </DialogHeader>
      <DialogContent>
        {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
        <P1>
          Are you sure you want to delete connector{' '}
          <Text as="span" bold color="text.main">
            {name}
          </Text>
          ?
        </P1>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning mr="3" disabled={isDisabled} onClick={onOk}>
          Yes, Remove Connector
        </ButtonWarning>
        <ButtonSecondary disabled={isDisabled} onClick={onClose}>
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

type Props = {
  name: string;
  onClose: ResourceState['disregard'];
  onDelete(): Promise<any>;
};
