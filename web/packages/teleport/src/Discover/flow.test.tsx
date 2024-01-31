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

/*
 Discover is a complicated wizard that has different steps depending on what
 input has been given

 To be able to support this, we have the flow configured in an object, allowing
 infinitely deep states.

 To start, you define an array of `Resource`s

 const resources: Resource[] = [
   {
     kind: ResourceKind.Name,
     icon: <SomeIcon />,
     shouldPrompt(currentStep) {
       return true;
     },
     views: [],
   }
 ];

 `shouldPrompt` allows for the resource type to decide when to prompt the user
 if they try and navigate away. It receives `currentStep: number` which points
 to the active view in the `views` array. It should return a `boolean`, where
 `true` would prompt the user if they navigated away, and `false` would not.

 All the different views the resource can have go into the `views` property.

 const resources: Resource[] = [
   {
     kind: ResourceKind.Name,
     icon: <SomeIcon />,
     shouldPrompt(currentStep) {
       return true;
     },
     views: [
       {
         title: 'Select Resource Type',
         component: SomeComponent,
       },
       {
         title: 'Configure Resource',
         component: SomeOtherComponent,
       },
     ],
   }
 ];

 To add child views to a view, specify `views` again with the same schema

 const resources: Resource[] = [
   {
     kind: ResourceKind.Name,
     shouldPrompt(currentStep) {
       return true;
     },
     icon: <SomeIcon />,
     views: [
       {
         title: 'Select Resource Type',
         component: SomeComponent,
       },
       {
         title: 'Configure Resource',
         views: [
           {
             title: 'Deploy Database Agent',
             component: DatabaseAgentComponent,
           },
           {
             title: 'Register a Database',
             component: RegisterDatabaseComponent,
           },
         ],
       },
     ],
   }
 ];

 To keep track of what view is active, we track the currentStep index.

 Once a view has children, the first child's index is the same as the parent's index.

 This means we can just increment the `currentStep` by 1 each time to land on the next step,
 regardless of how deep inside the configuration object it is.

 Take this view configuration -

 const views: View[] = [
   {
     title: 'Select Resource Type',
     component: SomeComponent,
   },
   {
     title: 'Configure Resource',
     views: [
       {
         title: 'Deploy Database Agent',
         component: DatabaseAgentComponent,
       },
       {
         title: 'Register a Database',
         component: RegisterDatabaseComponent,
       },
     ],
   },
   {
     title: 'Test Connection',
     component: TestConnectionComponent,
   },
 ];

 `Select Resource Type` is index 0
 `Configure Resource` is index 1
    `Deploy Database Agent` is also index 1
      - This is because when you're on step 1, you don't want to view "Configure Resource" -
        there's no component for that stage, as it consists only of child views
    `Register a Database` is index 2
  `Test Connection` is index 3

  By tracking the step like this, we can increment the value from 0 and end up with
  - index === 0 - show "Select Resource Type"
  - index === 1 - show "Deploy Database Agent"
  - index === 2 - show "Register a Database"
  - index === 3 - show "Test Connection"

  The index of each stage is calculated via the `addParentAndIndexToViews` method.
 */

import { ResourceKind } from 'teleport/Discover/Shared';

import { computeViewChildrenSize, ResourceViewConfig, View } from './flow';

describe('discover flow', () => {
  describe('computeViewChildrenSize', () => {
    it('should calculate the children size correctly', () => {
      const resource: ResourceViewConfig = {
        kind: ResourceKind.Server,
        shouldPrompt: () => null,
        views: [
          {
            title: 'Select Resource Type',
            views: [
              {
                title: 'Ridiculous',
                views: [
                  {
                    title: 'Nesting',
                    views: [
                      {
                        title: 'Here',
                      },
                      {
                        title: 'Again',
                      },
                    ],
                  },
                ],
              },
            ],
          },
        ],
      };

      expect(computeViewChildrenSize(resource.views as View[])).toBe(2);
    });
  });
});
