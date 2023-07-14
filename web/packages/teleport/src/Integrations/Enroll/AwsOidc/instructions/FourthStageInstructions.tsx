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

import Text from 'design/Text';
import Box from 'design/Box';

import { ButtonPrimary, ButtonSecondary } from 'design';

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
        <ButtonSecondary ml={3} onClick={() => props.onPrev()}>
          Back
        </ButtonSecondary>
      </Box>
    </InstructionsContainer>
  );
}
