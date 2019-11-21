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
import { Terminal } from './Terminal';
import TerminalStore from './storeTerminal';

const defaultProps = {
  terminal: new TerminalStore(),
  onClose: () => null,
  onOpenPlayer: () => null,
  clusterId: '233',
  sid: '234',
};

storiesOf('TeleportConsole/TabTerminal', module)
  .add('Loading', () => {
    const terminal = new TerminalStore();
    terminal.state.status = `loading`;

    const props = {
      ...defaultProps,
      terminal,
    };

    return <Terminal {...props} />;
  })
  .add('Error', () => {
    const terminal = new TerminalStore();
    terminal.state = {
      ...terminal.state,
      status: 'error',
      statusText: 'system error',
    };

    const props = {
      ...defaultProps,
      terminal,
    };

    return <Terminal {...props} />;
  })
  .add('NotFoundSession', () => {
    const terminal = new TerminalStore();
    terminal.state = {
      ...terminal.state,
      status: 'notfound',
      statusText: 'system error',
    };

    const props = {
      ...defaultProps,
      terminal,
    };

    return <Terminal {...props} />;
  });
