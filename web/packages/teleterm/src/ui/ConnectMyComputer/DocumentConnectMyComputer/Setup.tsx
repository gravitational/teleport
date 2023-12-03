/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import React, { useCallback, useEffect, useRef, useState } from 'react';
import styled from 'styled-components';
import { Box, ButtonPrimary, Flex, Text, Alert } from 'design';
import { makeEmptyAttempt, useAsync } from 'shared/hooks/useAsync';
import { wait } from 'shared/utils/wait';
import * as Alerts from 'design/Alert';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { assertUnreachable, retryWithRelogin } from 'teleterm/ui/utils';
import {
  AgentProcessError,
  NodeWaitJoinTimeout,
  useConnectMyComputerContext,
} from 'teleterm/ui/ConnectMyComputer';
import { codeOrSignal } from 'teleterm/ui/utils/process';
import { isAccessDeniedError } from 'teleterm/services/tshd/errors';
import { useResourcesContext } from 'teleterm/ui/DocumentCluster/resourcesContext';
import { useLogger } from 'teleterm/ui/hooks/useLogger';
import { DocumentConnectMyComputer } from 'teleterm/ui/services/workspacesService';

import { useAgentProperties } from '../useAgentProperties';
import { Logs } from '../Logs';
import { CompatibilityError } from '../CompatibilityPromise';
import { ConnectMyComputerAccessNoAccess } from '../access';

import { ProgressBar } from './ProgressBar';

export function Setup(props: {
  updateDocumentStatus: (status: DocumentConnectMyComputer['status']) => void;
}) {
  const [step, setStep] = useState<'information' | 'agent-setup'>(
    'information'
  );

  return (
    <Box maxWidth="680px" mx="auto" mt="4" px="5" width="100%">
      <Text typography="h3" mb="4">
        Connect My Computer
      </Text>
      {step === 'information' && (
        <Information
          onSetUpAgentClick={() => setStep('agent-setup')}
          updateDocumentStatus={props.updateDocumentStatus}
        />
      )}
      {step === 'agent-setup' && <AgentSetup />}
    </Box>
  );
}

function Information(props: {
  onSetUpAgentClick(): void;
  updateDocumentStatus(status: DocumentConnectMyComputer['status']): void;
}) {
  const { updateDocumentStatus } = props;
  const { systemUsername, hostname, roleName, clusterName } =
    useAgentProperties();
  const { agentCompatibility, access } = useConnectMyComputerContext();

  let disabledButtonReason: string;
  if (access.status === 'unknown') {
    disabledButtonReason = 'Checking access…';
  } else if (access.status === 'no-access') {
    disabledButtonReason = "You don't have access to use Connect My Computer.";
  } else if (agentCompatibility === 'unknown') {
    disabledButtonReason = 'Checking agent compatibility…';
  } else if (agentCompatibility === 'incompatible') {
    disabledButtonReason =
      'The agent version is not compatible with the cluster version.';
  }

  const isWaiting =
    access.status === 'unknown' || agentCompatibility === 'unknown';

  useEffect(() => {
    if (isWaiting) {
      updateDocumentStatus('connecting');
    } else {
      updateDocumentStatus('connected');
    }
  }, [isWaiting, updateDocumentStatus]);

  let $alert: JSX.Element;
  if (access.status === 'no-access') {
    $alert = <AccessError access={access} />;
  } else if (agentCompatibility === 'incompatible') {
    $alert = <CompatibilityError />;
  }

  return (
    <>
      {$alert && (
        <>
          {$alert}
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
        Note that users with administrator privileges can assign that role to
        themselves or craft another role which grants access to the node. We
        recommend using Connect My Computer only in scenarios where no other
        user could plausibly gain access to the node, such as when exploring a
        Teleport cluster as its only user or in a home lab.
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
        title={disabledButtonReason}
        disabled={!!disabledButtonReason}
        onClick={props.onSetUpAgentClick}
        data-testid="start-setup"
      >
        Connect
      </ButtonPrimary>
    </>
  );
}

function AccessError(props: { access: ConnectMyComputerAccessNoAccess }) {
  const $documentation = (
    <>
      See{' '}
      <a
        href="https://goteleport.com/docs/connect-your-client/teleport-connect/#prerequisites"
        target="_blank"
      >
        the documentation
      </a>{' '}
      for more details.
    </>
  );

  switch (props.access.reason) {
    case 'unsupported-platform': {
      return (
        <Alert mb={0}>
          <Text>
            Connect My Computer is not supported on your operating system.
            <br />
            {$documentation}
          </Text>
        </Alert>
      );
    }
    case 'insufficient-permissions': {
      return (
        <Alert mb={0}>
          <Text>
            You have insufficient permissions to use Connect My Computer. Reach
            out to your Teleport administrator to request{' '}
            <a
              href="https://goteleport.com/docs/connect-your-client/teleport-connect/#prerequisites"
              target="_blank"
            >
              additional permissions
            </a>
            .
          </Text>
        </Alert>
      );
    }
    case 'sso-user': {
      return (
        <Alert mb={0}>
          <Text>
            Connect My Computer does not work with SSO users. {$documentation}
          </Text>
        </Alert>
      );
    }
    default: {
      return assertUnreachable(props.access);
    }
  }
}

function AgentSetup() {
  const logger = useLogger('AgentSetup');
  const ctx = useAppContext();
  const { mainProcessClient, notificationsService } = ctx;
  const { rootClusterUri } = useWorkspaceContext();
  const {
    startAgent,
    markAgentAsConfigured,
    downloadAgent: runDownloadAgentAttempt,
    downloadAgentAttempt,
    setDownloadAgentAttempt,
    agentProcessState,
  } = useConnectMyComputerContext();
  const { requestResourcesRefresh } = useResourcesContext();
  const rootCluster = ctx.clustersService.findCluster(rootClusterUri);
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
        ctx.connectMyComputerService.createAgentConfigFile(rootCluster)
      );
      nodeToken.current = token;
    }, [rootCluster, ctx, rootClusterUri])
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
            rootCluster.uri,
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
        rootCluster.uri,
        requestResourcesRefresh,
        logger,
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

  const runSteps = async () => {
    function withEventOnFailure(
      fn: () => Promise<[void, Error]>,
      failedStep: string
    ): () => Promise<[void, Error]> {
      return async () => {
        const result = await fn();
        const [, error] = result;
        if (error) {
          ctx.usageService.captureConnectMyComputerSetup(rootCluster.uri, {
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
    ctx.usageService.captureConnectMyComputerSetup(rootCluster.uri, {
      success: true,
    });
    // Wait before navigating away from the document, so the user has time
    // to notice that all four steps have completed.
    await wait(750);
    markAgentAsConfigured();
  };

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
  }, []);

  const retryRunSteps = async () => {
    try {
      // This will remove the binary but only if no other agents are running.
      //
      // Removing the binary is useful in situations where the download got corrupted or the OS
      // decided to ban the binary from being executed for some reason. In those cases,
      // redownloading the binary might resolve the problem.
      //
      // If other agents are running, then we at least know that there's probably no problems with
      // the binary itself, in which case we can simply ignore the fact that it wasn't removed and
      // carry on.
      await mainProcessClient.tryRemoveConnectMyComputerAgentBinary();
    } catch (error) {
      const { agentBinaryPath } = mainProcessClient.getRuntimeSettings();
      notificationsService.notifyError({
        title: 'Could not remove the agent binary',
        description: `Please try removing the binary manually to continue. The binary is at ${agentBinaryPath}. The error message was: ${error.message}`,
      });
      return;
    }

    await runSteps();
  };

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
        <ButtonPrimary alignSelf="center" onClick={retryRunSteps}>
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
