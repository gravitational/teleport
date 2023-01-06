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
