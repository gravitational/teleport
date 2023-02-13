/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { storiesOf } from '@storybook/react';
import { Flex } from 'design';
import { RouterProvider, createMemoryRouter } from 'react-router';

import Console from './Console';
import { colors } from './colors';
import ConsoleContext from './consoleContext';
import ConsoleContextProvider from './consoleContextProvider';

storiesOf('Teleport/Console', module).add('Console', () => {
  const ctx = new ConsoleContext();
  return (
    <TestLayout ctx={ctx}>
      <Console />
    </TestLayout>
  );
});

export function TestLayout(props: PropType) {
  const [context] = React.useState((): ConsoleContext => {
    return props.ctx || new ConsoleContext();
  });

  const routes = [
    {
      path: '/:clusterId',
      element: (
        <ConsoleContextProvider value={context}>
          <Flex
            m={-3}
            style={{ position: 'absolute' }}
            width="100%"
            height="100%"
            bg={colors.bgTerminal}
          >
            {props.children}
          </Flex>
        </ConsoleContextProvider>
      ),
    },
  ];

  const router = createMemoryRouter(routes, {
    initialEntries: ['/clusterX'],
    initialIndex: 0,
  });

  return <RouterProvider router={router} />;
}

type PropType = {
  children: any;
  ctx?: ConsoleContext;
  history?: History;
};
