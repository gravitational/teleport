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

import { Flex, Box, Text, ButtonPrimary, Alert, ButtonSecondary } from 'design';

import { Cross } from 'design/Icon';

import { Option } from 'shared/components/Select';

import { SelectCreatable } from 'teleport/Discover/Shared/SelectCreatable';

import * as types from 'teleterm/ui/services/workspacesService';
import Document from 'teleterm/ui/Document';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import {
  useConnectMyComputerContext,
  CurrentAction,
} from 'teleterm/ui/ConnectMyComputer';
import { Logs } from 'teleterm/ui/ConnectMyComputer/Logs';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { getFileSharingAppName } from 'teleterm/fileSharing';

import { prettifyCurrentAction } from '../DocumentConnectMyComputer/Status';

export function DocumentFileSharing(props: {
  visible: boolean;
  doc: types.DocumentFileSharing;
}) {
  const { mainProcessClient, connectMyComputerService, clustersService } =
    useAppContext();
  const { rootClusterUri } = useWorkspaceContext();
  const { currentAction, killAgent, startAgent } =
    useConnectMyComputerContext();
  const cluster = clustersService.findCluster(rootClusterUri);
  const [selectedDirectory, setSelectedDirectory] = useState<string>();
  const isRunning =
    currentAction.kind === 'observe-process' &&
    currentAction.agentProcessState.status === 'running';
  let appUrl =
    cluster?.loggedInUser &&
    `https://${getFileSharingAppName(cluster.loggedInUser.name)}.${
      cluster.proxyHost
    }`;
  const [allowedUsersInputValue, setAllowedUsersInputValue] = useState('');
  const [allowedRolesInputValue, setAllowedRolesInputValue] = useState('');
  const [allowAnyone, setAllowAnyone] = useState(false);
  const [allowedUsers, setAllowedUsers] = useState<Option[]>([]);
  const [allowedRoles, setAllowedRoles] = useState<Option[]>([]);

  if (selectedDirectory) {
    appUrl += '/file-sharing';
  }

  async function updateServerConfig(args: {
    allowAnyone: boolean;
    path: string;
    allowedUsersList: Option[];
    allowedRolesList: Option[];
  }) {
    await connectMyComputerService.setFileServerConfig({
      clusterUri: rootClusterUri,
      config: {
        sharesList: args.path
          ? [
              {
                name: 'file-sharing',
                path: args.path,
                allowAnyone: args.allowAnyone,
                allowedUsersList:
                  args.allowedUsersList?.map(r => r.value) || [],
                allowedRolesList:
                  args.allowedRolesList?.map(r => r.value) || [],
              },
            ]
          : [],
      },
    });
    setSelectedDirectory(args.path);
    setAllowedUsers(args.allowedUsersList);
    setAllowedRoles(args.allowedRolesList);
    setAllowAnyone(args.allowAnyone);
  }

  return (
    <Document visible={props.visible}>
      <Box maxWidth="850px" mx="auto" mt="4" px="5" width="100%">
        <Text typography="h3" mb="4">
          File Sharing
        </Text>
        <Flex gap={4} justifyContent="space-between" flexWrap="wrap">
          <AgentStatus currentAction={currentAction} killAgent={killAgent} />
          <Flex gap={2} flexWrap="wrap">
            <ButtonSecondary
              onClick={() => mainProcessClient.clipboardWriteText(appUrl)}
            >
              Copy link
            </ButtonSecondary>
            <ButtonSecondary as="a" target="_blank" href={appUrl}>
              Open app
            </ButtonSecondary>
            {isRunning && (
              <ButtonPrimary
                onClick={() => {
                  killAgent();
                }}
              >
                Stop agent
              </ButtonPrimary>
            )}
            {!isRunning && selectedDirectory && (
              <ButtonPrimary
                onClick={() => {
                  startAgent('');
                }}
              >
                Start agent
              </ButtonPrimary>
            )}
          </Flex>
        </Flex>
        <Flex gap={3} mt={3}>
          <Box flex="2">
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
              <>
                <Flex justifyContent="space-between">
                  {selectedDirectory || 'No directory selected'}
                  {selectedDirectory && (
                    <Cross
                      css={`
                        cursor: pointer;
                      `}
                      onClick={() => {
                        updateServerConfig({
                          path: undefined,
                          allowAnyone: false,
                          allowedRolesList: [],
                          allowedUsersList: [],
                        });
                      }}
                    />
                  )}
                </Flex>
                <ButtonPrimary
                  onClick={async () => {
                    const { filePaths, canceled } =
                      await mainProcessClient.showDirectorySelectDialog();
                    if (!canceled) {
                      updateServerConfig({
                        allowAnyone,
                        path: filePaths[0],
                        allowedUsersList: allowedUsers,
                        allowedRolesList: allowedRoles,
                      });
                      if (!isRunning) {
                        startAgent('');
                      }
                    }
                  }}
                >
                  {selectedDirectory ? 'Change directory' : 'Select & share'}
                </ButtonPrimary>
              </>
            </Flex>
          </Box>

          <Flex
            flex="1"
            flexDirection="column"
            gap={2}
            maxWidth="260px"
            css={`
              flex-shrink: 0;
            `}
          >
            <label>
              Share with
              <select
                name="allowAnyone"
                value={String(allowAnyone)}
                onChange={event => {
                  const updatedAllowAnyone = event.target.value === 'true';
                  updateServerConfig({
                    allowAnyone: updatedAllowAnyone,
                    allowedRolesList: allowedRoles,
                    allowedUsersList: allowedUsers,
                    path: selectedDirectory,
                  });
                }}
                css={`
                  margin-left: ${props => props.theme.space[1]}px;
                `}
              >
                <option value="false">specific people</option>
                <option value="true">anyone</option>
              </select>
            </label>
            {!allowAnyone && (
              <>
                <Text>Allow users</Text>
                <SelectCreatable
                  inputValue={allowedUsersInputValue}
                  onInputChange={setAllowedUsersInputValue}
                  options={[{ label: 'sadf', value: 'sadf' }]}
                  onChange={users => {
                    updateServerConfig({
                      allowAnyone,
                      path: selectedDirectory,
                      allowedUsersList: users,
                      allowedRolesList: allowedRoles,
                    });
                  }}
                  value={allowedUsers}
                  formatCreateLabel={inputValue => `Add "${inputValue}"`}
                />{' '}
                <Text>Allow roles</Text>
                <SelectCreatable
                  options={[]}
                  inputValue={allowedRolesInputValue}
                  onInputChange={setAllowedRolesInputValue}
                  onChange={roles => {
                    updateServerConfig({
                      allowAnyone,
                      path: selectedDirectory,
                      allowedUsersList: allowedUsers,
                      allowedRolesList: roles,
                    });
                  }}
                  value={allowedRoles}
                  formatCreateLabel={inputValue => `Add "${inputValue}"`}
                />
              </>
            )}{' '}
          </Flex>
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
