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
import styled from 'styled-components';
import { ButtonPrimary, Text, Box, LabelInput, Flex } from 'design';
import * as Icons from 'design/Icon';
import Select from 'shared/components/Select';

import { Header, ActionButtons, TextIcon } from '../Shared';
import { useDiscoverContext } from '../discoverContextProvider';

import { useTestConnection, State } from './useTestConnection';

import type { Option } from 'shared/components/Select';
import type { AgentStepProps } from '../types';

export default function Container(props: AgentStepProps) {
  const ctx = useDiscoverContext();
  const state = useTestConnection({ ctx, props });

  return <TestConnection {...state} />;
}

export function TestConnection({
  attempt,
  startSshSession,
  logins,
  runConnectionDiagnostic,
  diagnosis,
}: State) {
  const [usernameOpts] = useState(() =>
    logins.map(l => ({ value: l, label: l }))
  );
  // There will always be one login, as the user cannot proceed
  // the step that requires users to have at least one login.
  const [selectedOpt, setSelectedOpt] = useState(usernameOpts[0]);

  let $diagnosisStateComponent;
  if (attempt.status === 'processing') {
    $diagnosisStateComponent = (
      <TextIcon>
        <Icons.Restore fontSize={4} />
        Testing in-progress
      </TextIcon>
    );
  }

  let diagnosisStateBorderColor = 'transparent';
  if (attempt.status === 'failed' || (diagnosis && !diagnosis.success)) {
    diagnosisStateBorderColor = 'danger';
    $diagnosisStateComponent = (
      <TextIcon>
        <Icons.Warning ml={1} color="danger" />
        Testing failed
      </TextIcon>
    );
  } else if (attempt.status === 'success' && diagnosis?.success) {
    diagnosisStateBorderColor = 'success';
    $diagnosisStateComponent = (
      <TextIcon>
        <Icons.CircleCheck ml={1} color="success" />
        Testing complete
      </TextIcon>
    );
  }

  const showDiagnosisOutput = !!diagnosis || attempt.status === 'failed';

  return (
    <Box>
      <Header>Test Connection</Header>
      <StyledBox mb={5}>
        <Text bold mb={3}>
          Step 1
        </Text>
        <Box width="250px">
          <LabelInput>Select OS Username</LabelInput>
          <Select
            value={selectedOpt}
            options={usernameOpts}
            onChange={(o: Option) => setSelectedOpt(o)}
            isDisabled={attempt.status === 'processing'}
          />
        </Box>
      </StyledBox>
      <StyledBox mb={5}>
        <Text bold mb={3}>
          Step 2
        </Text>
        <Flex alignItems="center">
          <ButtonPrimary
            width="200px"
            onClick={() => runConnectionDiagnostic(selectedOpt.value)}
            disabled={attempt.status === 'processing'}
          >
            {diagnosis ? 'Restart Test' : 'Test Connection'}
          </ButtonPrimary>
          <Box ml={4}>{$diagnosisStateComponent}</Box>
        </Flex>
        {showDiagnosisOutput && (
          <Box
            mt={4}
            bg="rgba(255, 255, 255, 0.05)"
            p={3}
            borderRadius={3}
            border={2}
            borderColor={diagnosisStateBorderColor}
          >
            <Text bold>Output</Text>
            {attempt.status === 'failed' &&
              `Failed to Start Testing: ${attempt.statusText}`}
            {attempt.status === 'success' && <Box>{diagnosis.message}</Box>}
          </Box>
        )}
      </StyledBox>
      <StyledBox>
        <Text bold mb={3}>
          Step 3
        </Text>
        <ButtonPrimary
          width="200px"
          onClick={() => startSshSession(selectedOpt.value)}
          disabled={attempt.status !== 'success' || !diagnosis?.success}
        >
          Start SSH Session
        </ButtonPrimary>
      </StyledBox>
      <ActionButtons />
    </Box>
  );
}

const StyledBox = styled(Box)`
  max-width: 800px;
  background-color: rgba(255, 255, 255, 0.05);
  border-radius: 8px;
  padding: 20px;
`;
