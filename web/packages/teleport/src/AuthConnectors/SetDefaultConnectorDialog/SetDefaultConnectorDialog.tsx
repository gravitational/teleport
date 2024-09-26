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

export default function SetDefaultConnectorDialog(props: Props) {
  const { connectorName, connectorType, onClose, onSetDefaultConnector } =
    props;
  const { attempt, run } = useAttempt();
  const isDisabled = attempt.status === 'processing';

  function onOk() {
    run(() => onSetDefaultConnector(connectorName, connectorType)).then(
      ok => ok && onClose()
    );
  }

  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '500px', width: '100%' })}
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>Set as default connector?</DialogTitle>
      </DialogHeader>
      <DialogContent>
        {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
        <P1>
          Do you want to set{' '}
          <Text as="span" bold color="text.main">
            {connectorName}
          </Text>{' '}
          as the default authentication method?
        </P1>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning mr="3" disabled={isDisabled} onClick={onOk}>
          Yes
        </ButtonWarning>
        <ButtonSecondary disabled={isDisabled} onClick={onClose}>
          No
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

type Props = {
  connectorName: string;
  connectorType: string;
  onClose: ResourceState['disregard'];
  onSetDefaultConnector(
    connectorName: string,
    connectorType: string
  ): Promise<any>;
};
