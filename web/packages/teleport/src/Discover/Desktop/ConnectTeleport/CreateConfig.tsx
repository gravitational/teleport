/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import * as Icons from 'design/Icon';
import { Text } from 'design';
import { ButtonPrimary } from 'design/Button';

import {
  StepContent,
  StepInstructions,
  StepTitle,
  StepTitleIcon,
} from 'teleport/Discover/Desktop/ConnectTeleport/Step';

interface EditConfigProps {
  onNext: () => void;
}

export function CreateConfig(props: React.PropsWithChildren<EditConfigProps>) {
  return (
    <StepContent>
      <StepTitle>
        <StepTitleIcon>
          <Icons.Code />
        </StepTitleIcon>
        3. Create /etc/teleport.yaml
      </StepTitle>

      <StepInstructions>
        <Text mb={4}>
          Paste the output you just copied into /etc/teleport.yaml.
        </Text>

        <ButtonPrimary onClick={() => props.onNext()}>Next</ButtonPrimary>
      </StepInstructions>
    </StepContent>
  );
}
