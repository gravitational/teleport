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

import {
  Alert,
  Box,
  ButtonSecondary,
  Flex,
  H1,
  Indicator,
  Link,
  Text,
} from 'design';
import { Gateway } from 'gen-proto-ts/teleport/lib/teleterm/v1/gateway_pb';
import { TextSelectCopy } from 'shared/components/TextSelectCopy';
import Validation from 'shared/components/Validation';
import { Attempt } from 'shared/hooks/useAsync';
import { debounce } from 'shared/utils/highbar';

import { PortFieldInput } from '../components/FieldInputs';

export function AppGateway(props: {
  gateway: Gateway;
  disconnectAttempt: Attempt<void>;
  changeLocalPort(port: string): void;
  changeLocalPortAttempt: Attempt<void>;
  disconnect(): void;
}) {
  const { gateway } = props;
  const formRef = useRef<HTMLFormElement>();

  const { changeLocalPort } = props;
  const handleChangeLocalPort = useMemo(() => {
    return debounce((value: string) => {
      if (formRef.current.reportValidity()) {
        changeLocalPort(value);
      }
    }, 1000);
  }, [changeLocalPort]);

  let address = `${gateway.localAddress}:${gateway.localPort}`;
  if (gateway.protocol === 'HTTP') {
    address = `http://${address}`;
  }

  return (
    <Box maxWidth="680px" width="100%" mx="auto" mt="4" px="5">
      <Flex justifyContent="space-between" mb="3" flexWrap="wrap" gap={2}>
        <H1>App Connection</H1>
        <ButtonSecondary size="small" onClick={props.disconnect}>
          Close Connection
        </ButtonSecondary>
      </Flex>

      {props.disconnectAttempt.status === 'error' && (
        <Alert details={props.disconnectAttempt.statusText}>
          Could not close the connection
        </Alert>
      )}

      <Flex as="form" ref={formRef} gap={2}>
        <Validation>
          <PortFieldInput
            label="Port"
            defaultValue={gateway.localPort}
            onChange={e => handleChangeLocalPort(e.target.value)}
            mb={2}
          />
        </Validation>
        {props.changeLocalPortAttempt.status === 'processing' && (
          <Indicator
            size="large"
            pt={3} // aligns the spinner to be at the center of the port input
            css={`
              display: flex;
            `}
          />
        )}
      </Flex>

      <Text>Access the app at:</Text>
      <TextSelectCopy my={1} text={address} bash={false} />

      {props.changeLocalPortAttempt.status === 'error' && (
        <Alert details={props.changeLocalPortAttempt.statusText}>
          Could not change the port number
        </Alert>
      )}

      <Text>
        The connection is made through an authenticated proxy so no extra
        credentials are necessary. See{' '}
        <Link
          href="https://goteleport.com/docs/connect-your-client/teleport-connect/#creating-an-authenticated-tunnel"
          target="_blank"
        >
          the documentation
        </Link>{' '}
        for more details.
      </Text>
    </Box>
  );
}
