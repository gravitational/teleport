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

import React, { useCallback, useEffect, useRef, useState } from 'react';
import { Box, ButtonPrimary, Flex, Text } from 'design';
import { makeEmptyAttempt, useAsync } from 'shared/hooks/useAsync';
import * as Alerts from 'design/Alert';
import { CircleCheck, CircleCross, CirclePlay, Spinner } from 'design/Icon';

import * as types from 'teleterm/ui/services/workspacesService';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import Document from 'teleterm/ui/Document';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { retryWithRelogin } from 'teleterm/ui/utils';
import { useConnectMyComputerContext } from 'teleterm/ui/ConnectMyComputer';
import Logger from 'teleterm/logger';

interface DocumentConnectMyComputerSetupProps {
  visible: boolean;
  doc: types.DocumentConnectMyComputerSetup;
}

const logger = new Logger('DocumentConnectMyComputerSetup');

export function DocumentConnectMyComputerSetup(
  props: DocumentConnectMyComputerSetupProps
) {
  const [step, setStep] = useState<'information' | 'agent-setup'>(
    'information'
  );

  return (
    <Document visible={props.visible}>
      <Box maxWidth="590px" mx="auto" mt="4" px="5" width="100%">
        <Text typography="h3" mb="4">
          Connect My Computer
        </Text>
        {step === 'information' && (
          <Information onSetUpAgentClick={() => setStep('agent-setup')} />
        )}
        {step === 'agent-setup' && <AgentSetup />}
      </Box>
    </Document>
  );
}

function Information(props: { onSetUpAgentClick(): void }) {
  const { rootClusterUri } = useWorkspaceContext();
  const { clustersService, mainProcessClient } = useAppContext();
  const cluster = clustersService.findCluster(rootClusterUri);
  const { username: systemUsername, hostname } =
    mainProcessClient.getRuntimeSettings();

  return (
    <>
      <Text>
        The setup process will download and launch the Teleport agent, making
        your computer available in the <strong>{cluster.name}</strong> cluster
        as <strong>{hostname}</strong>.
        <br />
        <br />
        Cluster users with the role{' '}
        <strong>connect-my-computer-{cluster.loggedInUser.name}</strong> will be
        able to access your computer as <strong>{systemUsername}</strong>.
        <br />
        <br />
        Your computer will be shared while Teleport Connect is open. To stop
        sharing, close Teleport Connect or stop the agent through the Connect My
        Computer tab. Sharing will resume on app restart, unless you stop the
        agent before exiting.
      </Text>
      <ButtonPrimary
        mt={4}
        mx="auto"
        css={`
          display: block;
        `}
        onClick={props.onSetUpAgentClick}
      >
        Connect
      </ButtonPrimary>
    </>
  );
}

function AgentSetup() {
  const ctx = useAppContext();
  const { rootClusterUri } = useWorkspaceContext();
  const { runAgentAndWaitForNodeToJoin } = useConnectMyComputerContext();
  const cluster = ctx.clustersService.findCluster(rootClusterUri);
  const nodeToken = useRef<string>();

  const [createRoleAttempt, runCreateRoleAttempt, setCreateRoleAttempt] =
    useAsync(
      useCallback(async () => {
        retryWithRelogin(ctx, rootClusterUri, async () => {
          let certsReloaded = false;

          try {
            const response = await ctx.connectMyComputerService.createRole(
              rootClusterUri
            );
            certsReloaded = response.certsReloaded;
          } catch (error) {
            if (isAccessDeniedError(error)) {
              throw new Error(
                'Access denied. Contact your administrator for permissions to manage users and roles.'
              );
            }
            throw error;
          }

          // If tshd reloaded the certs to refresh the role list, the Electron app must resync details
          // of the cluster to also update the role list in the UI.
          if (certsReloaded) {
            await ctx.clustersService.syncRootCluster(rootClusterUri);
          }
        });
      }, [ctx, rootClusterUri])
    );
  const [
    downloadAgentAttempt,
    runDownloadAgentAttempt,
    setDownloadAgentAttempt,
  ] = useAsync(
    useCallback(
      () => ctx.connectMyComputerService.downloadAgent(),
      [ctx.connectMyComputerService]
    )
  );
  const [
    generateConfigFileAttempt,
    runGenerateConfigFileAttempt,
    setGenerateConfigFileAttempt,
  ] = useAsync(
    useCallback(async () => {
      const { token } = await retryWithRelogin(ctx, rootClusterUri, () =>
        ctx.connectMyComputerService.createAgentConfigFile(cluster)
      );
      nodeToken.current = token;
    }, [cluster, ctx, rootClusterUri])
  );
  const [joinClusterAttempt, runJoinClusterAttempt, setJoinClusterAttempt] =
    useAsync(
      useCallback(async () => {
        if (!nodeToken.current) {
          throw new Error('Node token is empty');
        }
        await runAgentAndWaitForNodeToJoin();
        try {
          await ctx.connectMyComputerService.deleteToken(
            cluster.uri,
            nodeToken.current
          );
        } catch (error) {
          // the user may not have permissions to remove the token, but it will expire in a few minutes anyway
          if (isAccessDeniedError(error)) {
            logger.error('Access denied when deleting a token.', error);
          }
          throw error;
        }
      }, [
        runAgentAndWaitForNodeToJoin,
        ctx.connectMyComputerService,
        cluster.uri,
      ])
    );

  const steps = [
    {
      name: 'Setting up the role',
      attempt: createRoleAttempt,
    },
    {
      name: 'Downloading the agent',
      attempt: downloadAgentAttempt,
    },
    {
      name: 'Generating the config file',
      attempt: generateConfigFileAttempt,
    },
    {
      name: 'Joining the cluster',
      attempt: joinClusterAttempt,
    },
  ];

  const runSteps = useCallback(async () => {
    setCreateRoleAttempt(makeEmptyAttempt());
    setDownloadAgentAttempt(makeEmptyAttempt());
    setGenerateConfigFileAttempt(makeEmptyAttempt());
    setJoinClusterAttempt(makeEmptyAttempt());

    const actions = [
      runCreateRoleAttempt,
      runDownloadAgentAttempt,
      runGenerateConfigFileAttempt,
      runJoinClusterAttempt,
    ];
    for (const action of actions) {
      const [, error] = await action();
      if (error) {
        break;
      }
    }
  }, [
    setCreateRoleAttempt,
    setDownloadAgentAttempt,
    setGenerateConfigFileAttempt,
    setJoinClusterAttempt,
    runCreateRoleAttempt,
    runDownloadAgentAttempt,
    runGenerateConfigFileAttempt,
    runJoinClusterAttempt,
  ]);

  useEffect(() => {
    if (
      [
        createRoleAttempt,
        downloadAgentAttempt,
        generateConfigFileAttempt,
        joinClusterAttempt,
      ].every(attempt => attempt.status === '')
    ) {
      runSteps();
    }
  }, [
    downloadAgentAttempt,
    generateConfigFileAttempt,
    joinClusterAttempt,
    createRoleAttempt,
    runSteps,
  ]);

  const hasSetupFailed = steps.some(s => s.attempt.status === 'error');

  return (
    <>
      <ol
        css={`
          padding-left: 0;
          list-style: inside decimal;
        `}
      >
        {steps.map(step => (
          <Flex key={step.name} alignItems="baseline" gap={2}>
            {step.attempt.status === '' && <CirclePlay />}
            {step.attempt.status === 'processing' && (
              <Spinner
                css={`
                  animation: spin 1s linear infinite;
                  @keyframes spin {
                    from {
                      transform: rotate(0deg);
                    }
                    to {
                      transform: rotate(360deg);
                    }
                  }
                `}
              />
            )}
            {step.attempt.status === 'success' && (
              <CircleCheck color="success" />
            )}
            {step.attempt.status === 'error' && (
              <CircleCross color="error.main" />
            )}
            <li>
              {step.name}
              {step.attempt.status === 'error' && (
                <Alerts.Danger
                  mb={0}
                  css={`
                    white-space: pre-wrap;
                  `}
                >
                  {step.attempt.statusText}
                </Alerts.Danger>
              )}
            </li>
          </Flex>
        ))}
      </ol>

      {hasSetupFailed && (
        <ButtonPrimary onClick={runSteps}>Retry</ButtonPrimary>
      )}
    </>
  );
}

function isAccessDeniedError(error: Error): boolean {
  return (error.message as string)?.includes('access denied');
}
