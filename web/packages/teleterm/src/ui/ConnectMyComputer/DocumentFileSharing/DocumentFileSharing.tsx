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

import React, { useState, useEffect } from 'react';
import { enableMapSet, produce } from 'immer';

import { Flex, Box, Text, ButtonPrimary, Alert, ButtonSecondary } from 'design';

import { Cross } from 'design/Icon';

import { Option } from 'shared/components/Select';

import { SelectCreatable } from 'teleport/Discover/Shared/SelectCreatable';

import { useAsync } from 'shared/hooks/useAsync';

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

enableMapSet();

export function DocumentFileSharing(props: {
  visible: boolean;
  doc: types.DocumentFileSharing;
}) {
  const { mainProcessClient, connectMyComputerService, clustersService, tshd } =
    useAppContext();
  const { rootClusterUri } = useWorkspaceContext();
  const { currentAction, killAgent, startAgent } =
    useConnectMyComputerContext();
  const cluster = clustersService.findCluster(rootClusterUri);
  const isRunning =
    currentAction.kind === 'observe-process' &&
    currentAction.agentProcessState.status === 'running';
  const appUrl =
    cluster?.loggedInUser &&
    `https://${getFileSharingAppName(cluster.loggedInUser.name)}.${
      cluster.proxyHost
    }`;

  const [listSuggestedUsersAttempt, runListSuggestedUsers] = useAsync(
    tshd.listUsers
  );
  const [listSuggestedRolesAttempt, runListSuggestedRoles] = useAsync(
    tshd.listRoles
  );
  const suggestedUsers =
    (listSuggestedUsersAttempt.status === 'success' &&
      listSuggestedUsersAttempt.data) ||
    [];
  const suggestedRoles =
    (listSuggestedRolesAttempt.status === 'success' &&
      listSuggestedRolesAttempt.data) ||
    cluster.loggedInUser?.rolesList ||
    [];
  const [fileShares, setFileShares] = useState<Map<string, FileShare>>(
    new Map()
  );

  // TODO: handle errors, think how to fetch fresh data
  useEffect(() => {
    runListSuggestedUsers({ clusterUri: rootClusterUri });
    runListSuggestedRoles({ clusterUri: rootClusterUri });
  }, [rootClusterUri, runListSuggestedRoles, runListSuggestedUsers]);

  async function updateServerConfig(updatedFileShare: FileShare) {
    setFileShares(
      produce(draft => {
        draft.set(updatedFileShare.path, updatedFileShare);
      })
    );

    // TODO: Pass updated set to the server.

    // TODO: Handle errors.
    await connectMyComputerService.setFileServerConfig({
      clusterUri: rootClusterUri,
      config: {
        sharesList: updatedFileShare.path
          ? [
              {
                name: 'file-sharing',
                path: updatedFileShare.path,
                allowAnyone: updatedFileShare.sharingMode === 'anyone',
                allowedUsersList: updatedFileShare.allowedUsers,
                allowedRolesList: updatedFileShare.allowedRoles,
              },
            ]
          : [],
      },
    });
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
            {!isRunning && fileShare.path && (
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
        <FileShareForm
          fileShare={fileShare}
          onChange={updateServerConfig}
          suggestedUsers={suggestedUsers}
          suggestedRoles={suggestedRoles}
        />
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

type FileShare = {
  path: string;
  sharingMode: SharingMode;
  allowedUsers: string[];
  allowedRoles: string[];
};

type SharingMode = 'anyone' | 'specific-people';

export function FileShareForm(props: {
  fileShare: FileShare;
  onChange: (fileShare: FileShare) => void;
  suggestedUsers: string[];
  suggestedRoles: string[];
}) {
  const { fileShare } = props;
  const { mainProcessClient } = useAppContext();
  const [allowedUsersInputValue, setAllowedUsersInputValue] = useState('');
  const [allowedRolesInputValue, setAllowedRolesInputValue] = useState('');

  return (
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
      <Flex flexWrap="wrap" gap={2} justifyContent="space-between">
        <Flex alignItems="baseline" gap={2}>
          <ButtonPrimary
            onClick={async () => {
              const { filePaths, canceled } =
                await mainProcessClient.showDirectorySelectDialog();
              if (!canceled) {
                props.onChange({ ...fileShare, path: filePaths[0] });
              }
            }}
          >
            {fileShare.path ? 'Change directory' : 'Select & share'}
          </ButtonPrimary>
          {fileShare.path || 'No directory selected'}
        </Flex>
        {fileShare.path && (
          <Cross
            css={`
              cursor: pointer;
            `}
            onClick={() => {
              props.onChange({ ...fileShare, path: '' });
            }}
          />
        )}
      </Flex>
      <Flex flexDirection="column">
        <label>
          Share with
          <select
            name="allowAnyone"
            value={fileShare.sharingMode}
            onChange={event => {
              props.onChange({
                ...fileShare,
                sharingMode: event.target.value as SharingMode,
              });
            }}
            css={`
              margin-left: ${props => props.theme.space[1]}px;
            `}
          >
            <option value="specific-people">specific people</option>
            <option value="anyone">anyone</option>
          </select>
        </label>
        {fileShare.sharingMode === 'specific-people' && (
          <div>
            <label>
              <Text>Allow users</Text>
              <SelectCreatable
                inputValue={allowedUsersInputValue}
                onInputChange={setAllowedUsersInputValue}
                options={props.suggestedUsers.map(makeSelectOption)}
                onChange={users => {
                  props.onChange({
                    ...fileShare,
                    allowedUsers: (users || []).map(u => u.value),
                  });
                }}
                value={fileShare.allowedUsers.map(makeSelectOption)}
                formatCreateLabel={inputValue => `Add "${inputValue}"`}
              />
            </label>
            <label>
              <Text>Allow roles</Text>
              <SelectCreatable
                inputValue={allowedRolesInputValue}
                onInputChange={setAllowedRolesInputValue}
                options={props.suggestedRoles.map(makeSelectOption)}
                onChange={roles => {
                  props.onChange({
                    ...fileShare,
                    allowedRoles: (roles || []).map(r => r.value),
                  });
                }}
                value={fileShare.allowedRoles.map(makeSelectOption)}
                formatCreateLabel={inputValue => `Add "${inputValue}"`}
              />
            </label>
          </div>
        )}
      </Flex>
    </Flex>
  );
}

const makeSelectOption = (value: string): Option => ({ label: value, value });
