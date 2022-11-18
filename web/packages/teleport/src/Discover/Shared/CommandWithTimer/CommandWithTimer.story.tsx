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

import { CommandWithTimer } from './CommandWithTimer';

export default {
  title: 'Teleport/Discover/Shared/CommandWithTimer',
};

export const DefaultPolling = () => (
  <CommandWithTimer {...props} poll={{ state: 'polling' }} />
);

export const DefaultPollingSuccess = () => (
  <CommandWithTimer {...props} poll={{ state: 'success' }} />
);

export const DefaultPollingError = () => (
  <CommandWithTimer
    {...props}
    poll={{
      state: 'error',
      error: { reasonContents: [<>error reason 1</>, <>error reason 2</>] },
    }}
  />
);

export const CustomPolling = () => (
  <CommandWithTimer
    {...props}
    poll={{ state: 'polling', customStateDesc: 'custom polling text' }}
    header={<div>Custom Header Component</div>}
  />
);

export const CustomPollingSuccess = () => (
  <CommandWithTimer
    {...props}
    poll={{ state: 'success', customStateDesc: 'custom polling success text' }}
  />
);

export const CustomPollingError = () => (
  <CommandWithTimer
    {...props}
    poll={{
      state: 'error',
      error: {
        reasonContents: [<>error reason 1</>, <>error reason 2</>],
        customErrContent: <>custom error content</>,
      },
    }}
  />
);

const props = {
  command: 'some kind of command',
  pollingTimeout: 100000,
};
