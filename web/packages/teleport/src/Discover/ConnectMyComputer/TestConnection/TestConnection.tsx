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

import {
  Box,
  ButtonPrimary,
  ButtonSecondary,
  H3,
  Indicator,
  LabelInput,
  Link,
  Subtitle3,
  Text,
} from 'design';
import * as Icons from 'design/Icon';
import { P } from 'design/Text/Text';
import Select, { type Option } from 'shared/components/Select';
import * as connectMyComputer from 'shared/connectMyComputer';
import { useAsync } from 'shared/hooks/useAsync';

import ReAuthenticate from 'teleport/components/ReAuthenticate';
import cfg from 'teleport/config';
import {
  ActionButtons,
  ConnectionDiagnosticResult,
  Header,
  HeaderSubtitle,
  StyledBox,
  TextIcon,
  useConnectionDiagnostic,
} from 'teleport/Discover/Shared';
import { openNewTab } from 'teleport/lib/util';
import type { ConnectionDiagnosticRequest } from 'teleport/services/agents';
import { ApiError } from 'teleport/services/api/parseError';
import { MfaChallengeScope } from 'teleport/services/auth/auth';
import type { MfaChallengeResponse } from 'teleport/services/mfa';
import { sortNodeLogins } from 'teleport/services/nodes';
import useTeleport from 'teleport/useTeleport';

import type { AgentStepProps } from '../../types';
import { NodeMeta } from '../../useDiscover';

export function TestConnection(props: AgentStepProps) {
  const { userService, storeUser } = useTeleport();
  const {
    runConnectionDiagnostic,
    attempt: connectionDiagAttempt,
    diagnosis,
    nextStep,
    canTestConnection,
    showMfaDialog,
    cancelMfaDialog,
  } = useConnectionDiagnostic();
  const node = (props.agentMeta as NodeMeta).node;
  const [selectedLoginOpt, setSelectedLoginOpt] = useState<Option>();

  const abortController = useRef<AbortController>();
  // When the user sets up Connect My Computer in Teleport Connect, a new role gets added to the
  // user. Because of that, we need to reload the current session so that the user is able to
  // connect to the new node, without having to log in to the cluster again.
  //
  // We also need to fetch the list of logins from that role. The user might have many logins
  // available, but the Connect My Computer agent is always started by the system user that is
  // running Connect. As such, the Connect My Computer role should include that valid login.
  const [fetchLoginsAttempt, fetchLogins] = useAsync(
    useCallback(
      async (signal: AbortSignal) => {
        await userService.reloadUser(signal);

        return await userService.fetchConnectMyComputerLogins(signal);
      },
      [userService]
    )
  );

  const fetchLoginsAndUpdateLoginSelection = async (signal: AbortSignal) => {
    // fetchLogins is from useAsync which always resolves the underlying promise and uses a Go-style
    // API for error handling.
    const [logins, err] = await fetchLogins(signal);
    if (err) {
      return;
    }

    if (logins.length > 0) {
      setSelectedLoginOpt(mapLoginToSelectOption(logins[0]));

      // Start the test automatically if there are no logins to choose from.
      if (logins.length == 1) {
        const { mfaRequired } = await testConnection({
          login: logins[0],
          sshPrincipalSelectionMode: 'auto',
        });

        // If MFA is required, let's just wait for the user to start the connection themselves.
        if (mfaRequired) {
          cancelMfaDialog();
        }
      }
    }
  };

  useEffect(() => {
    abortController.current = new AbortController();

    if (fetchLoginsAttempt.status === '') {
      fetchLoginsAndUpdateLoginSelection(abortController.current.signal);
    }

    return () => {
      abortController.current.abort();
    };
  }, []);

  function startSshSession(login: string) {
    const url = cfg.getSshConnectRoute({
      clusterId: node.clusterId,
      serverId: node.id,
      login,
    });

    openNewTab(url);
  }

  function testConnection(args: {
    login: string;
    sshPrincipalSelectionMode: ConnectionDiagnosticRequest['sshPrincipalSelectionMode'];
    mfaResponse?: MfaChallengeResponse;
  }) {
    return runConnectionDiagnostic(
      {
        resourceKind: 'node',
        resourceName: props.agentMeta.resourceName,
        sshNodeSetupMethod: 'connect_my_computer',
        sshPrincipal: args.login,
        sshPrincipalSelectionMode: args.sshPrincipalSelectionMode,
      },
      args.mfaResponse
    );
  }

  const hasMultipleLogins =
    fetchLoginsAttempt.status === 'success' &&
    fetchLoginsAttempt.data.length > 1;
  // If there are multiple logins available, we show an extra step at the beginning, so we have to
  // account for that when numbering the steps.
  const stepOffset = hasMultipleLogins ? 1 : 0;
  const sshPrincipalSelectionMode = hasMultipleLogins ? 'manual' : 'auto';

  return (
    <Box>
      {showMfaDialog && (
        <ReAuthenticate
          onMfaResponse={async res => {
            await testConnection({
              login: selectedLoginOpt.value,
              sshPrincipalSelectionMode,
              mfaResponse: res,
            });
          }}
          onClose={cancelMfaDialog}
          challengeScope={MfaChallengeScope.USER_SESSION}
        />
      )}
      <Header>
        Test Connection to &ldquo;{props.agentMeta.resourceName}&rdquo;
      </Header>
      <HeaderSubtitle>
        Optionally verify that you can connect to the computer you just added.
      </HeaderSubtitle>

      {(fetchLoginsAttempt.status === '' ||
        fetchLoginsAttempt.status === 'processing') && <Indicator />}

      {fetchLoginsAttempt.status === 'error' &&
        (fetchLoginsAttempt.error instanceof ApiError &&
        fetchLoginsAttempt.error.response.status === 404 ? (
          <FetchLoginsAttemptError
            buttonText="Refresh"
            buttonOnClick={() => window.location.reload()}
          >
            <P>
              For Connect My Computer to work, the role{' '}
              {connectMyComputer.getRoleNameForUser(storeUser.getUsername())}{' '}
              must be assigned to you.
            </P>
            <P>{$restartSetupInstructions}</P>
          </FetchLoginsAttemptError>
        ) : (
          <FetchLoginsAttemptError
            buttonText="Retry"
            buttonOnClick={() =>
              fetchLoginsAndUpdateLoginSelection(abortController.current.signal)
            }
          >
            Encountered Error: {fetchLoginsAttempt.statusText}
          </FetchLoginsAttemptError>
        ))}

      {fetchLoginsAttempt.status === 'success' &&
        (fetchLoginsAttempt.data.length === 0 ? (
          <FetchLoginsAttemptError
            buttonText="Refresh"
            buttonOnClick={() => window.location.reload()}
          >
            The role{' '}
            {connectMyComputer.getRoleNameForUser(storeUser.getUsername())} does
            not contain any logins. It has likely been manually edited.
            <br />
            {$restartSetupInstructions}
          </FetchLoginsAttemptError>
        ) : (
          <>
            {hasMultipleLogins && (
              <StepSkeletonPickUser>
                <Box width="320px">
                  <LabelInput>Select Login</LabelInput>
                  <Select
                    value={selectedLoginOpt}
                    options={sortNodeLogins(fetchLoginsAttempt.data).map(
                      mapLoginToSelectOption
                    )}
                    onChange={(o: Option) => setSelectedLoginOpt(o)}
                    isDisabled={connectionDiagAttempt.status === 'processing'}
                  />
                </Box>
              </StepSkeletonPickUser>
            )}
            <ConnectionDiagnosticResult
              attempt={connectionDiagAttempt}
              diagnosis={diagnosis}
              canTestConnection={canTestConnection}
              testConnection={() =>
                testConnection({
                  login: selectedLoginOpt.value,
                  sshPrincipalSelectionMode,
                })
              }
              stepNumber={1 + stepOffset}
              stepDescription="Verify that your computer is accessible"
              numberAndDescriptionOnSameLine
            />
            <StyledBox>
              <Text bold mb={3}>
                Step {2 + stepOffset}: Connect to Your Computer
              </Text>
              <ButtonSecondary
                width="200px"
                onClick={() => startSshSession(selectedLoginOpt.value)}
              >
                Start Session
              </ButtonSecondary>
            </StyledBox>
          </>
        ))}
      <ActionButtons
        disableProceed={
          fetchLoginsAttempt.status !== 'success' ||
          fetchLoginsAttempt.data.length === 0
        }
        onProceed={nextStep}
        lastStep={true}
        // onPrev is not passed on purpose to disable the back button. The flow would go back to
        // polling which wouldn't make sense as the user has already connected their computer so the
        // step would poll forever, unless the user removed the agent and configured it again.
      />
    </Box>
  );
}

const StepSkeletonPickUser = (props: { children: React.ReactNode }) => (
  <StyledBox mb={5}>
    <header>
      <H3>Step 1</H3>
      <Subtitle3 mb={3}>Pick the OS user to test</Subtitle3>
    </header>
    {props.children}
  </StyledBox>
);

/**
 * To display the first step, fetchLoginsAttempt must be successful, hence why we show errors from
 * fetchLoginsAttempt within the first step.
 */
const FetchLoginsAttemptError = (props: {
  children: React.ReactNode;
  buttonText: string;
  buttonOnClick: () => void;
}) => (
  <StepSkeletonPickUser>
    <TextIcon mt={2} mb={3}>
      <Icons.Warning size="medium" ml={1} mr={2} color="error.main" />
      <Text>{props.children}</Text>
    </TextIcon>

    <ButtonPrimary type="button" onClick={props.buttonOnClick}>
      {props.buttonText}
    </ButtonPrimary>
  </StepSkeletonPickUser>
);

const mapLoginToSelectOption = (login: string) => ({
  value: login,
  label: login,
});

const $restartSetupInstructions = (
  <>
    Refresh this page to repeat the process of enrolling a new resource and then{' '}
    <Link
      href="https://goteleport.com/docs/connect-your-client/teleport-connect/#restarting-the-setup"
      target="_blank"
    >
      restart the Connect My Computer setup
    </Link>{' '}
    in Teleport Connect.
  </>
);
