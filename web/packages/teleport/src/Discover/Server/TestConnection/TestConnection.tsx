/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useState } from 'react';
import { ButtonSecondary, Text, Box, LabelInput } from 'design';
import Select from 'shared/components/Select';

import cfg from 'teleport/config';
import ReAuthenticate from 'teleport/components/ReAuthenticate';
import { openNewTab } from 'teleport/lib/util';
import {
  useConnectionDiagnostic,
  Header,
  ActionButtons,
  HeaderSubtitle,
  ConnectionDiagnosticResult,
  StyledBox,
} from 'teleport/Discover/Shared';
import { sortNodeLogins } from 'teleport/services/nodes';

import { NodeMeta } from '../../useDiscover';

import type { Option } from 'shared/components/Select';
import type { AgentStepProps } from '../../types';
import type { MfaAuthnResponse } from 'teleport/services/mfa';

export function TestConnection(props: AgentStepProps) {
  const {
    runConnectionDiagnostic,
    attempt,
    diagnosis,
    nextStep,
    prevStep,
    canTestConnection,
    showMfaDialog,
    cancelMfaDialog,
  } = useConnectionDiagnostic();
  const nodeMeta = props.agentMeta as NodeMeta;
  const logins = sortNodeLogins(nodeMeta.node.sshLogins);

  function startSshSession(login: string) {
    const url = cfg.getSshConnectRoute({
      clusterId: nodeMeta.node.clusterId,
      serverId: nodeMeta.node.id,
      login,
    });

    openNewTab(url);
  }

  function testConnection(login: string, mfaResponse?: MfaAuthnResponse) {
    runConnectionDiagnostic(
      {
        resourceKind: 'node',
        resourceName: props.agentMeta.resourceName,
        sshPrincipal: login,
      },
      mfaResponse
    );
  }

  const usernameOpts = logins.map(l => ({ value: l, label: l }));
  // There will always be one login, as the user cannot proceed
  // the step that requires users to have at least one login.
  const [selectedOpt, setSelectedOpt] = useState(usernameOpts[0]);

  return (
    <Box>
      {showMfaDialog && (
        <ReAuthenticate
          onMfaResponse={res => testConnection(selectedOpt.value, res)}
          onClose={cancelMfaDialog}
        />
      )}
      <Header>Test Connection</Header>
      <HeaderSubtitle>
        Optionally verify that you can successfully connect to the server you
        just added.
      </HeaderSubtitle>
      <StyledBox mb={5}>
        <Text bold>Step 1</Text>
        <Text typography="subtitle1" mb={3}>
          Pick the OS user to test
        </Text>
        <Box width="320px">
          <LabelInput>Select Login</LabelInput>
          <Select
            value={selectedOpt}
            options={usernameOpts}
            onChange={(o: Option) => setSelectedOpt(o)}
            isDisabled={attempt.status === 'processing'}
          />
        </Box>
      </StyledBox>
      <ConnectionDiagnosticResult
        attempt={attempt}
        diagnosis={diagnosis}
        canTestConnection={canTestConnection}
        testConnection={() => testConnection(selectedOpt.value)}
        stepNumber={2}
        stepDescription="Verify that the server is accessible"
      />
      <StyledBox>
        <Text bold>Step 3</Text>
        <Text typography="subtitle1" mb={3}>
          Connect to the server
        </Text>
        <ButtonSecondary
          width="200px"
          onClick={() => startSshSession(selectedOpt.value)}
        >
          Start Session
        </ButtonSecondary>
      </StyledBox>
      <ActionButtons onProceed={nextStep} lastStep={true} onPrev={prevStep} />
    </Box>
  );
}
