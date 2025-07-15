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

import { useCallback, useEffect, useState, type JSX } from 'react';
import styled from 'styled-components';

import { Alert, Box, ButtonPrimary, Flex, H1, Text } from 'design';
import * as Alerts from 'design/Alert';
import { Attempt, makeEmptyAttempt, useAsync } from 'shared/hooks/useAsync';
import { wait } from 'shared/utils/wait';

import { isTshdRpcError } from 'teleterm/services/tshd/cloneableClient';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import {
  AgentProcessError,
  NodeWaitJoinTimeout,
  useConnectMyComputerContext,
} from 'teleterm/ui/ConnectMyComputer';
import { useResourcesContext } from 'teleterm/ui/DocumentCluster/resourcesContext';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { DocumentConnectMyComputer } from 'teleterm/ui/services/workspacesService';
import { assertUnreachable, retryWithRelogin } from 'teleterm/ui/utils';
import { codeOrSignal } from 'teleterm/ui/utils/process';

import { ConnectMyComputerAccessNoAccess } from '../access';
import { CompatibilityError } from '../CompatibilityPromise';
import { Logs } from '../Logs';
import { useAgentProperties } from '../useAgentProperties';
import { ProgressBar } from './ProgressBar';

export function Setup(props: {
  updateDocumentStatus: (status: DocumentConnectMyComputer['status']) => void;
}) {
  const [step, setStep] = useState<'information' | 'agent-setup'>(
    'information'
  );

  return (
    <Box maxWidth="680px" mx="auto" mt="4" px="5" width="100%">
      <H1 mb="4">Connect My Computer</H1>
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
        <ClusterAndHostnameCopy clusterName={clusterName} hostname={hostname} />{' '}
        Cluster users with the role <strong>{roleName}</strong> and users with
        administrator privileges will be able to access your computer as{' '}
        <strong>{systemUsername}</strong>.
        <br />
        <br />
        Your device will be shared while Teleport Connect is open. To stop
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
  const { requestResourcesRefresh } = useResourcesContext(rootClusterUri);
  const rootCluster = ctx.clustersService.findCluster(rootClusterUri);

  // The verify agent step checks if we can execute the binary. This triggers OS-level checks, such
  // as Gatekeeper on macOS, before we do any real work. It is useful because it makes failures due
  // to OS protections be reported in telemetry as failures of the verify agent step.
  //
  // If we didn't have this check as a separate step, then the step with generating the config file
  // could run into Gatekeeper problems and that step can already fail for a myriad of other reasons.
  const [verifyAgentAttempt, runVerifyAgentAttempt, setVerifyAgentAttempt] =
    useAsync(
      useCallback(() => ctx.connectMyComputerService.verifyAgent(), [ctx])
    );

  const [createRoleAttempt, runCreateRoleAttempt, setCreateRoleAttempt] =
    useAsync(
      useCallback(
        () =>
          retryWithRelogin(ctx, rootClusterUri, async () => {
            let certsReloaded = false;

            try {
              const response =
                await ctx.connectMyComputerService.createRole(rootClusterUri);
              certsReloaded = response.certsReloaded;
            } catch (error) {
              if (
                isTshdRpcError(error, 'PERMISSION_DENIED') &&
                !error.isResolvableWithRelogin
              ) {
                throw new Error(
                  `Cannot set up the role: ${error.message}. Contact your administrator for permissions to manage users and roles.`
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
    useCallback(
      () =>
        retryWithRelogin(ctx, rootClusterUri, () =>
          ctx.connectMyComputerService.createAgentConfigFile(rootCluster)
        ),
      [rootCluster, ctx, rootClusterUri]
    )
  );

  const [joinClusterAttempt, runJoinClusterAttempt, setJoinClusterAttempt] =
    useAsync(
      useCallback(async () => {
        const [, error] = await startAgent();
        if (error) {
          throw error;
        }

        // Now that the node has joined the server, let's refresh open DocumentCluster
        // instances in the workspace to show the new node.
        requestResourcesRefresh();
      }, [startAgent, requestResourcesRefresh])
    );

  const steps: SetupStep[] = [
    {
      name: 'Downloading the agent',
      nameInFailureEvent: 'downloading_agent',
      attempt: downloadAgentAttempt,
      runAttempt: runDownloadAgentAttempt,
      setAttempt: setDownloadAgentAttempt,
    },
    {
      name: 'Verifying the agent',
      nameInFailureEvent: 'verifying_agent',
      attempt: verifyAgentAttempt,
      runAttempt: runVerifyAgentAttempt,
      setAttempt: setVerifyAgentAttempt,
    },
    {
      name: 'Setting up the role',
      nameInFailureEvent: 'setting_up_role',
      attempt: createRoleAttempt,
      runAttempt: runCreateRoleAttempt,
      setAttempt: setCreateRoleAttempt,
    },
    {
      name: 'Generating the config file',
      nameInFailureEvent: 'generating_config_file',
      attempt: generateConfigFileAttempt,
      runAttempt: runGenerateConfigFileAttempt,
      setAttempt: setGenerateConfigFileAttempt,
    },
    {
      name: 'Joining the cluster',
      nameInFailureEvent: 'joining_cluster',
      attempt: joinClusterAttempt,
      runAttempt: runJoinClusterAttempt,
      setAttempt: setJoinClusterAttempt,
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
    // all steps have to be cleared before starting the setup process;
    // otherwise we could see old errors on retry
    // (the error would be cleared when the given step starts, but it would be too late)
    for (const step of steps) {
      step.setAttempt(makeEmptyAttempt());
    }

    for (const step of steps) {
      const [, error] = await step.runAttempt();
      if (error) {
        ctx.usageService.captureConnectMyComputerSetup(rootCluster.uri, {
          success: false,
          failedStep: step.nameInFailureEvent,
        });
        // The error is reported by showing the attempt in the UI.
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
    // TODO(ravicious): We should run the steps only when every attempt has its status set to '' and
    // abort any action on unmount. However, there's a couple of things preventing us from doing so:
    //
    // * downloadAgentAttempt is kept in the context, so it's not reset between remounts like other
    // attempts from this component. Instead of re-using the attempt from the context, the step with
    // downloading an agent should be a separate attempt that merely calls downloadAgent from the
    // context.
    // * None of the steps support an abort signal at the moment.
    //
    // See the discussion on GitHub for more details:
    // https://github.com/gravitational/teleport/pull/37330#discussion_r1467646824
    runSteps();
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
  return <Alerts.Danger mb={props.mb || 0}>{props.error}</Alerts.Danger>;
}

function ClusterAndHostnameCopy(props: {
  clusterName: string;
  hostname: string;
}): JSX.Element {
  return (
    <>
      The setup process will make <strong>{props.hostname}</strong> available in
      the <strong>{props.clusterName}</strong> cluster as an SSH server.
    </>
  );
}

const Separator = styled(Box)`
  background: ${props => props.theme.colors.spotBackground[2]};
  height: 1px;
`;

type SetupStep = {
  name: string;
  nameInFailureEvent: string;
  attempt: Attempt<void>;
  runAttempt: () => Promise<[void, Error]>;
  setAttempt: (attempt: Attempt<void>) => void;
  customError?: () => JSX.Element;
};
