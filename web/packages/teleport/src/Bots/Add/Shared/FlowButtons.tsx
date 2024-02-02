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

import React from 'react';
import { Link } from 'react-router-dom';

import { ButtonPrimary, ButtonSecondary } from 'design/Button';

import cfg from 'teleport/config';

import { FlowStepProps } from './GuidedFlow';

export type ButtonState = {
  disabled?: boolean;
  hidden?: boolean;
};

export type FlowButtonsProps = {
  isLast?: boolean;
  isFirst?: boolean;
  nextButton?: ButtonState;
  backButton?: ButtonState;
} & FlowStepProps;

export function FlowButtons({
  nextStep,
  prevStep,
  isFirst = false,
  isLast = false,
  nextButton,
  backButton,
}: FlowButtonsProps) {
  return (
    <>
      {!nextButton?.hidden && (
        <ButtonPrimary
          disabled={nextButton?.disabled}
          onClick={nextStep}
          mr="3"
          data-testid="button-next"
        >
          {isLast ? 'Complete Integration' : 'Next'}
        </ButtonPrimary>
      )}
      {!backButton?.hidden && (
        <BackButton
          isFirst={isFirst}
          disabled={backButton?.disabled}
          prevStep={prevStep}
        />
      )}
    </>
  );
}

function BackButton({
  isFirst,
  disabled,
  prevStep,
}: {
  isFirst: boolean;
  disabled: boolean;
  prevStep: () => void;
}) {
  if (isFirst) {
    return (
      <ButtonSecondary
        disabled={disabled}
        as={Link}
        to={cfg.getBotsNewRoute()}
        data-testid="button-back-integration"
      >
        Back
      </ButtonSecondary>
    );
  }
  return (
    <ButtonSecondary
      data-testid="button-back"
      disabled={disabled}
      onClick={prevStep}
    >
      Back
    </ButtonSecondary>
  );
}
