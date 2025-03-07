/* eslint-disable no-console */
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

import { useCallback, useEffect, useRef, useState } from 'react';

import { Attempt } from 'shared/hooks/useAttemptNext';

import cfg from 'teleport/config';
import { TdpClient } from 'teleport/lib/tdp';
import { getHostName } from 'teleport/services/api';

import { TopBarHeight } from './TopBar';
import { Setter } from './useDesktopSession';

declare global {
  interface Navigator {
    userAgentData?: { platform: any };
  }
}

export default function useTdpClientCanvas(props: Props) {
  const { username, desktopName, clusterId, setTdpConnection } = props;
  const [tdpClient, setTdpClient] = useState<TdpClient | null>(null);
  const initialTdpConnectionSucceeded = useRef(false);

  useEffect(() => {
    const addr = cfg.api.desktopWsAddr
      .replace(':fqdn', getHostName())
      .replace(':clusterId', clusterId)
      .replace(':desktopName', desktopName)
      .replace(':username', username);

    setTdpClient(new TdpClient(addr));
  }, [clusterId, username, desktopName]);

  const setInitialTdpConnectionSucceeded = useCallback(
    (callback: () => void) => {
      // The first image fragment we see signals a successful TDP connection.
      if (!initialTdpConnectionSucceeded.current) {
        callback();
        setTdpConnection({ status: 'success' });
        initialTdpConnectionSucceeded.current = true;
      }
    },
    [setTdpConnection]
  );

  return {
    tdpClient,
    clientScreenSpecToRequest: getDisplaySize(),
    setInitialTdpConnectionSucceeded,
  };
}

// Calculates the size (in pixels) of the display.
// Since we want to maximize the display size for the user, this is simply
// the full width of the screen and the full height sans top bar.
function getDisplaySize() {
  return {
    width: window.innerWidth,
    height: window.innerHeight - TopBarHeight,
  };
}

type Props = {
  username: string;
  desktopName: string;
  clusterId: string;
  setTdpConnection: Setter<Attempt>;
};
