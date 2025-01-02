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

import {
  ChangeEvent,
  ChangeEventHandler,
  MutableRefObject,
  useMemo,
  useRef,
} from 'react';

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
  changeTargetPort(port: string): void;
  changeTargetPortAttempt: Attempt<void>;
  disconnect(): void;
}) {
  const { gateway } = props;
  const formRef = useRef<HTMLFormElement>();

  const { changeLocalPort, changeTargetPort } = props;
  const handleLocalPortChange = useDebouncedPortChangeHandler(
    formRef,
    changeLocalPort
  );
  const handleTargetPortChange = useDebouncedPortChangeHandler(
    formRef,
    changeTargetPort
  );

  let address = `${gateway.localAddress}:${gateway.localPort}`;
  if (gateway.protocol === 'HTTP') {
    address = `http://${address}`;
  }

  // AppGateway doesn't have access to the app resource itself, so it has to decide whether the
  // app is multi-port or not in some other way.
  // For multi-port apps, DocumentGateway comes with targetSubresourceName prefilled to the first
  // port number found in TCP ports. Single-port apps have this field empty.
  // So, if targetSubresourceName is present, then the app must be multi-port. In this case, the
  // user is free to change it and can never provide an empty targetSubresourceName.
  // When the app is not multi-port, targetSubresourceName is empty and the user cannot change it.
  const isMultiPort =
    gateway.protocol === 'TCP' && gateway.targetSubresourceName;

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

      {
        // TODO: Disable fields while a request is in progress.
      }
      <Flex as="form" ref={formRef} gap={2}>
        <Validation>
          <PortFieldInput
            label="Local Port"
            defaultValue={gateway.localPort}
            onChange={handleLocalPortChange}
            mb={2}
          />
          {
            // TODO: How do we check if the app is multi-port?
          }
          {isMultiPort && (
            // TODO: We need to show available target ports here.
            <PortFieldInput
              label="Target Port"
              required
              defaultValue={gateway.targetSubresourceName}
              onChange={handleTargetPortChange}
              mb={2}
            />
          )}
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

/**
 * useDebouncedPortChangeHandler returns a debounced change handler that calls the change function
 * only if the form is valid.
 */
const useDebouncedPortChangeHandler = (
  formRef: MutableRefObject<HTMLFormElement>,
  changeFunc: (port: string) => void
): ChangeEventHandler<HTMLInputElement> =>
  useMemo(
    () =>
      debounce((event: ChangeEvent<HTMLInputElement>) => {
        const value = event.target.value;
        if (formRef.current.reportValidity()) {
          changeFunc(value);
        }
      }, 1000),
    [formRef, changeFunc]
  );
