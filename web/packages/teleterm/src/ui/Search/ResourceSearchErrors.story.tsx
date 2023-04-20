/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import { routing } from 'teleterm/ui/uri';
import { ResourceSearchError } from 'teleterm/ui/services/resources';

import { ResourceSearchErrors } from './ResourceSearchErrors';

export default {
  title: 'Teleterm/ModalsHost/ResourceSearchErrors',
};

export const Story = () => (
  <ResourceSearchErrors
    getClusterName={routing.parseClusterName}
    onCancel={() => {}}
    errors={[
      new ResourceSearchError(
        '/clusters/foo',
        'server',
        new Error(
          '14 UNAVAILABLE: connection error: desc = "transport: authentication handshake failed: EOF"'
        )
      ),
      new ResourceSearchError(
        '/clusters/bar',
        'database',
        new Error(
          '2 UNKNOWN: Unable to connect to ssh proxy at teleport.local:443. Confirm connectivity and availability.\n	dial tcp: lookup teleport.local: no such host'
        )
      ),
      new ResourceSearchError(
        '/clusters/baz',
        'kube',
        new Error(
          '14 UNAVAILABLE: connection error: desc = "transport: authentication handshake failed: EOF"'
        )
      ),
      new ResourceSearchError(
        '/clusters/foo',
        'server',
        new Error(
          '2 UNKNOWN: Unable to connect to ssh proxy at teleport.local:443. Confirm connectivity and availability.\n	dial tcp: lookup teleport.local: no such host'
        )
      ),
      new ResourceSearchError(
        '/clusters/baz',
        'kube',
        new Error(
          '14 UNAVAILABLE: connection error: desc = "transport: authentication handshake failed: EOF"'
        )
      ),
      new ResourceSearchError(
        '/clusters/foo',
        'server',
        new Error(
          '2 UNKNOWN: Unable to connect to ssh proxy at teleport.local:443. Confirm connectivity and availability.\n	dial tcp: lookup teleport.local: no such host'
        )
      ),
    ]}
  />
);
