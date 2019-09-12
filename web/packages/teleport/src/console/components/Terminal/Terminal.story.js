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
import { StoreSession, StoreScp } from './../../stores';

const defaultProps = {
  storeSession: new StoreSession(),
  storeScp: new StoreScp(),
  onClose: () => null,
  onOpenPlayer: () => null,
  clusterId: '233',
  sid: '234',
};

storiesOf('TeleportConsole/Terminal', module)
  .add('Loading', () => {
    const storeSession = new StoreSession();
    storeSession.setStatus({ isLoading: true });

    const props = {
      ...defaultProps,
      storeSession,
    };

    return <Terminal {...props} />;
  })
  .add('Error', () => {
    const storeSession = new StoreSession();
    storeSession.setStatus({
      isError: true,
      errorText: 'system error with long text',
    });
    const props = {
      ...defaultProps,
      storeSession,
    };

    return <Terminal {...props} />;
  });
