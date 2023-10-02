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
import styled from 'styled-components';
import { Box, ButtonPrimary, Flex, Text } from 'design';
import { makeEmptyAttempt, useAsync } from 'shared/hooks/useAsync';
import { wait } from 'shared/utils/wait';
import * as Alerts from 'design/Alert';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { retryWithRelogin } from 'teleterm/ui/utils';
import {
  AgentProcessError,
  NodeWaitJoinTimeout,
  useConnectMyComputerContext,
} from 'teleterm/ui/ConnectMyComputer';
import Logger from 'teleterm/logger';
import { codeOrSignal } from 'teleterm/ui/utils/process';
import { RootClusterUri } from 'teleterm/ui/uri';
import { isAccessDeniedError } from 'teleterm/services/tshd/errors';
import { useResourcesContext } from 'teleterm/ui/DocumentCluster/resourcesContext';

import { useAgentProperties } from '../useAgentProperties';
import { Logs } from '../Logs';
import { CompatibilityError } from '../CompatibilityPromise';

import { ProgressBar } from './ProgressBar';

const logger = new Logger('DocumentConnectMyComputerSetup');

// TODO(gzdunek): Rename to `Setup`
export function DocumentConnectMyComputerSetup() {
  const [step, setStep] = useState<'information' | 'agent-setup'>(
    'information'
  );
  const { rootClusterUri } = useWorkspaceContext();

  return (
    <Box maxWidth="680px" mx="auto" mt="4" px="5" width="100%">
      <Text typography="h3" mb="4">
        Connect My Computer
      </Text>
      {step === 'information' && (
        <Information onSetUpAgentClick={() => setStep('agent-setup')} />
      )}
      {step === 'agent-setup' && <AgentSetup rootClusterUri={rootClusterUri} />}
    </Box>
  );
}

function Information(props: { onSetUpAgentClick(): void }) {
  const { systemUsername, hostname, roleName, clusterName } =
    useAgentProperties();
  const { agentCompatibility } = useConnectMyComputerContext();
  const isAgentIncompatible = agentCompatibility === 'incompatible';
  const isAgentIncompatibleOrUnknown =
    agentCompatibility === 'incompatible' || agentCompatibility === 'unknown';

  return (
    <>
      {isAgentIncompatible && (
        <>
          <CompatibilityError />
          <Separator mt={3} mb={2} />
        </>
      )}
      <Text>
        Connect My Computer allows you to add this device to the Teleport
        cluster with just a few clicks.{' '}
        <ClusterAndHostnameCopy clusterName={clusterName} hostname={hostname} />
        <br />
        <br />
        Cluster users with the role <strong>{roleName}</strong> will be able to
        access your computer as <strong>{systemUsername}</strong>.
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
        disabled={isAgentIncompatibleOrUnknown}
        onClick={props.onSetUpAgentClick}
        data-testid="start-setup"
      >
        Connect
      </ButtonPrimary>
    </>
  );
}

function AgentSetup({ rootClusterUri }: { rootClusterUri: RootClusterUri }) {
  const ctx = useAppContext();
  const {
    startAgent,
    markAgentAsConfigured,
    downloadAgent: runDownloadAgentAttempt,
    downloadAgentAttempt,
    setDownloadAgentAttempt,
    agentProcessState,
  } = useConnectMyComputerContext();
  const { requestResourcesRefresh } = useResourcesContext();
  const cluster = ctx.clustersService.findCluster(rootClusterUri);
  const nodeToken = useRef<string>();

  const [createRoleAttempt, runCreateRoleAttempt, setCreateRoleAttempt] =
    useAsync(
      useCallback(
        () =>
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
          }),
        [ctx, rootClusterUri]
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
        const [, error] = await startAgent();
        if (error) {
          throw error;
        }

        // Now that the node has joined the server, let's refresh all open DocumentCluster instances
        // to show the new node.
        requestResourcesRefresh();

        try {
          await ctx.connectMyComputerService.deleteToken(
            cluster.uri,
            nodeToken.current
          );
        } catch (error) {
          // the user may not have permissions to remove the token, but it will expire in a few minutes anyway
          if (isAccessDeniedError(error)) {
            logger.error('Access denied when deleting a token.', error);
            return;
          }
          throw error;
        }
      }, [
        startAgent,
        ctx.connectMyComputerService,
        cluster.uri,
        requestResourcesRefresh,
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
      customError: () => {
        if (joinClusterAttempt.status !== 'error') {
          return;
        }

        if (joinClusterAttempt.error instanceof NodeWaitJoinTimeout) {
          return (
            <>
              <StandardError
                error={
                  'The agent did not join the cluster within the timeout window.'
                }
                mb={1}
              />
              <Logs logs={joinClusterAttempt.error.logs} />
            </>
          );
        }

        if (!(joinClusterAttempt.error instanceof AgentProcessError)) {
          return <StandardError error={joinClusterAttempt.statusText} />;
        }

        if (agentProcessState.status === 'error') {
          return <StandardError error={agentProcessState.message} />;
        }

        if (agentProcessState.status === 'exited') {
          const { code, signal } = agentProcessState;

          return (
            <>
              <StandardError
                error={`Agent process exited with ${codeOrSignal(
                  code,
                  signal
                )}.`}
                mb={1}
              />
              <Logs logs={agentProcessState.logs} />
            </>
          );
        }
      },
    },
  ];

  const runSteps = useCallback(async () => {
    function withEventOnFailure(
      fn: () => Promise<[void, Error]>,
      failedStep: string
    ): () => Promise<[void, Error]> {
      return async () => {
        const result = await fn();
        const [, error] = result;
        if (error) {
          ctx.usageService.captureConnectMyComputerSetup(cluster.uri, {
            success: false,
            failedStep,
          });
        }
        return result;
      };
    }

    // all steps have to be cleared when starting the setup process;
    // otherwise we could see old errors on retry
    // (the error would be cleared when the given step starts, but it would be too late)
    setCreateRoleAttempt(makeEmptyAttempt());
    setDownloadAgentAttempt(makeEmptyAttempt());
    setGenerateConfigFileAttempt(makeEmptyAttempt());
    setJoinClusterAttempt(makeEmptyAttempt());

    const actions = [
      withEventOnFailure(runCreateRoleAttempt, 'setting_up_role'),
      withEventOnFailure(runDownloadAgentAttempt, 'downloading_agent'),
      withEventOnFailure(
        runGenerateConfigFileAttempt,
        'generating_config_file'
      ),
      withEventOnFailure(runJoinClusterAttempt, 'joining_cluster'),
    ];
    for (const action of actions) {
      const [, error] = await action();
      if (error) {
        return;
      }
    }
    ctx.usageService.captureConnectMyComputerSetup(cluster.uri, {
      success: true,
    });
    // Wait before navigating away from the document, so the user has time
    // to notice that all four steps have completed.
    await wait(750);
    markAgentAsConfigured();
  }, [
    setCreateRoleAttempt,
    setDownloadAgentAttempt,
    setGenerateConfigFileAttempt,
    setJoinClusterAttempt,
    runCreateRoleAttempt,
    runDownloadAgentAttempt,
    runGenerateConfigFileAttempt,
    runJoinClusterAttempt,
    markAgentAsConfigured,
    ctx.usageService,
    cluster.uri,
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
  const { clusterName, hostname } = useAgentProperties();

  return (
    <Flex flexDirection="column" alignItems="flex-start" gap={3}>
      <Text>
        <ClusterAndHostnameCopy clusterName={clusterName} hostname={hostname} />
      </Text>
      <ProgressBar
        phases={steps.map(step => ({
          status: step.attempt.status,
          name: step.name,
          Error: () =>
            step.attempt.status === 'error' &&
            (step.customError?.() || (
              <StandardError error={step.attempt.statusText} />
            )),
        }))}
      />
      {hasSetupFailed && (
        <ButtonPrimary alignSelf="center" onClick={runSteps}>
          Retry
        </ButtonPrimary>
      )}
    </Flex>
  );
}

function StandardError(props: {
  error: string;
  mb?: number | string;
}): JSX.Element {
  return (
    <Alerts.Danger
      mb={props.mb || 0}
      css={`
        white-space: pre-wrap;
      `}
    >
      {props.error}
    </Alerts.Danger>
  );
}

function ClusterAndHostnameCopy(props: {
  clusterName: string;
  hostname: string;
}): JSX.Element {
  return (
    <>
      The setup process will download and launch a Teleport agent, making your
      computer available in the <strong>{props.clusterName}</strong> cluster as{' '}
      <strong>{props.hostname}</strong>.
    </>
  );
}

const Separator = styled(Box)`
  background: ${props => props.theme.colors.spotBackground[2]};
  height: 1px;
`;
