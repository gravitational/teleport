import Flex from 'design/Flex';

import styled from 'styled-components';

import { ResourceIcon, ResourceIconName } from 'design/ResourceIcon';
import Text from 'design/Text';
import React, { useState } from 'react';
import { Bullet, StepTitle, StepsContainer, BulletProps, StepNavigation } from 'teleport/components/StepNavigation';
import Image from 'design/Image';
import Box from 'design/Box';

export type FlowStepProps = {
  nextStep?: () => void;
  prevStep?: () => void;
};

export type View = {
  component: ((props: FlowStepProps) => JSX.Element);
  name: string;
}

export type FlowProps = {
  name: string;
  title: string;
  views: View[];
  icon: JSX.Element;
}

export function Flow({ name, title, views, icon }: FlowProps) {
  if (views.length < 1) {
    return null
  }

  const steps = views.length
  let [currentStep, setCurrentStep] = useState(0)

  function handleNextStep() {
    if (currentStep < steps - 1) {
      setCurrentStep(currentStep += 1)
    }
  }

  function handlePrevStep() {
    if (currentStep > 0) {
      setCurrentStep(currentStep -= 1)
    }
  }

  const currentView = views[currentStep]
  const Component = currentView.component

  return (
    <>
      <Flex pt="3">
        {icon}
        <Text bold ml="2" mr="4">{name}</Text>
        <StepNavigation
          currentStep={currentStep}
          steps={views.map(v => ({ title: v.name }))}
        />
      </Flex>
      <Text as="h2" fontSize="24px" pt="2" pb="3">{title}</Text>
      <Component nextStep={handleNextStep} prevStep={handlePrevStep} />
    </>
  )
}
