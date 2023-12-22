import React from 'react';
import { Link } from 'react-router-dom'

import { FlowStepProps } from './Flow';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import cfg from 'teleport/config';

type FlowButtonsProps = {
  isLast?: boolean;
  isFirst?: boolean;
  // onFinish is called when the user clicks the primary button
  // in the last step
  onFinish?: () => void;
} & FlowStepProps

export function FlowButtons({ nextStep, prevStep, isFirst = false, isLast = false, onFinish }: FlowButtonsProps) {
  const handleConfirm = isLast ? onFinish : nextStep
  return (
    <>
      <ButtonPrimary onClick={handleConfirm} mr="3">{isLast ? 'Finish' : 'Next'}</ButtonPrimary>
      {isFirst ?
        <ButtonSecondary as={Link} to={cfg.getIntegrationEnrollRoute(null)}>Back</ButtonSecondary>
        :
        <ButtonSecondary onClick={prevStep}>Back</ButtonSecondary>
      }

    </>
  )
}