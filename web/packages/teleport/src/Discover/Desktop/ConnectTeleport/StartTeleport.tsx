import React, { useEffect, useState } from 'react';
import styled from 'styled-components';

import logoSrc from 'design/assets/images/teleport-medallion.svg';

import { Box, Flex, Text } from 'design';

import { ButtonPrimary } from 'design/Button';

import * as Icons from 'design/Icon';

import {
  StepContent,
  StepInstructions,
  StepTitle,
  StepTitleIcon,
} from 'teleport/Discover/Desktop/ConnectTeleport/Step';

import { usePingTeleport } from 'teleport/Discover/Shared/PingTeleportContext';
import { HintBox } from 'teleport/Discover/Shared/HintBox';
import { Mark, TextIcon } from 'teleport/Discover/Shared';

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

const SHOW_HINT_TIMEOUT = 1000 * 60 * 5; // 5 minutes

export function StartTeleport(
  props: React.PropsWithChildren<StartTeleportProps>
) {
  const { active, result } = usePingTeleport();

  const [showHint, setShowHint] = useState(false);

  useEffect(() => {
    if (active) {
      const id = window.setTimeout(() => setShowHint(true), SHOW_HINT_TIMEOUT);

      return () => window.clearTimeout(id);
    }
  }, [active]);

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
        <HintBox>
          <Text color="warning">
            <Flex alignItems="center" mb={2}>
              <TextIcon
                color="warning"
                css={`
                  white-space: pre;
                `}
              >
                <Icons.Warning fontSize={4} color="warning" />
              </TextIcon>
              We're still looking for your Windows Desktop service
            </Flex>
          </Text>

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
