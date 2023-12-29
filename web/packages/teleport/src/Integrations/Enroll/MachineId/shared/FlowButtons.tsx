import React from 'react';
import { Link } from 'react-router-dom';

import { ButtonPrimary, ButtonSecondary } from 'design/Button';

import cfg from 'teleport/config';

import { FlowStepProps } from './GuidedFlow';

export type ButtonState = {
  disabled?: boolean;
  hidden?: boolean;
};

type FlowButtonsProps = {
  isLast?: boolean;
  isFirst?: boolean;
  // onFinish is called when the user clicks the primary button
  // in the last step
  onFinish?: () => void;
  disableNext?: boolean;
  disableBack?: boolean;
  nextButton?: ButtonState;
  backButton?: ButtonState;
} & FlowStepProps;

export function FlowButtons({
  nextStep,
  prevStep,
  isFirst = false,
  isLast = false,
  onFinish,
  nextButton,
  backButton,
}: FlowButtonsProps) {
  const handleConfirm = isLast ? onFinish : nextStep;
  return (
    <>
      {!nextButton?.hidden && (
        <ButtonPrimary
          disabled={nextButton?.disabled}
          onClick={handleConfirm}
          mr="3"
        >
          {isLast ? 'Finish' : 'Next'}
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
        to={cfg.getIntegrationEnrollRoute(null)}
      >
        Back
      </ButtonSecondary>
    );
  }
  return (
    <ButtonSecondary disabled={disabled} onClick={prevStep}>
      Back
    </ButtonSecondary>
  );
}
