/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React, { useEffect, useMemo, useState } from 'react';
import {
  Text,
  Flex,
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Link,
  Indicator,
} from 'design';

import * as Alerts from 'design/Alert';

import FieldInput from 'shared/components/FieldInput';
import Validation from 'shared/components/Validation';
import { debounce } from 'lodash';
import styled from 'styled-components';

import LinearProgress from 'teleterm/ui/components/LinearProgress';
import * as types from 'teleterm/ui/services/workspacesService';
import Document from 'teleterm/ui/Document';

import useDocumentGateway, { State } from './useDocumentGateway';

type Props = {
  visible: boolean;
  doc: types.DocumentGateway;
};

export default function Container(props: Props) {
  const { doc, visible } = props;
  const state = useDocumentGateway(doc);
  return (
    <Document visible={visible}>
      <DocumentGateway {...state} />
    </Document>
  );
}

export function DocumentGateway(props: State) {
  const {
    gateway,
    connected,
    connectAttempt,
    disconnect,
    reconnect,
    runCliCommand,
    changeDbName,
    changeDbNameAttempt,
  } = props;

  const handleChangeDbName = useMemo(() => {
    return debounce((value: string) => {
      changeDbName(value);
    }, 150);
  }, [changeDbName]);

  const isLoading = changeDbNameAttempt.status === 'processing';

  if (!connected) {
    const statusDescription =
      connectAttempt.status === 'processing' ? 'being set up' : 'offline';

    return (
      <Flex flexDirection="column" mx="auto" alignItems="center" mt={100}>
        <Text
          typography="h5"
          color="text.primary"
          style={{ position: 'relative' }}
        >
          The database connection is {statusDescription}
          {connectAttempt.status === 'processing' && <LinearProgress />}
        </Text>
        {connectAttempt.status === 'error' && (
          <Alerts.Danger>{connectAttempt.statusText}</Alerts.Danger>
        )}
        <ButtonPrimary
          mt={4}
          width="100px"
          onClick={reconnect}
          disabled={connectAttempt.status === 'processing'}
        >
          Reconnect
        </ButtonPrimary>
      </Flex>
    );
  }

  return (
    <Box maxWidth="590px" width="100%" mx="auto" mt="4" px="5">
      <Flex justifyContent="space-between" mb="4" flexWrap="wrap">
        <Text typography="h3" color="text.secondary">
          Database Connection
        </Text>
        <ButtonSecondary size="small" onClick={disconnect}>
          Close Connection
        </ButtonSecondary>
      </Flex>
      <Text typography="h4" mb={1}>
        Connect with CLI
      </Text>
      <Flex>
        <Validation>
          <ConfigInput
            label="Database name"
            defaultValue={gateway.targetSubresourceName}
            onChange={e => handleChangeDbName(e.target.value)}
            spellCheck={false}
            mb={2}
          />
        </Validation>
      </Flex>
      <CliCommand
        cliCommand={gateway.cliCommand}
        isLoading={isLoading}
        onRun={runCliCommand}
      />
      {changeDbNameAttempt.status === 'error' && (
        <Alerts.Danger>
          Could not change the database name: {changeDbNameAttempt.statusText}
        </Alerts.Danger>
      )}
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

function CliCommand({
  cliCommand,
  onRun,
  isLoading,
}: {
  cliCommand: string;
  onRun(): void;
  isLoading: boolean;
}) {
  const [shouldDisplayIsLoading, setShouldDisplayIsLoading] = useState(false);

  useEffect(() => {
    let timeout;
    if (isLoading) {
      timeout = setTimeout(() => {
        setShouldDisplayIsLoading(true);
      }, 200);
    } else {
      setShouldDisplayIsLoading(false);
    }

    return () => clearTimeout(timeout);
  }, [isLoading]);

  return (
    <Flex
      p="2"
      alignItems="center"
      justifyContent="space-between"
      borderRadius={2}
      bg={'primary.dark'}
    >
      <Flex
        mr="2"
        color={shouldDisplayIsLoading ? 'text.secondary' : 'text.primary'}
        width="100%"
        css={`
          overflow: auto;
          white-space: pre;
          word-break: break-all;
          font-size: 12px;
          font-family: ${props => props.theme.fonts.mono};
        `}
      >
        <Box mr="1">{`$`}</Box>
        <span>{cliCommand}</span>
        {shouldDisplayIsLoading && (
          <Indicator
            fontSize="14px"
            delay="none"
            css={`
              display: inline;
              margin: auto 0 auto auto;
            `}
          />
        )}
      </Flex>
      <ButtonPrimary
        onClick={onRun}
        disabled={shouldDisplayIsLoading}
        css={`
          max-width: 48px;
          width: 100%;
          padding: 4px 8px;
          min-height: 10px;
          font-size: 10px;
        `}
      >
        Run
      </ButtonPrimary>
    </Flex>
  );
}

const ConfigInput: typeof FieldInput = styled(FieldInput)`
  input {
    background: inherit;
    border: 1px ${props => props.theme.colors.action.disabledBackground} solid;
    color: ${props => props.theme.colors.text.primary};
    box-shadow: none;
    font-size: 14px;
    height: 34px;

    ::placeholder {
      opacity: 1;
      color: ${props => props.theme.colors.text.secondary};
    }

    &:hover {
      border-color: ${props => props.theme.colors.text.secondary};
    }

    &:focus {
      border-color: ${props => props.theme.colors.secondary.main};
    }
`;
