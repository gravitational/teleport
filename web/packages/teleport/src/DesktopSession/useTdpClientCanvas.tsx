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
import { AuthenticatedWebSocket } from 'teleport/lib/AuthenticatedWebSocket';
import { TdpClient } from 'teleport/lib/tdp';
import { getHostName } from 'teleport/services/api';

export default function useTdpClientCanvas(props: Props) {
  const { username, desktopName, clusterId } = props;
  //TODO(gzdunek): It doesn't really matter here, but make TdpClient reactive to addr change.
  //Perhaps pass it to TdpClient.connect().
  const [tdpClient] = useState<TdpClient>(
    () =>
      new TdpClient(
        () =>
          new AuthenticatedWebSocket(
            cfg.api.desktopWsAddr
              .replace(':fqdn', getHostName())
              .replace(':clusterId', clusterId)
              .replace(':desktopName', desktopName)
              .replace(':username', username)
          )
      )
  );

  return {
    tdpClient,
  };
}

type Props = {
  username: string;
  desktopName: string;
  clusterId: string;
};
