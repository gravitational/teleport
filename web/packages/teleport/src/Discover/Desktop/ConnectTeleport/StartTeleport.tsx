import React from 'react';
import styled from 'styled-components';

import logoSrc from 'design/assets/images/teleport-medallion.svg';

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
          <TeleportIcon />
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
  const { joinToken } = useJoinTokenSuspender(ResourceKind.Desktop);
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
            <Mark>journalctl status teleport</Mark>.
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

const TeleportIcon = styled.div`
  width: 30px;
  height: 30px;
  background: url(${logoSrc}) no-repeat;
  background-size: contain;
  top: 1px;
  position: relative;
`;
