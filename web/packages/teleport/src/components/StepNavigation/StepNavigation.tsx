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

import { Flex } from 'design';

import { StepTitle, StepsContainer } from './Shared';
import { Bullet } from './Bullet';

export type StepItem = {
  title: string;
};

interface NavigationProps {
  currentStep: number;
  steps: StepItem[];
}

export function StepNavigation({ currentStep, steps }: NavigationProps) {
  const items: JSX.Element[] = [];

  steps.forEach((step, index) => {
    const isDone = currentStep > index;
    let isActive = currentStep === index;

    items.push(
      <StepsContainer active={isDone || isActive} key={`${step.title}${index}`}>
        <StepTitle>
          <Bullet isDone={isDone} isActive={isActive} stepNumber={index + 1} />
          {step.title}
        </StepTitle>
      </StepsContainer>
    );
  });

  return <Flex>{items}</Flex>;
}
