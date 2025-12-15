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

import { useMemo, useRef } from 'react';

import {
  Alert,
  Box,
  ButtonSecondary,
  Flex,
  H1,
  H2,
  Link,
  Stack,
  Text,
} from 'design';
import * as Alerts from 'design/Alert';
import { Gateway } from 'gen-proto-ts/teleport/lib/teleterm/v1/gateway_pb';
import Validation from 'shared/components/Validation';
import { Attempt, RunFuncReturnValue } from 'shared/hooks/useAsync';
import { debounce } from 'shared/utils/highbar';

import { CliCommand } from '../components/CliCommand';
import { ConfigFieldInput, PortFieldInput } from '../components/FieldInputs';

export function OnlineDocumentGateway(props: {
  changeDbName: (name: string) => RunFuncReturnValue<void>;
  changeDbNameAttempt: Attempt<void>;
  changePort: (port: string) => RunFuncReturnValue<void>;
  changePortAttempt: Attempt<void>;
  disconnect: () => RunFuncReturnValue<void>;
  disconnectAttempt: Attempt<void>;
  gateway: Gateway;
  runCliCommand: () => void;
}) {
  const isPortOrDbNameProcessing =
    props.changeDbNameAttempt.status === 'processing' ||
    props.changePortAttempt.status === 'processing';
  const hasError =
    props.changeDbNameAttempt.status === 'error' ||
    props.changePortAttempt.status === 'error';
  const formRef = useRef<HTMLFormElement>(null);
  const { gateway } = props;

  const handleChangeDbName = useMemo(() => {
    return debounce((value: string) => {
      props.changeDbName(value);
    }, 150);
  }, [props.changeDbName]);

  const handleChangePort = useMemo(() => {
    return debounce((value: string) => {
      if (formRef.current.reportValidity()) {
        props.changePort(value);
      }
    }, 1000);
  }, [props.changePort]);

  const $errors = hasError && (
    <Flex flexDirection="column" gap={2} mb={3}>
      {props.changeDbNameAttempt.status === 'error' && (
        <Alerts.Danger mb={0} details={props.changeDbNameAttempt.statusText}>
          Could not change the database name
        </Alerts.Danger>
      )}
      {props.changePortAttempt.status === 'error' && (
        <Alerts.Danger mb={0} details={props.changePortAttempt.statusText}>
          Could not change the port number
        </Alerts.Danger>
      )}
    </Flex>
  );

  return (
    <Box maxWidth="680px" width="100%" mx="auto" mt="4" px="5">
      <Flex justifyContent="space-between" mb="4" flexWrap="wrap" gap={2}>
        <H1>Database Connection</H1>
        <ButtonSecondary size="small" onClick={props.disconnect}>
          Close Connection
        </ButtonSecondary>
      </Flex>

      {props.disconnectAttempt.status === 'error' && (
        <Alert details={props.disconnectAttempt.statusText}>
          Could not close the connection
        </Alert>
      )}

      <H2 mb={2}>Connect with CLI</H2>
      <Stack gap={2} alignItems="normal">
        <Flex as="form" ref={formRef}>
          <Validation>
            <PortFieldInput
              label="Port"
              defaultValue={gateway.localPort}
              onChange={e => handleChangePort(e.target.value)}
              mb={0}
            />
            <ConfigFieldInput
              label="Database Name"
              defaultValue={gateway.targetSubresourceName}
              onChange={e => handleChangeDbName(e.target.value)}
              spellCheck={false}
              ml={2}
              mb={0}
            />
          </Validation>
        </Flex>
        <CliCommand
          cliCommand={props.gateway.gatewayCliCommand.preview}
          isLoading={isPortOrDbNameProcessing}
          button={{ onClick: props.runCliCommand }}
        />
        {$errors}
      </Stack>

      <H2 mt={3} mb={2}>
        Connect with GUI
      </H2>
      <Text
        // Break long usernames rather than putting an ellipsis.
        css={`
          word-break: break-word;
        `}
      >
        Configure the GUI database client to connect to host{' '}
        <code>{gateway.localAddress}</code> on port{' '}
        <code>{gateway.localPort}</code> as user{' '}
        <code>{gateway.targetUser}</code>.
      </Text>
      <Text>
        The connection is made through an authenticated proxy so no extra
        credentials are necessary. See{' '}
        <Link
          href="https://goteleport.com/docs/connect-your-client/gui-clients/"
          target="_blank"
        >
          the documentation
        </Link>{' '}
        for more details.
      </Text>
    </Box>
  );
}
