import React, { useState } from 'react';

import Flex from 'design/Flex';
import Text from 'design/Text';

import { StepNavigation } from 'teleport/components/StepNavigation';

export type FlowStepProps = {
  nextStep?: () => void;
  prevStep?: () => void;
};

export type View = {
  component: (props: FlowStepProps) => JSX.Element;
  name: string;
};

export type FlowProps = {
  name: string;
  title: string;
  views: View[];
  icon: JSX.Element;
};

export function GuidedFlow({ name, title, views, icon }: FlowProps) {
  if (views.length < 1) {
    return null;
  }

  const steps = views.length;
  let [currentStep, setCurrentStep] = useState(0);

  function handleNextStep() {
    if (currentStep < steps - 1) {
      setCurrentStep((currentStep += 1));
    }
  }

  function handlePrevStep() {
    if (currentStep > 0) {
      setCurrentStep((currentStep -= 1));
    }
  }

  const currentView = views[currentStep];
  const Component = currentView.component;

  return (
    <>
      <Flex pt="3">
        {icon}
        <Text bold ml="2" mr="4">
          {name}
        </Text>
        <StepNavigation
          currentStep={currentStep}
          steps={views.map(v => ({ title: v.name }))}
        />
      </Flex>
      <Text as="h2" fontSize="24px" pt="2" pb="3">
        {title}
      </Text>
      <Component nextStep={handleNextStep} prevStep={handlePrevStep} />
    </>
  );
}
