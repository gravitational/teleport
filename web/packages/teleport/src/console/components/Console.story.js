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
import { createMemoryHistory } from 'history';
import { Router } from 'react-router';
import ConsoleComponent from './Console';
import { colors } from './colors';
import { ConsoleContext, Console } from './../useConsoleContext';

storiesOf('TeleportConsole', module).add('Console', () => {
  const teleConsole = new Console();
  teleConsole.storeDocs.add({
    title: 'root@root',
    url: '/root/',
  });

  const onClose = () => null;

  return (
    <TestLayout teleContext={teleConsole}>
      <ConsoleComponent onClose={onClose} />
    </TestLayout>
  );
});

export function TestLayout({ children, teleContext, teleHistory }) {
  const [state] = React.useState(() => {
    const context = teleContext || new Console();
    const history = teleHistory || createMemoryHistory({});
    return {
      context,
      history,
    };
  });

  return (
    <Router history={state.history}>
      <ConsoleContext.Provider value={state.context}>
        <Flex
          m={-3}
          style={{ position: 'absolute' }}
          width="100%"
          height="100%"
          bg={colors.bgTerminal}
        >
          {children}
        </Flex>
      </ConsoleContext.Provider>
    </Router>
  );
}
