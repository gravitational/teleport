/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import Flex from 'design/Flex';

import { DiscoverIcon } from 'teleport/Discover/SelectResource/icons';
import { StepTitle, StepsContainer } from 'teleport/components/StepNavigation';
import {
  Bullet,
  Props as BulletProps,
} from 'teleport/components/StepNavigation/Bullet';

import { StepList } from './StepList';

import type { View } from 'teleport/Discover/flow';
import type { ResourceSpec } from '../SelectResource';

// FirstStepItemProps are the required
// props to render the first step item
// in the step navigation.
type FirstStepItemProps = {
  view?: never;
  currentStep?: never;
  index?: never;
  selectedResource: ResourceSpec;
};

// RestOfStepItemProps are the required
// props to render the rest of the step item's
// after the `FirstStepItemProps`.
type RestOfStepItemProps = {
  view: View;
  currentStep: number;
  index: number;
  selectedResource?: never;
};

export type StepItemProps = FirstStepItemProps | RestOfStepItemProps;

export function StepItem(props: StepItemProps) {
  if (props.selectedResource) {
    return (
      <StepsContainer>
        <StepTitle>
          <BulletIcon
            Icon={<DiscoverIcon name={props.selectedResource.icon} />}
          />
          {props.selectedResource.name}
        </StepTitle>
      </StepsContainer>
    );
  }

  if (props.view.hide) {
    return null;
  }

  let isActive = props.currentStep === props.view.index;
  // Make items for nested views.
  // Nested views is possible when a view has it's
  // own set of sub-steps.
  if (props.view.views) {
    return (
      <StepList
        views={props.view.views}
        currentStep={props.currentStep}
        index={props.index}
      />
    );
  }

  const isDone = props.currentStep > props.view.index;

  return (
    <StepsContainer active={isDone || isActive}>
      <StepTitle>
        <BulletIcon
          isDone={isDone}
          isActive={isActive}
          stepNumber={props.view.index + 1}
        />
        {props.view.title}
      </StepTitle>
    </StepsContainer>
  );
}

function BulletIcon({
  isDone,
  isActive,
  Icon,
  stepNumber,
}: BulletProps & {
  Icon?: JSX.Element;
}) {
  if (Icon) {
    return <Flex mr={2}>{Icon}</Flex>;
  }

  return <Bullet isDone={isDone} isActive={isActive} stepNumber={stepNumber} />;
}
