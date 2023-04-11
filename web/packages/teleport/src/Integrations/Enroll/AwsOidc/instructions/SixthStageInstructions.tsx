import React from 'react';

import Box from 'design/Box';
import Text from 'design/Text';

import { ButtonPrimary } from 'design';

import { InstructionsContainer } from './common';

import type { CommonInstructionsProps } from './common';

export function SixthStageInstructions(props: CommonInstructionsProps) {
  return (
    <InstructionsContainer>
      <Text>Close the "Create policy tab"</Text>

      <Text mt={5}>
        Refresh the list of policies and select the policy you just created
      </Text>

      <Text mt={5}>Search for the policy you just created and select it</Text>

      <Text mt={5}>
        Click <strong>Next: Tags</strong> and then <strong>Next: Review</strong>
      </Text>

      <Text mt={5}>
        Give the role a name and then click <strong>Create role</strong>
      </Text>

      <Box mt={5}>
        <ButtonPrimary onClick={() => props.onNext()}>Next</ButtonPrimary>
      </Box>
    </InstructionsContainer>
  );
}
