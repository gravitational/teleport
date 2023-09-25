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
import styled, { useTheme } from 'styled-components';
import { Flex, Box } from 'design';
import { Check, Warning, Refresh } from 'design/Icon';
import { decomposeColor, emphasize } from 'design/theme/utils/colorManipulator';
import { AttemptStatus } from 'shared/hooks/useAsync';

interface ProgressBarProps {
  phases: {
    status: AttemptStatus;
    name: string;
    Error: React.ElementType;
  }[];
}

export function ProgressBar({ phases }: ProgressBarProps): JSX.Element {
  return (
    <Flex flexDirection="column">
      {phases.map((phase, index) => (
        <Flex
          py="12px"
          key={phase.name}
          style={{ position: 'relative' }}
          data-testid={phase.name}
          data-teststatus={phase.status}
        >
          <Phase status={phase.status} isLast={index === phases.length - 1} />
          <div>
            {index + 1}. {phase.name}
            <phase.Error />
          </div>
        </Flex>
      ))}
    </Flex>
  );
}

function Phase({
  status,
  isLast,
}: {
  status: AttemptStatus;
  isLast: boolean;
}): JSX.Element {
  const theme = useTheme();
  // we have to use a solid color here; otherwise
  // <StyledLine> would be visible through the component
  const phaseSolidColor = getPhaseSolidColor(theme);
  let bg = phaseSolidColor;

  if (status === 'success') {
    bg = theme.colors.success;
  }

  if (status === 'error') {
    bg = theme.colors.error.main;
  }

  return (
    <>
      <StyledPhase mr="3" bg={bg}>
        <PhaseIcon status={status} />
      </StyledPhase>
      {!isLast && (
        <StyledLine
          color={status === 'success' ? theme.colors.success : phaseSolidColor}
        />
      )}
    </>
  );
}

const StyledPhase = styled(Box)`
  display: flex;
  flex-shrink: 0;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  z-index: 1;
  justify-content: center;
  align-items: center;
`;

const StyledLine = styled(Box)`
  width: 0;
  position: absolute;
  left: 11px;
  bottom: -12px;
  border: 1px solid;
  height: 100%;
`;

function PhaseIcon({ status }: { status: AttemptStatus }): JSX.Element {
  if (status === 'success') {
    return <Check size="small" color="white" />;
  }

  if (status === 'error') {
    return <Warning size="small" color="white" />;
  }

  if (status === 'processing') {
    return (
      <Refresh
        size="extraLarge"
        color="success"
        css={`
          animation: anim-rotate 1.5s infinite linear;
          @keyframes anim-rotate {
            0% {
              transform: rotate(0);
            }
            100% {
              transform: rotate(360deg);
            }
        `}
      />
    );
  }

  return (
    <Box
      borderRadius="50%"
      css={`
        background: ${props => props.theme.colors.spotBackground[1]};
      `}
      as="span"
      height="14px"
      width="14px"
    />
  );
}

function getPhaseSolidColor(theme: any): string {
  const alpha = decomposeColor(theme.colors.spotBackground[1]).values[3] || 0;
  return emphasize(theme.colors.levels.surface, alpha);
}
