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

import { PingTeleportProvider } from 'teleport/Discover/Shared/PingTeleportContext';
import { JoinTokenProvider } from 'teleport/Discover/Shared/JoinTokenContext';

import { ResourceKind } from '../Shared';

const PING_TIMEOUT = 1000 * 60 * 5; // 5 minutes
const PING_INTERVAL = 1000 * 3; // 3 seconds
export const SCRIPT_TIMEOUT = 1000 * 60 * 5; // 5 minutes

export function ServerWrapper(props: ServerWrapperProps) {
  return (
    <JoinTokenProvider timeout={SCRIPT_TIMEOUT}>
      <PingTeleportProvider
        timeout={PING_TIMEOUT}
        interval={PING_INTERVAL}
        resourceKind={ResourceKind.Server}
      >
        {props.children}
      </PingTeleportProvider>
    </JoinTokenProvider>
  );
}

interface ServerWrapperProps {
  children: React.ReactNode;
}
