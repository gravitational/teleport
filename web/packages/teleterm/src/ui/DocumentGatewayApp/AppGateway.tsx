/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { Flex, Text, ButtonSecondary, Link, Box, Alert } from 'design';

import Validation from 'shared/components/Validation';
import { Attempt } from 'shared/hooks/useAsync';
import { debounce } from 'shared/utils/highbar';

import { Gateway } from 'teleterm/services/tshd/types';
import { CliCommand } from 'teleterm/ui/DocumentGateway/CliCommand';

import { PortFieldInput } from '../components/FieldInputs';

export function AppGateway(props: {
  gateway: Gateway;
  disconnectAttempt: Attempt<void>;
  changePort(port: string): void;
  changePortAttempt: Attempt<void>;
  disconnect(): void;
  copyCliCommandToClipboard(): void;
}) {
  const formRef = useRef<HTMLFormElement>();
  const cliCommandPreview = props.gateway.gatewayCliCommand.preview;

  const { changePort } = props;
  const handleChangePort = useMemo(() => {
    return debounce((value: string) => {
      if (formRef.current.reportValidity()) {
        changePort(value);
      }
    }, 1000);
  }, [changePort]);

  return (
    <Box maxWidth="680px" width="100%" mx="auto" mt="4" px="5">
      <Flex justifyContent="space-between" mb="3" flexWrap="wrap" gap={2}>
        <Text typography="h3">App Connection</Text>
        {props.disconnectAttempt.status === 'error' && (
          <Alert>
            Could not close the connection: {props.disconnectAttempt.statusText}
          </Alert>
        )}
        <ButtonSecondary size="small" onClick={props.disconnect}>
          Close Connection
        </ButtonSecondary>
      </Flex>
      <Flex as="form" ref={formRef}>
        <Validation>
          <PortFieldInput
            label="Port"
            defaultValue={props.gateway.localPort}
            onChange={e => handleChangePort(e.target.value)}
            mb={2}
          />
        </Validation>
      </Flex>
      {cliCommandPreview && (
        <CliCommand
          cliCommand={cliCommandPreview}
          isLoading={props.changePortAttempt.status === 'processing'}
          buttonText="Copy"
          onButtonClick={props.copyCliCommandToClipboard}
        />
      )}
      {props.changePortAttempt.status === 'error' && (
        <Alert>
          Could not change the port number: {props.changePortAttempt.statusText}
        </Alert>
      )}
      <Text>
        Access the app at{' '}
        <code>
          {props.gateway.protocol.toLowerCase()}://{props.gateway.localAddress}:
          {props.gateway.localPort}
        </code>
        .
      </Text>
      <Text>
        The connection is made through an authenticated proxy so no extra
        credentials are necessary. See{' '}
        {/*TODO(gzdunek): Replace with Teleport Connect App access docs.*/}
        <Link
          href="https://goteleport.com/docs/application-access/guides/tcp/#step-34-start-app-proxy"
          target="_blank"
        >
          the documentation
        </Link>{' '}
        for more details.
      </Text>
    </Box>
  );
}
