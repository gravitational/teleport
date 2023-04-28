/**
 * Copyright 2023 Gravitational, Inc.
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

import React from 'react';
import styled from 'styled-components';

import Box from 'design/Box';

import { ButtonPrimary } from 'design';

import { InstructionsContainer } from './common';

import type { CommonInstructionsProps } from './common';

const InstructionBlock = styled.div`
  margin-bottom: 30px;
`;

export function FirstStageInstructions(props: CommonInstructionsProps) {
  return (
    <InstructionsContainer>
      <InstructionBlock>
        To connect Teleport to AWS as an identity provider, go to the{' '}
        <strong>AWS Management Console</strong>
      </InstructionBlock>
      <InstructionBlock>
        Search for <strong>IAM</strong>, and then click on{' '}
        <strong>Identity providers</strong>
      </InstructionBlock>
      <InstructionBlock>
        After that, click on <strong>Add Provider</strong>
      </InstructionBlock>

      <Box mt={5}>
        <ButtonPrimary onClick={() => props.onNext()}>Next</ButtonPrimary>
      </Box>
    </InstructionsContainer>
  );
}
