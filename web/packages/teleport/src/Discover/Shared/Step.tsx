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

import React from 'react';
import styled from 'styled-components';

import { Text } from 'design';

import TextSelectCopy from 'teleport/components/TextSelectCopy';

interface StepProps {
  stepNumber?: number;
  title: string;
  text: string;
  isBash?: boolean;
}

export function Step(props: StepProps) {
  let prefix;
  if (props.stepNumber) {
    prefix = `Step ${props.stepNumber}: `;
  }

  return (
    <StepContainer>
      <Text bold>
        {prefix}
        {props.title}
      </Text>

      <TextSelectCopy text={props.text} mt={2} mb={1} bash={props.isBash} />
    </StepContainer>
  );
}

export const StepContainer = styled.div`
  background: ${props => props.theme.colors.spotBackground[0]};
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 12px;
`;
