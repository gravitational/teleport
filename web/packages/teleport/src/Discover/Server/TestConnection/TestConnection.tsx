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

import { useState } from 'react';

import { Box, ButtonSecondary, H3, LabelInput, Subtitle3 } from 'design';
import Select, { type Option } from 'shared/components/Select';

import ReAuthenticate from 'teleport/components/ReAuthenticate';
import cfg from 'teleport/config';
import {
  ActionButtons,
  ConnectionDiagnosticResult,
  Header,
  HeaderSubtitle,
  StyledBox,
  useConnectionDiagnostic,
} from 'teleport/Discover/Shared';
import { openNewTab } from 'teleport/lib/util';
import { MfaChallengeScope } from 'teleport/services/auth/auth';
import type { MfaChallengeResponse } from 'teleport/services/mfa';
import { sortNodeLogins } from 'teleport/services/nodes';

import type { AgentStepProps } from '../../types';
import { NodeMeta } from '../../useDiscover';

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
  const node = (props.agentMeta as NodeMeta).node;
  const logins = sortNodeLogins(node.sshLogins);

  function startSshSession(login: string) {
    const url = cfg.getSshConnectRoute({
      clusterId: node.clusterId,
      serverId: node.id,
      login,
    });

    openNewTab(url);
  }

  function testConnection(login: string, mfaResponse?: MfaChallengeResponse) {
    runConnectionDiagnostic(
      {
        resourceKind: 'node',
        resourceName: props.agentMeta.resourceName,
        sshPrincipal: login,
        sshNodeOS: props.resourceSpec.platform,
        sshNodeSetupMethod: 'script',
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
          onMfaResponse={async res => testConnection(selectedOpt.value, res)}
          onClose={cancelMfaDialog}
          challengeScope={MfaChallengeScope.USER_SESSION}
        />
      )}
      <Header>Test Connection</Header>
      <HeaderSubtitle>
        Optionally verify that you can successfully connect to the server you
        just added.
      </HeaderSubtitle>
      <StyledBox mb={5}>
        <header>
          <H3>Step 1</H3>
          <Subtitle3 mb={3}>Pick the OS user to test</Subtitle3>
        </header>
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
        <header>
          <H3>Step 3</H3>
          <Subtitle3 mb={3}>Connect to the server</Subtitle3>
        </header>
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
