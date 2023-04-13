import React from 'react';

import Text from 'design/Text';
import Box from 'design/Box';

import { ButtonPrimary } from 'design';

import { InstructionsContainer } from './common';

import type { CommonInstructionsProps } from './common';

export function ThirdStageInstructions(props: CommonInstructionsProps) {
  return (
    <InstructionsContainer>
      <Text>
        Now click <strong>Add Provider</strong>
      </Text>

      <Text mt={5}>
        Select the Identity provider that you just created{' '}
        <strong>({props.clusterPublicUri})</strong>
      </Text>

      <Text mt={5}>
        Select <strong>Assign role</strong> and create a new role
      </Text>

      <Box mt={5}>
        <ButtonPrimary onClick={() => props.onNext()}>Next</ButtonPrimary>
      </Box>
    </InstructionsContainer>
  );
}
