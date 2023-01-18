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
