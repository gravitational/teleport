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

import React, { useCallback, useRef, type JSX } from 'react';
import { Transition } from 'react-transition-group';
import styled, { css } from 'styled-components';

import {
  Alert,
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  H1,
  Label,
  MenuItem,
  Text,
} from 'design';
import * as icons from 'design/Icon';
import type { IconProps } from 'design/Icon/Icon';
import Indicator from 'design/Indicator';
import { MenuIcon } from 'shared/components/MenuAction';

import { makeLabelTag } from 'teleport/components/formatters';
import type * as tsh from 'teleterm/services/tshd/types';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import {
  AgentProcessError,
  CurrentAction,
  NodeWaitJoinTimeout,
  useConnectMyComputerContext,
} from 'teleterm/ui/ConnectMyComputer';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { connectToServer } from 'teleterm/ui/services/workspacesService';
import { assertUnreachable } from 'teleterm/ui/utils';
import { codeOrSignal } from 'teleterm/ui/utils/process';

import { CompatibilityError, useVersions } from '../CompatibilityPromise';
import { Logs } from '../Logs';
import {
  shouldShowAgentUpgradeSuggestion,
  UpgradeAgentSuggestion,
} from '../UpgradeAgentSuggestion';
import { useAgentProperties } from '../useAgentProperties';

export function Status(props: { closeDocument?: () => void }) {
  const ctx = useAppContext();
  const {
    currentAction,
    agentNode,
    downloadAndStartAgent,
    killAgent,
    isAgentConfiguredAttempt,
    markAgentAsNotConfigured,
    removeAgent,
    agentCompatibility,
  } = useConnectMyComputerContext();
  const { rootClusterUri } = useWorkspaceContext();
  const { roleName, systemUsername, hostname } = useAgentProperties();
  const { proxyVersion, appVersion, isLocalBuild } = useVersions();
  const isAgentIncompatible = agentCompatibility === 'incompatible';
  const isAgentIncompatibleOrUnknown =
    agentCompatibility === 'incompatible' || agentCompatibility === 'unknown';
  const downloadAndStartAgentAndIgnoreErrors = useCallback(async () => {
    try {
      await downloadAndStartAgent();
    } catch {
      // Ignore the error, it'll be shown in the UI by inspecting the attempts.
    }
  }, [downloadAndStartAgent]);

  const prettyCurrentAction = prettifyCurrentAction(currentAction);

  function replaceWithSetup(): void {
    markAgentAsNotConfigured();
  }

  function startSshSession(): void {
    connectToServer(
      ctx,
      { uri: agentNode.uri, hostname, login: systemUsername },
      { origin: 'resource_table' }
    );
  }

  async function removeAgentAndClose(): Promise<void> {
    const [, error] = await removeAgent();
    if (error) {
      return;
    }
    props.closeDocument();
  }

  async function openAgentLogs(): Promise<void> {
    try {
      await ctx.mainProcessClient.openAgentLogsDirectory({ rootClusterUri });
    } catch (e) {
      ctx.notificationsService.notifyError({
        title: 'Failed to open agent logs directory',
        description: `${e.message}\n\nNote: the logs directory is created only after the agent process successfully spawns.`,
      });
    }
  }

  const isRunning =
    currentAction.kind === 'observe-process' &&
    currentAction.agentProcessState.status === 'running';
  const isKilling =
    currentAction.kind === 'kill' &&
    currentAction.attempt.status === 'processing';
  const isDownloading =
    currentAction.kind === 'download' &&
    currentAction.attempt.status === 'processing';
  const isStarting =
    currentAction.kind === 'start' &&
    currentAction.attempt.status === 'processing';
  const isRemoving =
    currentAction.kind === 'remove' &&
    currentAction.attempt.status === 'processing';
  const isRemoved =
    currentAction.kind === 'remove' &&
    currentAction.attempt.status === 'success';

  const showConnectAndStopAgentButtons = isRunning || isKilling;
  const disableConnectAndStopAgentButtons = isKilling;
  const disableStartAgentButton =
    isDownloading ||
    isStarting ||
    isRemoving ||
    isRemoved ||
    isAgentIncompatibleOrUnknown;

  const transitionRef = useRef<HTMLDivElement>(null);

  return (
    <Box maxWidth="680px" mx="auto" mt="4" px="5" width="100%">
      {shouldShowAgentUpgradeSuggestion(proxyVersion, {
        appVersion,
        isLocalBuild,
      }) && (
        <UpgradeAgentSuggestion
          proxyVersion={proxyVersion}
          appVersion={appVersion}
        />
      )}
      {isAgentConfiguredAttempt.status === 'error' && (
        <Alert
          primaryAction={{
            content: 'Run setup again',
            onClick: replaceWithSetup,
          }}
          details={isAgentConfiguredAttempt.statusText}
        >
          An error occurred while reading the agent config file
        </Alert>
      )}

      <Flex flexDirection="column" gap={3}>
        <Flex flexDirection="column" gap={1}>
          <Flex justifyContent="space-between">
            <H1
              css={`
                display: flex;
              `}
            >
              <icons.Laptop mr={2} />
              {/** The node name can be changed, so it might be different from the system hostname. */}
              {agentNode?.hostname || hostname}
            </H1>
            <MenuIcon
              Icon={icons.MoreVert}
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
              <MenuItem onClick={openAgentLogs}>
                Open agent logs directory
              </MenuItem>
              <MenuItem onClick={removeAgentAndClose}>Remove agent</MenuItem>
            </MenuIcon>
          </Flex>

          <Transition
            in={!!agentNode}
            nodeRef={transitionRef}
            timeout={1_800}
            mountOnEnter
            unmountOnExit
          >
            {state => (
              <LabelsContainer gap={1} className={state} ref={transitionRef}>
                {/* Explicitly check for existence of agentNode because Transition doesn't seem to
                unmount immediately when `in` becomes falsy. */}
                {agentNode?.labels && renderLabels(agentNode.labels)}
              </LabelsContainer>
            )}
          </Transition>
        </Flex>

        <Flex flexDirection="column" gap={2}>
          <Flex gap={1} alignItems="center" minHeight="32px">
            {prettyCurrentAction.Icon && (
              <prettyCurrentAction.Icon size="medium" />
            )}
            {prettyCurrentAction.title}
            {showConnectAndStopAgentButtons && (
              <ButtonSecondary
                onClick={killAgent}
                disabled={disableConnectAndStopAgentButtons}
                ml={3}
              >
                Stop Agent
              </ButtonSecondary>
            )}
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

        <Flex flexDirection="column" gap={2}>
          {isAgentIncompatible ? (
            <CompatibilityError
              // Hide the alert if the current action has failed. downloadAgent and startAgent already
              // return an error message related to compatibility.
              //
              // Basically, we have to cover two use cases:
              //
              // * Auto start has failed due to compatibility promise, so the downloadAgent failed with
              // an error.
              // * Auto start wasn't enabled, so the current action has no errors, but the user should
              // not be able to start the agent due to compatibility issues.
              hideAlert={!!prettyCurrentAction.error}
            />
          ) : (
            <>
              {isRunning ? (
                <Text>
                  Cluster users with the role <strong>{roleName}</strong> and
                  users with administrator privileges can now access your
                  computer as <strong>{systemUsername}</strong>.
                </Text>
              ) : (
                <Text>
                  Starting the agent will allow clusters users with the role{' '}
                  <strong>{roleName}</strong> and users with administrator
                  privileges to access it as an SSH resource as the user{' '}
                  <strong>{systemUsername}</strong>.
                </Text>
              )}
              {showConnectAndStopAgentButtons ? (
                <ButtonPrimary
                  block
                  disabled={disableConnectAndStopAgentButtons}
                  onClick={startSshSession}
                  size="large"
                >
                  Connect
                </ButtonPrimary>
              ) : (
                <ButtonPrimary
                  block
                  disabled={disableStartAgentButton}
                  onClick={downloadAndStartAgentAndIgnoreErrors}
                  size="large"
                  data-testid="start-agent"
                >
                  Start Agent
                </ButtonPrimary>
              )}
            </>
          )}
        </Flex>
      </Flex>
    </Box>
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

function prettifyCurrentAction(currentAction: CurrentAction): {
  Icon: React.FC<IconProps>;
  title: string;
  error?: string;
  logs?: string;
} {
  const noop = {
    Icon: StyledIndicator,
    title: '',
  };

  switch (currentAction.kind) {
    case 'download': {
      switch (currentAction.attempt.status) {
        case '':
        case 'processing': {
          // TODO(gzdunek) add progress
          return {
            Icon: StyledIndicator,
            title: 'Checking agent version',
          };
        }
        case 'error': {
          return {
            Icon: StyledWarning,
            title: 'Failed to verify agent binary',
            error: currentAction.attempt.statusText,
          };
        }
        case 'success': {
          return noop; // noop, not used, at this point it should be start processing.
        }
        default: {
          return assertUnreachable(currentAction.attempt);
        }
      }
    }
    case 'start': {
      switch (currentAction.attempt.status) {
        case '':
        case 'processing': {
          return {
            Icon: StyledIndicator,
            title: 'Starting',
          };
        }
        case 'error': {
          if (currentAction.attempt.error instanceof NodeWaitJoinTimeout) {
            return {
              Icon: StyledWarning,
              title: 'Failed to start agent',
              error:
                'The agent did not join the cluster within the timeout window.',
              logs: currentAction.attempt.error.logs,
            };
          }

          if (!(currentAction.attempt.error instanceof AgentProcessError)) {
            return {
              Icon: StyledWarning,
              title: 'Failed to start agent',
              error: currentAction.attempt.statusText,
            };
          }

          if (currentAction.agentProcessState.status === 'error') {
            return {
              Icon: StyledWarning,
              title:
                'Failed to start agent – an error occurred while spawning the agent process',
              error: currentAction.agentProcessState.message,
            };
          }

          if (currentAction.agentProcessState.status === 'exited') {
            const { code, signal } = currentAction.agentProcessState;
            return {
              Icon: StyledWarning,
              title: `Failed to start agent - the agent process quit unexpectedly with ${codeOrSignal(
                code,
                signal
              )}`,
              logs: currentAction.agentProcessState.logs,
            };
          }
          break;
        }
        case 'success': {
          return noop; // noop, not used, at this point it should be observe-process running.
        }
        default: {
          return assertUnreachable(currentAction.attempt);
        }
      }
      break;
    }
    case 'observe-process': {
      switch (currentAction.agentProcessState.status) {
        case 'not-started': {
          return {
            Icon: icons.Moon,
            title: 'Agent not running',
          };
        }
        case 'running': {
          return {
            Icon: props => (
              <icons.CircleCheck {...props} color="success.main" />
            ),
            title: 'Agent running',
          };
        }
        case 'exited': {
          const { code, signal, exitedSuccessfully } =
            currentAction.agentProcessState;

          if (exitedSuccessfully) {
            return {
              Icon: icons.Moon,
              title: 'Agent not running',
            };
          } else {
            return {
              Icon: StyledWarning,
              title: `Agent process exited with ${codeOrSignal(code, signal)}`,
              logs: currentAction.agentProcessState.logs,
            };
          }
        }
        case 'error': {
          // TODO(ravicious): This can happen only just before killing the process. 'error' should
          // not be considered a separate process state. See the comment above the 'error' status
          // definition.
          return {
            Icon: StyledWarning,
            title: 'An error occurred to agent process',
            error: currentAction.agentProcessState.message,
          };
        }
        default: {
          return assertUnreachable(currentAction.agentProcessState);
        }
      }
    }
    case 'kill': {
      switch (currentAction.attempt.status) {
        case '':
        case 'processing': {
          return {
            Icon: StyledIndicator,
            title: 'Stopping',
          };
        }
        case 'error': {
          return {
            Icon: StyledWarning,
            title: 'Failed to stop agent',
            error: currentAction.attempt.statusText,
          };
        }
        case 'success': {
          return noop; // noop, not used, at this point it should be observe-process exited.
        }
        default: {
          return assertUnreachable(currentAction.attempt);
        }
      }
    }
    case 'remove': {
      switch (currentAction.attempt.status) {
        case '':
        case 'processing': {
          return {
            Icon: StyledIndicator,
            title: 'Removing',
          };
        }
        case 'error': {
          return {
            Icon: StyledWarning,
            title: 'Failed to remove agent',
            error: currentAction.attempt.statusText,
          };
        }
        case 'success': {
          return {
            Icon: icons.CircleCheck,
            title: 'Agent removed',
            error: currentAction.attempt.statusText,
          };
        }
        default: {
          return assertUnreachable(currentAction.attempt);
        }
      }
    }
  }
}

const StyledWarning = styled(icons.Warning).attrs({
  color: 'error.main',
})``;

const StyledIndicator = styled(Indicator).attrs({ delay: 'none' })`
  color: inherit;
  display: inline-flex;
`;

const LabelsContainer = styled(Flex).attrs({ flexWrap: 'wrap' })`
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
