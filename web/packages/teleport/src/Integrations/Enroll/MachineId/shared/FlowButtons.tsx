import React from 'react';
import { Link } from 'react-router-dom';

import { ButtonPrimary, ButtonSecondary } from 'design/Button';

import cfg from 'teleport/config';

import { FlowStepProps } from './GuidedFlow';

type FlowButtonsProps = {
  isLast?: boolean;
  isFirst?: boolean;
  // onFinish is called when the user clicks the primary button
  // in the last step
  onFinish?: () => void;
  disableNext?: boolean;
  disableBack?: boolean;
} & FlowStepProps;

export function FlowButtons({
  nextStep,
  prevStep,
  isFirst = false,
  isLast = false,
  onFinish,
  disableNext = false,
  disableBack = false,
}: FlowButtonsProps) {
  const handleConfirm = isLast ? onFinish : nextStep;
  return (
    <>
      <ButtonPrimary disabled={disableNext} onClick={handleConfirm} mr="3">
        {isLast ? 'Finish' : 'Next'}
      </ButtonPrimary>
      {isFirst ? (
        <ButtonSecondary
          disabled={disableBack}
          as={Link}
          to={cfg.getIntegrationEnrollRoute(null)}
        >
          Back
        </ButtonSecondary>
      ) : (
        <ButtonSecondary disabled={disableBack} onClick={prevStep}>
          Back
        </ButtonSecondary>
      )}
    </>
  );
}
