/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { Flex, Text, ButtonPrimary } from 'design';
import { Danger } from 'design/Alert';
import { Attempt } from 'shared/hooks/useAsync';

import { assertUnreachable } from 'teleterm/ui/utils';

import type * as types from 'teleterm/ui/services/workspacesService';

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
      <Text typography="h5" color="text.main">
        {message}
      </Text>
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
