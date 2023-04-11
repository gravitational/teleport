import React from 'react';
import styled from 'styled-components';

import Box from 'design/Box';

import { ButtonPrimary } from 'design';

import { Stage } from '../stages';

import { InstructionsContainer } from './common';

import type { CommonInstructionsProps } from './common';

interface FirstStageInstructionsProps {
  stage: Stage;
}

const InstructionBlock = styled.div`
  margin-bottom: 30px;
`;

export function FirstStageInstructions(
  props: CommonInstructionsProps & FirstStageInstructionsProps
) {
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
