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

import { Flex } from 'design';

import { BaseView } from '../flow';
import { Bullet } from './Bullet';
import { StepsContainer, StepTitle } from './Shared';
import { StepList } from './StepList';

export type StepIcon = {
  component: JSX.Element;
  title: string;
};

interface NavigationProps<T> {
  currentStep: number;
  views: BaseView<T>[];
  startWithIcon?: StepIcon;
}

/**
 * Renders horizontal steps for each view.
 *
 * @param views can be simple (non-nested) or nested for
 * more complex configurations (see below for an example).
 *
 * For nested views, it is required to apply
 * function `addIndexToViews(views: BaseView<T>[])`
 * before passing the views to Navigation so it can correctly
 * increment the steps.
 *
 * For simple views, defining indexes is not required.
 *
 *
 * @example
 * How nesting views are used for Discover wizards:
 *
 * Discover is a complicated wizard that has different steps depending on what
 * input has been given.
 *
 * To be able to support this, we have the flow configured in an object, allowing
 * infinitely deep states.
 *
 * All the different views the resource can have go into the `views` property eg:
 *
 * const resources: Resource[] = [
 *   {
 *     kind: ResourceKind.Name,
 *     icon: <SomeIcon />,
 *     shouldPrompt(currentStep) {
 *       return true;
 *     },
 *     views: [
 *       {
 *         title: 'Select Resource Type',
 *         component: SomeComponent,
 *       },
 *       {
 *         title: 'Configure Resource',
 *         component: SomeOtherComponent,
 *       },
 *     ],
 *   }
 * ];
 *
 * To add child views to a view, specify `views` again with the same schema
 *
 * const resources: Resource[] = [
 *   {
 *     kind: ResourceKind.Name,
 *     shouldPrompt(currentStep) {
 *       return true;
 *     },
 *     icon: <SomeIcon />,
 *     views: [
 *       {
 *         title: 'Select Resource Type',
 *         component: SomeComponent,
 *       },
 *       {
 *         title: 'Configure Resource',
 *         views: [
 *           {
 *             title: 'Deploy Database Agent',
 *             component: DatabaseAgentComponent,
 *           },
 *           {
 *             title: 'Register a Database',
 *             component: RegisterDatabaseComponent,
 *           },
 *         ],
 *       },
 *     ],
 *   }
 * ];
 *
 * To keep track of what view is active, we track the currentStep index.
 *
 * Once a view has children, the first child's index is the same as the parent's index.
 *
 * This means we can just increment the `currentStep` by 1 each time to land on the next step,
 * regardless of how deep inside the configuration object it is.
 *
 * Take this view configuration -
 *
 * const views: View[] = [
 *   {
 *     title: 'Select Resource Type',
 *     component: SomeComponent,
 *   },
 *   {
 *     title: 'Configure Resource',
 *     views: [
 *       {
 *         title: 'Deploy Database Agent',
 *         component: DatabaseAgentComponent,
 *       },
 *       {
 *         title: 'Register a Database',
 *         component: RegisterDatabaseComponent,
 *       },
 *     ],
 *   },
 *   {
 *     title: 'Test Connection',
 *     component: TestConnectionComponent,
 *   },
 * ];
 *
 * `Select Resource Type` is index 0
 * `Configure Resource` is index 1
 *    `Deploy Database Agent` is also index 1
 *      - This is because when you're on step 1, you don't want to view "Configure Resource" -
 *        there's no component for that stage, as it consists only of child views
 *    `Register a Database` is index 2
 *  `Test Connection` is index 3
 *
 *  By tracking the step like this, we can increment the value from 0 and end up with
 *  - index === 0 - show "Select Resource Type"
 *  - index === 1 - show "Deploy Database Agent"
 *  - index === 2 - show "Register a Database"
 *  - index === 3 - show "Test Connection"
 *
 *  The index of each stage is calculated via the `addParentAndIndexToViews` method.
 */
export function Navigation<T>({
  currentStep,
  views,
  startWithIcon,
}: NavigationProps<T>) {
  return (
    <Flex>
      {startWithIcon && (
        <StepsContainer>
          <StepTitle css={{ fontWeight: 'bold' }}>
            <Bullet Icon={startWithIcon.component} />
            {startWithIcon.title}
          </StepTitle>
        </StepsContainer>
      )}
      <StepList views={views} currentStep={currentStep} />
    </Flex>
  );
}
