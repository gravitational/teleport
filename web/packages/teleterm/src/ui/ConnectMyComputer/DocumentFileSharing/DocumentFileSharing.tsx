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

import { Flex, Box, Text, ButtonPrimary, Alert } from 'design';

import * as types from 'teleterm/ui/services/workspacesService';
import Document from 'teleterm/ui/Document';

import { useAppContext } from 'teleterm/ui/appContextProvider';

import {
  useConnectMyComputerContext,
  CurrentAction,
} from 'teleterm/ui/ConnectMyComputer';
import { Logs } from 'teleterm/ui/ConnectMyComputer/Logs';

import { prettifyCurrentAction } from '../DocumentConnectMyComputer/Status';

export function DocumentFileSharing(props: {
  visible: boolean;
  doc: types.DocumentFileSharing;
}) {
  const { mainProcessClient } = useAppContext();
  const { currentAction, killAgent, startAgent } =
    useConnectMyComputerContext();
  const [selectedDirectory, setSelectedDirectory] = useState<string>();

  return (
    <Document visible={props.visible}>
      <Box maxWidth="680px" mx="auto" mt="4" px="5" width="100%">
        <Text typography="h3" mb="4">
          File Sharing
        </Text>
        <AgentStatus currentAction={currentAction} killAgent={killAgent} />
        <Flex
          flexDirection="column"
          gap={3}
          mt={2}
          p={3}
          borderRadius={2}
          width="100%"
          css={`
            background: ${props => props.theme.colors.spotBackground[0]};
          `}
        >
          {!selectedDirectory && (
            <>
              No directory selected
              <ButtonPrimary
                onClick={async () => {
                  const { filePaths, canceled } =
                    await mainProcessClient.showDirectorySelectDialog();
                  if (!canceled) {
                    setSelectedDirectory(filePaths[0]);
                    startAgent('');
                  }
                }}
              >
                Select & share
              </ButtonPrimary>
            </>
          )}
          {selectedDirectory && (
            <>
              {selectedDirectory}
              <ButtonPrimary
                onClick={() => {
                  setSelectedDirectory(undefined);
                  killAgent();
                }}
              >
                Clear & stop sharing
              </ButtonPrimary>
            </>
          )}
        </Flex>
      </Box>
    </Document>
  );
}

function AgentStatus(props: {
  currentAction: CurrentAction;
  killAgent(): void;
}) {
  const prettyCurrentAction = prettifyCurrentAction(props.currentAction);

  return (
    <Flex flexDirection="column" gap={2}>
      <Flex gap={1} display="flex" alignItems="center" minHeight="32px">
        {prettyCurrentAction.Icon && <prettyCurrentAction.Icon size="medium" />}
        {prettyCurrentAction.title}
      </Flex>
      {prettyCurrentAction.error && (
        <Alert
          mb={0}
          css={`
            white-space: pre-wrap;
          `}
        >
          {prettyCurrentAction.error}
        </Alert>
      )}
      {prettyCurrentAction.logs && <Logs logs={prettyCurrentAction.logs} />}
    </Flex>
  );
}
