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

import { ButtonPrimary, Flex, Text } from 'design';
import { Danger } from 'design/Alert';
import { Attempt } from 'shared/hooks/useAsync';

import type * as types from 'teleterm/ui/services/workspacesService';
import { assertUnreachable } from 'teleterm/ui/utils';

export function Reconnect(props: {
  docKind: types.DocumentTerminal['kind'];
  attempt: Attempt<unknown>;
  reconnect: () => void;
}) {
  const { message, buttonText } = getReconnectCopy(props.docKind);

  return (
    <Flex
      gap={4}
      flexDirection="column"
      mx="auto"
      alignItems="center"
      mt={100}
      px="2"
    >
      <Text typography="h2">{message}</Text>
      <Flex flexDirection="column" alignItems="center" mx="auto">
        <Danger mb={3}>
          <Text
            textAlign="center"
            // pre-wrap to make sure that newlines from any errors are preserved.
            css={`
              white-space: pre-wrap;
            `}
          >
            {props.attempt.statusText}
          </Text>
        </Danger>
        <ButtonPrimary width="100px" onClick={props.reconnect}>
          {buttonText}
        </ButtonPrimary>
      </Flex>
    </Flex>
  );
}

function getReconnectCopy(docKind: types.DocumentTerminal['kind']) {
  switch (docKind) {
    case 'doc.terminal_tsh_node': {
      return {
        message: 'This SSH connection is currently offline.',
        buttonText: 'Reconnect',
      };
    }
    case 'doc.gateway_cli_client':
    case 'doc.gateway_kube':
    case 'doc.terminal_shell':
    case 'doc.terminal_tsh_kube': {
      return {
        message: 'Ran into an error when starting the terminal session.',
        buttonText: 'Retry',
      };
    }
    default:
      assertUnreachable(docKind);
  }
}
