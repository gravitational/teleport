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

import { Alert, ButtonSecondary, ButtonWarning, P1, Text } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';
import useAttempt from 'shared/hooks/useAttemptNext';

type Props = {
  close(): void;
  remove(): Promise<void>;
  name: string;
};

export function DeleteIntegrationDialog(props: Props) {
  const { close, remove } = props;
  const { attempt, run } = useAttempt();
  const isDisabled = attempt.status === 'processing';

  function onOk() {
    run(() => remove());
  }

  return (
    <Dialog disableEscapeKeyDown={false} onClose={close} open={true}>
      <DialogHeader>
        <DialogTitle>Delete Integration?</DialogTitle>
      </DialogHeader>
      <DialogContent width="450px">
        {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
        <P1>
          Are you sure you want to delete integration{' '}
          <Text as="span" bold color="text.main">
            {props.name}
          </Text>{' '}
          ?
        </P1>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning mr="3" disabled={isDisabled} onClick={onOk}>
          Yes, Delete Integration
        </ButtonWarning>
        <ButtonSecondary disabled={isDisabled} onClick={close}>
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
