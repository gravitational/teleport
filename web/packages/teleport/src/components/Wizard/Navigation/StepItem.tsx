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

import {
  StepsContainer,
  StepTitle,
} from 'teleport/components/Wizard/Navigation';
import { Bullet } from 'teleport/components/Wizard/Navigation/Bullet';

import { BaseView } from '../flow';
import { StepList } from './StepList';

export function StepItem<T>(props: {
  view: BaseView<T>;
  currentStep: number;
  index: number;
}) {
  if (props.view.hide) {
    return null;
  }

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

  const index = props.view.index ?? props.index;
  const isActive = props.currentStep === index;
  const isDone = props.currentStep > index;

  return (
    <StepsContainer active={isDone || isActive}>
      <StepTitle>
        <Bullet
          isDone={isDone}
          isActive={isActive}
          stepNumber={props.view.displayIndex ?? index + 1}
        />
        {props.view.title}
      </StepTitle>
    </StepsContainer>
  );
}
