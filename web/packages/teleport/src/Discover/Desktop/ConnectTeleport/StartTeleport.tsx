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
import { Box, Text } from 'design';

import { ButtonPrimary } from 'design/Button';

import {
  StepContent,
  StepInstructions,
  StepTitle,
  StepTitleIcon,
} from 'teleport/Discover/Desktop/ConnectTeleport/Step';

import { usePingTeleport } from 'teleport/Discover/Shared/PingTeleportContext';
import { HintBox } from 'teleport/Discover/Shared/HintBox';
import { Mark, ResourceKind, useShowHint } from 'teleport/Discover/Shared';
import { useJoinTokenSuspender } from 'teleport/Discover/Shared/useJoinTokenSuspender';
import LogoHero from 'teleport/components/LogoHero';

interface StartTeleportProps {
  onNext: () => void;
}

interface StepWrapperProps {
  children?: React.ReactNode;
}

function StepWrapper(props: StepWrapperProps) {
  return (
    <StepContent>
      <StepTitle>
        <StepTitleIcon>
          <LogoHero />
        </StepTitleIcon>
        4. Start Teleport
      </StepTitle>

      {props.children}
    </StepContent>
  );
}

export function StartTeleport(
  props: React.PropsWithChildren<StartTeleportProps>
) {
  const { joinToken } = useJoinTokenSuspender([ResourceKind.Desktop]);
  const { active, result } = usePingTeleport(joinToken);

  const showHint = useShowHint(active);

  if (result) {
    return (
      <StepWrapper>
        <StepInstructions>
          <Text mb={4}>
            Success! We've detected the new Teleport node you configured.
          </Text>

          <ButtonPrimary onClick={() => props.onNext()}>Next</ButtonPrimary>
        </StepInstructions>
      </StepWrapper>
    );
  }

  let hint;
  if (showHint) {
    hint = (
      <Box mb={3}>
        <HintBox header="We're still looking for your Windows Desktop service">
          <Text mb={3}>
            There are a couple of possible reasons for why we haven't been able
            to detect your server.
          </Text>

          <Text mb={1}>
            - The command was not run on the server you were trying to add.
          </Text>

          <Text mb={3}>
            - The Teleport Desktop Service could not join this Teleport cluster.
            Check the logs for errors by running{' '}
            <Mark>journalctl -fu teleport</Mark>.
          </Text>

          <Text>
            We'll continue to look for the Windows Desktop service whilst you
            diagnose the issue.
          </Text>
        </HintBox>
      </Box>
    );
  }

  return (
    <StepWrapper>
      <StepInstructions>
        <Text mb={4}>Once you've started Teleport, we'll detect it here.</Text>

        {hint}

        <ButtonPrimary disabled={!result} onClick={() => props.onNext()}>
          Next
        </ButtonPrimary>
      </StepInstructions>
    </StepWrapper>
  );
}
