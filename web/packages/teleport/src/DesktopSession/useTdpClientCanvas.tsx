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

import { useState } from 'react';

import cfg from 'teleport/config';
import { TdpClient } from 'teleport/lib/tdp';
import { getHostName } from 'teleport/services/api';

import { TopBarHeight } from './TopBar';

export default function useTdpClientCanvas(props: Props) {
  const { username, desktopName, clusterId } = props;
  const addr = cfg.api.desktopWsAddr
    .replace(':fqdn', getHostName())
    .replace(':clusterId', clusterId)
    .replace(':desktopName', desktopName)
    .replace(':username', username);
  //TODO(gzdunek): It doesn't really matter here, but make TdpClient reactive to addr change.
  //Perhaps pass it to TdpClient.connect().
  const [tdpClient] = useState<TdpClient | null>(() => new TdpClient(addr));

  return {
    tdpClient,
    clientScreenSpecToRequest: getDisplaySize(),
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
};
