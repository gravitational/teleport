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

import React, { useState } from 'react';
import { ButtonPrimary, Flex, Text } from 'design';

import * as Alerts from 'design/Alert';
import Validation from 'shared/components/Validation';
import { Attempt } from 'shared/hooks/useAsync';

import { PortFieldInput } from './FieldInputs';

export function OfflineGateway(props: {
  connectAttempt: Attempt<void>;
  /** Setting `isSupported` to false hides the port input. */
  gatewayPort:
    | { isSupported: true; defaultPort: string }
    | { isSupported: false };
  reconnect(port?: string): void;
  /** Gateway target displayed in the UI, for example, 'cockroachdb'. */
  targetName: string;
  /** Gateway kind displayed in the UI, for example, 'database'. */
  gatewayKind: string;
}) {
  const defaultPort = props.gatewayPort.isSupported
    ? props.gatewayPort.defaultPort
    : undefined;

  const [port, setPort] = useState(defaultPort);
  const [reconnectRequested, setReconnectRequested] = useState(false);

  const isProcessing = props.connectAttempt.status === 'processing';
  const statusDescription = isProcessing ? 'being set up…' : 'offline.';
  const shouldShowReconnectControls =
    props.connectAttempt.status === 'error' || reconnectRequested;

  return (
    <Flex
      flexDirection="column"
      mx="auto"
      mb="auto"
      alignItems="center"
      maxWidth="500px"
      css={`
        top: 11%;
        position: relative;
      `}
    >
      <Text typography="h4" bold>
        {props.targetName}
      </Text>
      <Text>
        The {props.gatewayKind} connection is {statusDescription}
      </Text>
      {props.connectAttempt.status === 'error' && (
        <Alerts.Danger mt={2} mb={0}>
          {props.connectAttempt.statusText}
        </Alerts.Danger>
      )}
      <Flex
        as="form"
        onSubmit={e => {
          e.preventDefault();
          setReconnectRequested(true);
          props.reconnect(props.gatewayPort.isSupported ? port : undefined);
        }}
        alignItems="flex-end"
        flexWrap="wrap"
        justifyContent="space-between"
        mt={3}
        gap={2}
      >
        {shouldShowReconnectControls && (
          <>
            {props.gatewayPort.isSupported && (
              <Validation>
                <PortFieldInput
                  label="Port (optional)"
                  value={port}
                  mb={0}
                  readonly={isProcessing}
                  onChange={e => setPort(e.target.value)}
                />
              </Validation>
            )}
            <ButtonPrimary type="submit" disabled={isProcessing}>
              Reconnect
            </ButtonPrimary>
          </>
        )}
      </Flex>
    </Flex>
  );
}
