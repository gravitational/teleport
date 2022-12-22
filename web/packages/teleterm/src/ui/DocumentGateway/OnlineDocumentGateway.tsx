import React, { useMemo, useRef } from 'react';
import { debounce } from 'lodash';
import { Box, ButtonSecondary, Flex, Link, Text } from 'design';
import Validation from 'shared/components/Validation';
import * as Alerts from 'design/Alert';

import { CliCommand } from './CliCommand';
import { ConfigFieldInput, PortFieldInput } from './common';
import { DocumentGatewayProps } from './DocumentGateway';

type OnlineDocumentGatewayProps = Pick<
  DocumentGatewayProps,
  | 'changeDbNameAttempt'
  | 'changePortAttempt'
  | 'disconnect'
  | 'changeDbName'
  | 'changePort'
  | 'gateway'
  | 'runCliCommand'
>;

export function OnlineDocumentGateway(props: OnlineDocumentGatewayProps) {
  const isPortOrDbNameProcessing =
    props.changeDbNameAttempt.status === 'processing' ||
    props.changePortAttempt.status === 'processing';
  const hasError =
    props.changeDbNameAttempt.status === 'error' ||
    props.changePortAttempt.status === 'error';
  const formRef = useRef<HTMLFormElement>();
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
        <Alerts.Danger mb={0}>
          Could not change the database name:{' '}
          {props.changeDbNameAttempt.statusText}
        </Alerts.Danger>
      )}
      {props.changePortAttempt.status === 'error' && (
        <Alerts.Danger mb={0}>
          Could not change the port number: {props.changePortAttempt.statusText}
        </Alerts.Danger>
      )}
    </Flex>
  );

  return (
    <Box maxWidth="590px" width="100%" mx="auto" mt="4" px="5">
      <Flex justifyContent="space-between" mb="4" flexWrap="wrap" gap={2}>
        <Text typography="h3" color="text.secondary">
          Database Connection
        </Text>
        <ButtonSecondary size="small" onClick={props.disconnect}>
          Close Connection
        </ButtonSecondary>
      </Flex>
      <Text typography="h4" mb={1}>
        Connect with CLI
      </Text>
      <Flex as="form" ref={formRef}>
        <Validation>
          <PortFieldInput
            label="Port"
            defaultValue={gateway.localPort}
            onChange={e => handleChangePort(e.target.value)}
            mb={2}
          />
          <ConfigFieldInput
            label="Database name"
            defaultValue={gateway.targetSubresourceName}
            onChange={e => handleChangeDbName(e.target.value)}
            spellCheck={false}
            ml={2}
            mb={2}
          />
        </Validation>
      </Flex>
      <CliCommand
        cliCommand={props.gateway.cliCommand}
        isLoading={isPortOrDbNameProcessing}
        onRun={props.runCliCommand}
      />
      {$errors}
      <Text typography="h4" mt={3} mb={1}>
        Connect with GUI
      </Text>
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
          href="https://goteleport.com/docs/database-access/guides/gui-clients/"
          target="_blank"
        >
          the documentation
        </Link>{' '}
        for more details.
      </Text>
    </Box>
  );
}
