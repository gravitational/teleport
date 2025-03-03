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

import React, { useEffect } from 'react';

import { PING_INTERVAL } from 'teleport/Discover/Database/config';
import { PingTeleportProvider } from 'teleport/Discover/Shared/PingTeleportContext';
import { clearCachedJoinTokenResult } from 'teleport/Discover/Shared/useJoinTokenSuspender';

import { ResourceKind } from '../Shared';

interface DatabaseWrapperProps {
  children: React.ReactNode;
}

export function DatabaseWrapper(props: DatabaseWrapperProps) {
  useEffect(() => {
    return () => {
      // once the user leaves the desktop setup flow, delete the existing token
      clearCachedJoinTokenResult([ResourceKind.Database]);
    };
  }, []);

  return (
    <PingTeleportProvider
      interval={PING_INTERVAL}
      resourceKind={ResourceKind.Database}
    >
      {props.children}
    </PingTeleportProvider>
  );
}
