import React from 'react';

import * as Icons from 'design/Icon';
import { ButtonPrimary } from 'design/Button';
import { Text } from 'design';

import {
  StepContent,
  StepInstructions,
  StepTitle,
  StepTitleIcon,
} from 'teleport/Discover/Desktop/ConnectTeleport/Step';

interface CopyOutputProps {
  onNext: () => void;
}

export function CopyOutput(props: React.PropsWithChildren<CopyOutputProps>) {
  return (
    <StepContent>
      <StepTitle>
        <StepTitleIcon>
          <Icons.Clipboard />
        </StepTitleIcon>
        2. Copy the outputted Teleport config
      </StepTitle>

      <StepInstructions>
        <Text mb={4}>You'll need this in the next step.</Text>

        <ButtonPrimary onClick={() => props.onNext()}>Next</ButtonPrimary>
      </StepInstructions>
    </StepContent>
  );
}
