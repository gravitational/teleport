/*
Copyright 2023 Gravitational, Inc.

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

import React from 'react';
import {
  Alert,
  Box,
  ButtonPrimary,
  Flex,
  Label,
  Link,
  MenuItem,
  Text,
} from 'design';
import styled, { css } from 'styled-components';
import { Transition } from 'react-transition-group';

import { makeLabelTag } from 'teleport/components/formatters';
import { MenuIcon } from 'shared/components/MenuAction';
import { Laptop } from 'design/Icon';

import {
  AgentState,
  useConnectMyComputerContext,
} from 'teleterm/ui/ConnectMyComputer';
import Document from 'teleterm/ui/Document';
import * as types from 'teleterm/ui/services/workspacesService';
import { useWorkspaceContext } from 'teleterm/ui/Documents';

import { useAgentProperties } from '../useAgentProperties';

import { StackTrace } from '../StackTrace';

import type * as tsh from 'teleterm/services/tshd/types';

interface DocumentConnectMyComputerStatusProps {
  visible: boolean;
  doc: types.DocumentConnectMyComputerStatus;
}

export function DocumentConnectMyComputerStatus(
  props: DocumentConnectMyComputerStatusProps
) {
  const {
    agentState,
    agentNode,
    downloadAndStartAgent,
    killAgent,
    isAgentConfiguredAttempt,
  } = useConnectMyComputerContext();
  const { documentsService, rootClusterUri } = useWorkspaceContext();
  const { roleName, systemUsername, hostname } = useAgentProperties();

  const prettyAgentState = prettifyAgentState(agentState);

  function replaceWithSetupDocument(): void {
    documentsService.replace(
      props.doc.uri,
      documentsService.createConnectMyComputerSetupDocument({
        rootClusterUri,
      })
    );
  }

  return (
    <Document visible={props.visible}>
      <Box maxWidth="590px" mx="auto" mt="4" px="5" width="100%">
        {isAgentConfiguredAttempt.status === 'error' && (
          <Alert
            css={`
              display: block;
            `}
          >
            An error occurred while reading the agent config file:{' '}
            {isAgentConfiguredAttempt.statusText}. <br />
            You can try to{' '}
            <Link
              onClick={replaceWithSetupDocument}
              css={`
                cursor: pointer;
              `}
            >
              run the setup
            </Link>{' '}
            again.
          </Alert>
        )}
        <Flex justifyContent="space-between" mb={3}>
          <Text
            typography="h3"
            css={`
              display: flex;
            `}
          >
            <Laptop mr={2} />
            {/** The node name can be changed, so it might be different from the system hostname. */}
            {agentNode?.hostname || hostname}
          </Text>
          <MenuIcon
            buttonIconProps={{
              css: css`
                border-radius: ${props => props.theme.space[1]}px;
                background: ${props => props.theme.colors.spotBackground[0]};
              `,
            }}
            menuProps={{
              anchorOrigin: {
                vertical: 'bottom',
                horizontal: 'right',
              },
              transformOrigin: {
                vertical: 'top',
                horizontal: 'right',
              },
            }}
          >
            <MenuItem onClick={() => alert('Not implemented')}>
              Remove agent
            </MenuItem>
          </MenuIcon>
        </Flex>

        <Transition in={!!agentNode} timeout={1_800} mountOnEnter>
          {state => (
            <LabelsContainer gap={1} className={state}>
              {renderLabels(agentNode.labelsList)}
            </LabelsContainer>
          )}
        </Transition>
        <Flex mt={3} mb={2} display="flex" alignItems="center">
          {prettyAgentState.title}
        </Flex>
        {prettyAgentState.error && (
          <Alert
            css={`
              white-space: pre-wrap;
            `}
          >
            {prettyAgentState.error}
          </Alert>
        )}
        {prettyAgentState.stackTrace && (
          <StackTrace lines={prettyAgentState.stackTrace} />
        )}
        <Text mb={4} mt={1}>
          Connecting your computer will allow any cluster user with the role{' '}
          <strong>{roleName}</strong> to access it as an SSH resource with the
          user <strong>{systemUsername}</strong>.
        </Text>
        {agentState.status === 'process-running' ||
        agentState.status === 'killing' ? (
          <ButtonPrimary
            block
            disabled={agentState.status === 'killing'}
            onClick={killAgent}
          >
            Disconnect
          </ButtonPrimary>
        ) : (
          <ButtonPrimary
            block
            disabled={
              agentState.status === 'downloading' ||
              agentState.status === 'starting'
            }
            onClick={downloadAndStartAgent}
          >
            Connect
          </ButtonPrimary>
        )}
      </Box>
    </Document>
  );
}

function renderLabels(labelsList: tsh.Label[]): JSX.Element[] {
  const labels = labelsList.map(makeLabelTag);
  return labels.map(label => (
    <Label key={label} kind="secondary">
      {label}
    </Label>
  ));
}

function prettifyAgentState(agentState: AgentState): {
  title: string;
  error?: string;
  stackTrace?: string;
} {
  switch (agentState.status) {
    case 'downloading': {
      //TODO(gzdunek) add progress
      return { title: '🔄 Verifying binary' };
    }
    case 'starting':
      return { title: '🔄 Starting' };
    case 'killing':
      return { title: '🔄 Stopping' };
    case 'process-not-started': {
      return { title: '🔘 Agent not running' };
    }
    case 'process-running': {
      return { title: '🟢 Agent running' };
    }
    case 'process-exited': {
      const { code, signal, exitedSuccessfully } = agentState;
      const codeOrSignal = [
        // code can be 0, so we cannot just check it the same way as the signal.
        code != null && `code ${code}`,
        signal && `signal ${signal}`,
      ]
        .filter(Boolean)
        .join(' ');

      return {
        title: [
          exitedSuccessfully ? '🔘' : '🔴',
          `Agent process exited with ${codeOrSignal}`,
        ].join('\n'),
        stackTrace: agentState.stackTrace,
      };
    }
    case 'download-error': {
      return {
        title: '🔴 Failed to download agent',
        error: agentState.message,
      };
    }
    case 'kill-error': {
      return {
        title: '🔴 Failed to kill agent',
        error: agentState.message,
      };
    }
    case 'join-error': {
      return {
        title: '🔴 Failed to join cluster',
        error: agentState.message,
      };
    }
    case 'process-error': {
      return {
        title: '🔴 An error occurred to the agent process.',
        error: agentState.message,
      };
    }
    default: {
      return {
        title: '',
      };
    }
  }
}

const LabelsContainer = styled(Flex)`
  &.entering {
    animation-duration: 1.8s;
    animation-name: lineInserted;
    animation-timing-function: ease-in;
    overflow: hidden;
    animation-fill-mode: forwards;
    // We don't know the height of labels, so we animate max-height instead of height
    @keyframes lineInserted {
      from {
        max-height: 0;
      }
      to {
        max-height: 100%;
      }
    }
  }
`;
