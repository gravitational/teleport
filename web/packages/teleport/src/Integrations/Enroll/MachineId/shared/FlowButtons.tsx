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
  disabled?: boolean;
} & FlowStepProps;

export function FlowButtons({
  nextStep,
  prevStep,
  isFirst = false,
  isLast = false,
  onFinish,
  disabled = false,
}: FlowButtonsProps) {
  const handleConfirm = isLast ? onFinish : nextStep;
  return (
    <>
      <ButtonPrimary disabled={disabled} onClick={handleConfirm} mr="3">
        {isLast ? 'Finish' : 'Next'}
      </ButtonPrimary>
      {isFirst ? (
        <ButtonSecondary
          disabled={disabled}
          as={Link}
          to={cfg.getIntegrationEnrollRoute(null)}
        >
          Back
        </ButtonSecondary>
      ) : (
        <ButtonSecondary disabled={disabled} onClick={prevStep}>
          Back
        </ButtonSecondary>
      )}
    </>
  );
}
