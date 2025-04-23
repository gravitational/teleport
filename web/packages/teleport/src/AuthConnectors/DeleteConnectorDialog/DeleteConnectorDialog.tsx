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

import {
  Alert,
  Box,
  ButtonSecondary,
  ButtonWarning,
  Flex,
  P1,
  Text,
} from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';
import useAttempt from 'shared/hooks/useAttemptNext';

import { State as ResourceState } from 'teleport/components/useResources';
import { KindAuthConnectors } from 'teleport/services/resources';

import getSsoIcon from '../ssoIcons/getSsoIcon';

export default function DeleteConnectorDialog(props: Props) {
  const { name, kind, onClose, onDelete, isDefault, nextDefault } = props;
  const { attempt, run } = useAttempt();
  const isDisabled = attempt.status === 'processing';

  function onOk() {
    run(() => onDelete()).then(ok => ok && onClose());
  }

  const Icon = getSsoIcon(kind, name);

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
      <DialogContent mb={4}>
        {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
        <Flex gap={3} width="100%">
          <Box>
            <Icon />
          </Box>
          <P1>
            Are you sure you want to delete connector{' '}
            <Text as="span" bold color="text.main">
              {name}
            </Text>
            ?
          </P1>
        </Flex>
        {isDefault && (
          <Alert kind="outline-warn" m={0} mt={3}>
            <P1>
              This is currently the default auth connector. Deleting this will
              cause{' '}
              <Text as="span" bold color="text.main">
                {nextDefault}
              </Text>{' '}
              to become the new default.
            </P1>
          </Alert>
        )}
      </DialogContent>
      <DialogFooter>
        <ButtonWarning mr="3" disabled={isDisabled} onClick={onOk}>
          Delete Connector
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
  kind: KindAuthConnectors;
  onClose: ResourceState['disregard'];
  onDelete(): Promise<any>;
  isDefault: boolean;
  nextDefault: string;
};
