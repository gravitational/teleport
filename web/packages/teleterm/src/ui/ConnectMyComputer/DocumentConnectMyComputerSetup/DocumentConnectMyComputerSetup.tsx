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

import React, { useCallback, useEffect, useState } from 'react';
import { Box, ButtonPrimary, Flex, Text } from 'design';
import { useAsync } from 'shared/hooks/useAsync';
import { wait } from 'shared/utils/wait';
import * as Alerts from 'design/Alert';
import { CircleCheck, CircleCross, CirclePlay, Spinner } from 'design/Icon';

import * as types from 'teleterm/ui/services/workspacesService';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import Document from 'teleterm/ui/Document';

interface DocumentConnectMyComputerSetupProps {
  visible: boolean;
  doc: types.DocumentConnectMyComputerSetup;
}

export function DocumentConnectMyComputerSetup(
  props: DocumentConnectMyComputerSetupProps
) {
  const [step, setStep] =
    useState<'information' | 'agent-setup'>('information');

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
  return (
    <>
      {/* TODO(gzdunek): change the temporary copy */}
      <Text>
        The setup process will download the teleport agent and configure it to
        make your computer available in the <strong>acme.teleport.sh</strong>{' '}
        cluster.
        <br />
        All cluster users with the role{' '}
        <strong>connect-my-computer-robert@acme.com</strong> will be able to
        access your computer as <strong>bob</strong>.
        <br />
        You can stop computer sharing at any time from Connect My Computer menu.
      </Text>
      <ButtonPrimary
        mt={4}
        mx="auto"
        css={`
          display: block;
        `}
        onClick={props.onSetUpAgentClick}
      >
        Set up agent
      </ButtonPrimary>
    </>
  );
}

function AgentSetup() {
  const ctx = useAppContext();

  const [setUpRolesAttempt, runSetUpRolesAttempt] = useAsync(
    useCallback(() => wait(1_000), [])
  );
  const [downloadAgentAttempt, runDownloadAgentAttempt] = useAsync(
    useCallback(
      () => ctx.connectMyComputerService.downloadAgent(),
      [ctx.connectMyComputerService]
    )
  );
  const [generateConfigFileAttempt, runGenerateConfigFileAttempt] = useAsync(
    useCallback(() => wait(1_000), [])
  );
  const [joinClusterAttempt, runJoinClusterAttempt] = useAsync(
    useCallback(() => wait(1_000), [])
  );

  const steps = [
    {
      name: 'Setting up roles',
      attempt: setUpRolesAttempt,
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
    // uncomment when implemented
    const actions = [
      // runSetUpRolesAttempt,
      runDownloadAgentAttempt,
      // runGenerateConfigFileAttempt,
      // runJoinClusterAttempt,
    ];
    for (const action of actions) {
      const [, error] = await action();
      if (error) {
        break;
      }
    }
  }, [
    runSetUpRolesAttempt,
    runDownloadAgentAttempt,
    runGenerateConfigFileAttempt,
    runJoinClusterAttempt,
  ]);

  useEffect(() => {
    if (
      [
        setUpRolesAttempt,
        downloadAgentAttempt,
        generateConfigFileAttempt,
        joinClusterAttempt,
      ].every(attempt => attempt.status === '')
    ) {
      runSteps();
    }
  }, [downloadAgentAttempt, generateConfigFileAttempt, joinClusterAttempt, setUpRolesAttempt, runSteps]);

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
                <Alerts.Danger mb={0}>{step.attempt.statusText}</Alerts.Danger>
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
