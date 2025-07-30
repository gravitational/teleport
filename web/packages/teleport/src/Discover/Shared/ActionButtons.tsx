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

import React, { PropsWithChildren } from 'react';

import { Box, ButtonPrimary, ButtonSecondary, ButtonText } from 'design';

export const ActionButtons = ({
  onProceed = null,
  onSkip = null,
  proceedHref = '',
  disableProceed = false,
  lastStep = false,
  onPrev = null,
}: {
  onProceed?(): void;
  onSkip?(): void;
  proceedHref?: string;
  disableProceed?: boolean;
  lastStep?: boolean;
  allowSkip?: boolean;
  onPrev?(): void;
}) => {
  const allowSkip = !!onSkip;

  return (
    <Box mt={4}>
      {proceedHref && (
        <ButtonPrimary
          size="medium"
          as="a"
          href={proceedHref}
          target="_blank"
          width="224px"
          mr={3}
          rel="noreferrer"
        >
          View Documentation
        </ButtonPrimary>
      )}
      {onProceed && (
        <ButtonPrimary
          width="165px"
          onClick={onProceed}
          data-testid="action-next"
          mr={3}
          disabled={disableProceed}
        >
          {lastStep ? 'Finish' : 'Next'}
        </ButtonPrimary>
      )}
      {allowSkip && (
        <ButtonSecondary width="165px" onClick={onSkip} mr={3}>
          Skip
        </ButtonSecondary>
      )}
      {onPrev && (
        <ButtonSecondary onClick={onPrev} mt={3} width="165px">
          Back
        </ButtonSecondary>
      )}
    </Box>
  );
};

export const AlternateInstructionButton: React.FC<
  PropsWithChildren<{
    onClick(): void;
    disabled?: boolean;
  }>
> = ({ onClick, children, disabled = false }) => {
  return (
    <ButtonText
      disabled={disabled}
      onClick={onClick}
      compact
      css={`
        color: ${p => p.theme.colors.buttons.link.default};
        text-decoration: underline;
        font-weight: normal;
        font-size: inherit;
      `}
    >
      {children || 'Use these instructions instead.'}
    </ButtonText>
  );
};
