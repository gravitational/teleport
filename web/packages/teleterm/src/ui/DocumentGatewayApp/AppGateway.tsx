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
  PropsWithChildren,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import styled from 'styled-components';

import {
  Alert,
  ButtonSecondary,
  disappear,
  Flex,
  H1,
  Link,
  rotate360,
  Text,
} from 'design';
import { Check, Spinner } from 'design/Icon';
import { Gateway } from 'gen-proto-ts/teleport/lib/teleterm/v1/gateway_pb';
import { LoginItem, MenuLogin } from 'shared/components/MenuLogin';
import { TextSelectCopy } from 'shared/components/TextSelectCopy';
import Validation from 'shared/components/Validation';
import { Attempt, useAsync } from 'shared/hooks/useAsync';
import { debounce } from 'shared/utils/highbar';

import {
  formatPortRange,
  portRangeSeparator,
} from 'teleterm/services/tshd/app';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { PortFieldInput } from 'teleterm/ui/components/FieldInputs';
import { useLogger } from 'teleterm/ui/hooks/useLogger';
import { setUpAppGateway } from 'teleterm/ui/services/workspacesService';
import { retryWithRelogin } from 'teleterm/ui/utils';

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
  const ctx = useAppContext();
  const { tshd } = ctx;
  const { targetUri } = gateway;
  const logger = useLogger('AppGateway');

  const {
    changeLocalPort,
    changeLocalPortAttempt,
    changeTargetPort,
    changeTargetPortAttempt,
    disconnectAttempt,
  } = props;
  // It must be possible to update local port while target port is invalid, hence why
  // useDebouncedPortChangeHandler checks the validity of only one input at a time. Otherwise the UI
  // would lose updates to the local port while the target port was invalid.
  const handleLocalPortChange = useDebouncedPortChangeHandler(changeLocalPort);
  const handleTargetPortChange =
    useDebouncedPortChangeHandler(changeTargetPort);

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
  const targetPortRef = useRef<HTMLInputElement>(null);

  const [tcpPortsAttempt, getTcpPorts] = useAsync(
    useCallback(
      () =>
        retryWithRelogin(ctx, targetUri, () =>
          tshd
            .getApp({ appUri: targetUri })
            .then(({ response }) => response.app.tcpPorts)
        ),
      [ctx, targetUri, tshd]
    )
  );
  const currentTargetPort = parseInt(gateway.targetSubresourceName);
  const getTcpPortsForMenuLogin: () => Promise<LoginItem[]> =
    useCallback(async () => {
      const [tcpPorts, error] = await getTcpPorts();

      if (error) {
        throw error;
      }

      return tcpPorts
        .filter(
          portRange =>
            // Filter out single-port port ranges that are equal to the current port.
            portRange.endPort !== 0 || portRange.port != currentTargetPort
        )
        .map(portRange => ({
          login: formatPortRange(portRange),
          url: '',
        }));
    }, [getTcpPorts, currentTargetPort]);

  const onPortRangeSelect = (_, formattedPortRange: string) => {
    const firstPort = formattedPortRange.split(portRangeSeparator)[0];
    const targetPort = parseInt(firstPort);

    if (Number.isNaN(targetPort)) {
      logger.error('Not a number', firstPort);
      return;
    }

    setUpAppGateway(ctx, targetUri, {
      telemetry: { origin: 'resource_table' },
      targetPort,
    });
  };

  return (
    <Flex
      flexDirection="column"
      maxWidth="680px"
      width="100%"
      mx="auto"
      mt="4"
      px="5"
      gap={3}
    >
      <Flex flexDirection="column" gap={2}>
        <Flex justifyContent="space-between" mb="2" flexWrap="wrap" gap={2}>
          <H1>App Connection</H1>
          <Flex gap={2}>
            {isMultiPort && (
              <MenuLogin
                getLoginItems={getTcpPortsForMenuLogin}
                onSelect={onPortRangeSelect}
                textTransform="none"
                placeholder="Pick target port"
                ButtonComponent={ButtonSecondary}
                buttonText="Open New Connection"
              />
            )}
            <ButtonSecondary size="small" onClick={props.disconnect}>
              Close Connection
            </ButtonSecondary>
          </Flex>
        </Flex>

        {disconnectAttempt.status === 'error' && (
          <Alert details={disconnectAttempt.statusText} m={0}>
            Could not close the connection
          </Alert>
        )}

        <Flex as="form" gap={2}>
          <Validation>
            <PortFieldInput
              label={
                <LabelWithAttemptStatus
                  text="Local Port"
                  attempt={changeLocalPortAttempt}
                />
              }
              defaultValue={gateway.localPort}
              onChange={handleLocalPortChange}
              mb={0}
            />
            {isMultiPort && (
              <PortFieldInput
                label={
                  <LabelWithAttemptStatus
                    text="Target Port"
                    attempt={changeTargetPortAttempt}
                  />
                }
                required
                defaultValue={gateway.targetSubresourceName}
                onChange={handleTargetPortChange}
                mb={0}
                ref={targetPortRef}
              />
            )}
          </Validation>
        </Flex>
      </Flex>

      <Flex flexDirection="column" gap={2}>
        <div>
          <Text>Access the app at:</Text>
          <TextSelectCopy mt={1} text={address} bash={false} />
        </div>

        {changeLocalPortAttempt.status === 'error' && (
          <Alert details={changeLocalPortAttempt.statusText} m={0}>
            Could not change the local port
          </Alert>
        )}

        {changeTargetPortAttempt.status === 'error' && (
          <Alert details={changeTargetPortAttempt.statusText} m={0}>
            Could not change the target port
          </Alert>
        )}

        {tcpPortsAttempt.status === 'error' && (
          <Alert kind="warning" details={tcpPortsAttempt.statusText} m={0}>
            Could not fetch available target ports
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
      </Flex>
    </Flex>
  );
}

const LabelWithAttemptStatus = (props: {
  text: string;
  attempt: Attempt<unknown>;
}) => (
  <Flex
    alignItems="center"
    justifyContent="space-between"
    css={`
      position: relative;
    `}
  >
    {props.text}
    {props.attempt.status === 'processing' && (
      <AnimatedSpinner color="interactive.tonal.neutral.2" size="small" />
    )}
    {props.attempt.status === 'success' && (
      // CSS animations are repeated whenever the parent goes from `display: none` to something
      // else. As a result, we need to unmount the animated check so that the animation is not
      // repeated when the user switches to this tab.
      // https://www.w3.org/TR/css-animations-1/#example-4e34d7ba
      <UnmountAfter timeoutMs={disappearanceDelayMs + disappearanceDurationMs}>
        <DisappearingCheck
          color="success.main"
          size="small"
          title={`${props.text} successfully updated`}
        />
      </UnmountAfter>
    )}
  </Flex>
);

/**
 * useDebouncedPortChangeHandler returns a debounced change handler that calls the change function
 * only if the input from which the event originated is valid.
 */
const useDebouncedPortChangeHandler = (
  changeFunc: (port: string) => void
): ChangeEventHandler<HTMLInputElement> =>
  useMemo(
    () =>
      debounce((event: ChangeEvent<HTMLInputElement>) => {
        if (event.target.reportValidity()) {
          changeFunc(event.target.value);
        }
      }, 1000),
    [changeFunc]
  );

const AnimatedSpinner = styled(Spinner)`
  animation: ${rotate360} 1.5s infinite linear;
  // The spinner needs to be positioned absolutely so that the fact that it's spinning
  // doesn't affect the size of the parent.
  position: absolute;
  right: 0;
  top: 0;
`;

const disappearanceDelayMs = 1000;
const disappearanceDurationMs = 200;

const DisappearingCheck = styled(Check)`
  opacity: 1;
  animation: ${disappear};
  animation-delay: ${disappearanceDelayMs}ms;
  animation-duration: ${disappearanceDurationMs}ms;
  animation-fill-mode: forwards;
`;

const UnmountAfter = ({
  timeoutMs,
  children,
}: PropsWithChildren<{ timeoutMs: number }>) => {
  const [isMounted, setIsMounted] = useState(true);

  useEffect(() => {
    const timeout = setTimeout(() => {
      setIsMounted(false);
    }, timeoutMs);

    return () => {
      clearTimeout(timeout);
    };
  }, [timeoutMs]);

  return isMounted ? children : null;
};
