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

import Box from 'design/Box';
import Text from 'design/Text';

import { ButtonPrimary, ButtonSecondary } from 'design';

import { InstructionsContainer } from './common';

import type { CommonInstructionsProps } from './common';

export function SixthStageInstructions(props: CommonInstructionsProps) {
  return (
    <InstructionsContainer>
      <Text>
        Close the tab for "Create Policy" and go back to the tab for{' '}
        <strong>Create Role</strong>
      </Text>

      <Text mt={5}>
        Refresh the list of policies and select the policy you just created
      </Text>

      <Text mt={5}>
        Click <strong>Next: Tags</strong> and then <strong>Next: Review</strong>
      </Text>

      <Text mt={5}>
        Give the role a name and then click <strong>Create role</strong>
      </Text>

      <Box mt={5}>
        <ButtonPrimary onClick={() => props.onNext()}>Next</ButtonPrimary>
        <ButtonSecondary ml={3} onClick={() => props.onPrev()}>
          Back
        </ButtonSecondary>
      </Box>
    </InstructionsContainer>
  );
}
