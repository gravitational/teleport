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
import { ButtonSecondary, Text, Box, LabelInput, Flex } from 'design';
import * as Icons from 'design/Icon';
import Select from 'shared/components/Select';

import useTeleport from 'teleport/useTeleport';

import {
  HeaderWithBackBtn,
  ActionButtons,
  TextIcon,
  HeaderSubtitle,
  Mark,
  ReadOnlyYamlEditor,
} from '../../Shared';
import { ruleConnectionDiagnostic } from '../../templates';

import { useTestConnection, State } from './useTestConnection';

import type { Option } from 'shared/components/Select';
import type { AgentStepProps } from '../../types';

export default function Container(props: AgentStepProps) {
  const ctx = useTeleport();
  const state = useTestConnection({ ctx, props });

  return <TestConnection {...state} />;
}

export function TestConnection({
  attempt,
  startSshSession,
  logins,
  runConnectionDiagnostic,
  diagnosis,
  nextStep,
  prevStep,
  canTestConnection,
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
  } else if (attempt.status === 'failed' || (diagnosis && !diagnosis.success)) {
    $diagnosisStateComponent = (
      <TextIcon>
        <Icons.Warning ml={1} color="danger" />
        Testing failed
      </TextIcon>
    );
  } else if (attempt.status === 'success' && diagnosis?.success) {
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
      <HeaderWithBackBtn onPrev={prevStep}>Test Connection</HeaderWithBackBtn>
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
      <StyledBox mb={5}>
        <Text bold>Step 2</Text>
        <Text typography="subtitle1" mb={3}>
          Verify that the server is accessible
        </Text>
        <Flex alignItems="center" mt={3}>
          {canTestConnection ? (
            <>
              <ButtonSecondary
                width="200px"
                onClick={() => runConnectionDiagnostic(selectedOpt.value)}
                disabled={attempt.status === 'processing'}
              >
                {diagnosis ? 'Restart Test' : 'Test Connection'}
              </ButtonSecondary>
              <Box ml={4}>{$diagnosisStateComponent}</Box>
            </>
          ) : (
            <Box>
              <Text>
                You don't have permission to test connection.
                <br />
                Please ask your Teleport administrator to update your role and
                add the <Mark>connection_diagnostic</Mark> rule:
              </Text>
              <Flex minHeight="155px" mt={3}>
                <ReadOnlyYamlEditor content={ruleConnectionDiagnostic} />
              </Flex>
            </Box>
          )}
        </Flex>
        {showDiagnosisOutput && (
          <Box mt={3}>
            {attempt.status === 'failed' &&
              `Encountered Error: ${attempt.statusText}`}
            {attempt.status === 'success' && (
              <Box>
                {diagnosis.traces.map((trace, index) => {
                  if (trace.status === 'failed') {
                    return (
                      <>
                        <TextIcon alignItems="baseline">
                          <Icons.CircleCross mr={1} color="danger" />
                          {trace.details}
                          <br />
                          {trace.error}
                        </TextIcon>
                      </>
                    );
                  }
                  if (trace.status === 'success') {
                    return (
                      <TextIcon key={index}>
                        <Icons.CircleCheck mr={1} color="success" />
                        {trace.details}
                      </TextIcon>
                    );
                  }

                  // For whatever reason the status is not the value
                  // of failed or success.
                  return (
                    <TextIcon key={index}>
                      <Icons.Question mr={1} />
                      {trace.details}
                    </TextIcon>
                  );
                })}
              </Box>
            )}
          </Box>
        )}
      </StyledBox>
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
      <ActionButtons onProceed={nextStep} lastStep={true} />
    </Box>
  );
}

const StyledBox = styled(Box)`
  max-width: 800px;
  background-color: rgba(255, 255, 255, 0.05);
  border-radius: 8px;
  padding: 20px;
`;
