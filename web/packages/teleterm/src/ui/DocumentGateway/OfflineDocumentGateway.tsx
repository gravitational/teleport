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

import React, { useState } from 'react';
import { ButtonPrimary, Flex, Text } from 'design';

import * as Alerts from 'design/Alert';
import Validation from 'shared/components/Validation';

import LinearProgress from 'teleterm/ui/components/LinearProgress';

import { PortFieldInput } from './common';
import { DocumentGatewayProps } from './DocumentGateway';

type OfflineDocumentGatewayProps = Pick<
  DocumentGatewayProps,
  'connectAttempt' | 'defaultPort' | 'reconnect'
>;

export function OfflineDocumentGateway(props: OfflineDocumentGatewayProps) {
  const [port, setPort] = useState(props.defaultPort);
  const statusDescription =
    props.connectAttempt.status === 'processing' ? 'being set up' : 'offline';
  const isProcessing = props.connectAttempt.status === 'processing';
  const shouldShowPortInput =
    props.connectAttempt.status === 'error' || port !== props.defaultPort;

  return (
    <Flex
      maxWidth="590px"
      width="100%"
      flexDirection="column"
      mx="auto"
      alignItems="center"
      mt={11}
    >
      <Text
        typography="h5"
        color="text.primary"
        mb={2}
        style={{ position: 'relative' }}
      >
        The database connection is {statusDescription}
        {/* TODO(ravicious): Use doc.status instead of LinearProgress. */}
        {props.connectAttempt.status === 'processing' && <LinearProgress />}
      </Text>
      {props.connectAttempt.status === 'error' && (
        <Alerts.Danger mb={0}>{props.connectAttempt.statusText}</Alerts.Danger>
      )}
      <Flex
        as="form"
        onSubmit={() => props.reconnect(port)}
        alignItems="flex-end"
        flexWrap="wrap"
        justifyContent="space-between"
        mt={3}
        gap={2}
      >
        {shouldShowPortInput && (
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
      </Flex>
    </Flex>
  );
}
