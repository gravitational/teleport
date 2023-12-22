import React from 'react';
import { FlowStepProps } from './Flow';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';

type FlowButtonsProps = {
  isLast: boolean;
  isFirst: boolean;
  // onFinish is called when the user clicks the primary button
  // in the last step
  onFinish?: () => void;
} & FlowStepProps

export function FlowButtons({ nextStep, prevStep, isFirst, isLast, onFinish }: FlowButtonsProps) {
  const handleConfirm = isLast ? onFinish : nextStep
  return (
    <>
      <ButtonPrimary onClick={handleConfirm}>{isLast ? 'Finish' : 'Next'}</ButtonPrimary>
      {!isFirst && <ButtonSecondary onClick={prevStep} ml="3">Back</ButtonSecondary>}
    </>
  )
}