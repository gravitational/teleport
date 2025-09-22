/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import React, { type JSX } from 'react';
import styled, { useTheme } from 'styled-components';

import { Box, Flex, rotate360 } from 'design';
import * as icons from 'design/Icon';
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
    bg = theme.colors.success.main;
  }

  if (status === 'error') {
    bg = theme.colors.error.main;
  }

  return (
    <>
      <StyledPhase
        mr="3"
        bg={bg}
        css={`
          position: relative;
        `}
      >
        <PhaseIcon status={status} />
      </StyledPhase>
      {!isLast && (
        <StyledLine
          color={
            status === 'success' ? theme.colors.success.main : phaseSolidColor
          }
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
    return <icons.Check size="small" color="white" />;
  }

  if (status === 'error') {
    return <icons.Warning size="small" color="white" />;
  }

  if (status === 'processing') {
    return (
      <>
        <Spinner />
        <icons.Restore size="small" color="buttons.text" />
      </>
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

const Spinner = styled.div`
  opacity: 1;
  color: ${props => props.theme.colors.spotBackground[2]};
  border: 3px solid ${props => props.theme.colors.success.main};
  border-radius: 50%;
  border-top: 3px solid ${props => props.theme.colors.spotBackground[0]};
  width: 24px;
  height: 24px;
  position: absolute;
  animation: ${rotate360} 4s linear infinite;
`;
