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

import { FormEvent, useState } from 'react';
import { z } from 'zod';

import { ButtonPrimary, Flex, H2, Text } from 'design';
import * as Alerts from 'design/Alert';
import Validation from 'shared/components/Validation';
import { Attempt } from 'shared/hooks/useAsync';

import { useLogger } from 'teleterm/ui/hooks/useLogger';

import { PortFieldInput } from './FieldInputs';

export function OfflineGateway(props: {
  connectAttempt: Attempt<void>;
  /** Setting `isSupported` to false hides the port input. */
  gatewayPort:
    | { isSupported: true; defaultPort: string }
    | { isSupported: false };
  reconnect(args: { localPort?: string }): void;
  /** Gateway target displayed in the UI, for example, 'cockroachdb'. */
  targetName: string;
  /** Gateway kind displayed in the UI, for example, 'database'. */
  gatewayKind: string;
  portFieldLabel?: string;
}) {
  const logger = useLogger('OfflineGateway');
  const { reconnect } = props;
  const portFieldLabel = props.portFieldLabel || 'Port (optional)';
  const defaultPort = props.gatewayPort.isSupported
    ? props.gatewayPort.defaultPort
    : undefined;

  const [reconnectRequested, setReconnectRequested] = useState(false);
  const [parseError, setParseError] = useState('');

  const isProcessing = props.connectAttempt.status === 'processing';
  const statusDescription = isProcessing ? 'being set up…' : 'offline.';
  const shouldShowReconnectControls =
    props.connectAttempt.status === 'error' || reconnectRequested;

  const submitForm = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setReconnectRequested(true);
    setParseError('');

    const formData = new FormData(event.currentTarget);
    const parseResult = schema.safeParse(
      Object.fromEntries(formData.entries())
    );

    // Explicitly compare to false to make type inference work since strictNullChecks are off.
    if (parseResult.success === false) {
      // There's no need to show validation errors in the UI, since they come from a programmer
      // error, not user inputting actually invalid data.
      logger.error('Could not parse form', parseResult.error);
      setParseError(`Could not submit form. See logs for more details.`);
      return;
    }

    reconnect(parseResult.data);
  };

  return (
    <Flex
      flexDirection="column"
      mx="auto"
      mb="auto"
      alignItems="center"
      px={4}
      css={`
        top: 11%;
        position: relative;
      `}
    >
      <H2 mb={1}>{props.targetName}</H2>
      <Text>
        The {props.gatewayKind} connection is {statusDescription}
      </Text>
      {props.connectAttempt.status === 'error' && (
        <Alerts.Danger mt={2} mb={0} details={props.connectAttempt.statusText}>
          Could not establish the connection
        </Alerts.Danger>
      )}
      {!!parseError && (
        <Alerts.Danger mt={2} mb={0} details={parseError}>
          Form validation error
        </Alerts.Danger>
      )}
      <Flex
        as="form"
        onSubmit={submitForm}
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
                  name={FIELD_NAME_LOCAL_PORT}
                  label={portFieldLabel}
                  defaultValue={defaultPort}
                  mb={0}
                  readonly={isProcessing}
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

export const FIELD_NAME_LOCAL_PORT = 'localPort';

export const schema = z.object({
  [FIELD_NAME_LOCAL_PORT]: z.string(),
});
