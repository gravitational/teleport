import React from 'react';

import Text from 'design/Text';
import Box from 'design/Box';

import { ButtonPrimary } from 'design';

import { InstructionsContainer } from './common';

import type { CommonInstructionsProps } from './common';

export function FourthStageInstructions(props: CommonInstructionsProps) {
  return (
    <InstructionsContainer>
      <Text>
        Select <strong>discover.teleport</strong> as the audience for the role
      </Text>

      <Text mt={5}>
        Then click on <strong>Next: Permissions</strong>
      </Text>

      <Text mt={5}>
        From the permissions page, click on <strong>Create policy</strong>
      </Text>

      <Box mt={5}>
        <ButtonPrimary onClick={() => props.onNext()}>Next</ButtonPrimary>
      </Box>
    </InstructionsContainer>
  );
}
