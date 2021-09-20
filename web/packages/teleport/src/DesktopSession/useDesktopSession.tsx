/*
Copyright 2021 Gravitational, Inc.

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

import { useMemo, useEffect, useState } from 'react';
import { getAccessToken, getHostName } from 'teleport/services/api';
import { useParams } from 'react-router';
import useAttempt from 'shared/hooks/useAttemptNext';
import cfg, { UrlDesktopParams } from 'teleport/config';
import TdpClient from 'teleport/lib/tdp/client';
import Ctx from 'teleport/teleportContext';
import { Desktop } from 'teleport/services/desktops';
import { stripRdpPort } from '../Desktops/DesktopList';

export default function useDesktopSession(ctx: Ctx) {
  // Tracks combination of tdpclient/websocket and api call state,
  // as well as whether the tdp client for this session was intentionally disconnected.
  const { attempt, run } = useAttempt('processing');
  const { clusterId, username, desktopId } = useParams<UrlDesktopParams>();
  const [hostname, setHostname] = useState<string>('');
  const [connection, setConnection] = useState<TdpClientConnectionState>({
    status: 'connecting',
  });

  // creates hostname string from list of desktops based on url's desktopId
  const makeHostname = (desktops: Desktop[]) => {
    const desktop = desktops.find(d => d.name === desktopId);
    if (!desktop) {
      // throw error here so that runFetchDesktopAttempt knows to set the attempt to failed
      throw new Error('Desktop not found');
    }
    setHostname(stripRdpPort(desktop.addr));
  };

  // Build a client based on url parameters.
  const tdpClient = useMemo(() => {
    const addr = cfg.api.desktopWsAddr
      .replace(':fqdm', getHostName())
      .replace(':clusterId', clusterId)
      .replace(':desktopId', desktopId)
      .replace(':token', getAccessToken());

    return new TdpClient(addr, username);
  }, [clusterId, username, desktopId]);

  useEffect(() => {
    run(() => ctx.desktopService.fetchDesktops(clusterId).then(makeHostname));
  }, [tdpClient]);

  return {
    username,
    hostname,
    tdpClient,
    attempt,
    connection,
    setConnection,
    // clipboard and recording settings will eventuall come from backend, hardcoded for now
    clipboard: false,
    recording: false,
  };
}

export type State = ReturnType<typeof useDesktopSession>;

export type TdpClientConnectionState = {
  status: 'connecting' | 'connected' | 'disconnected' | 'error';
  statusText?: string;
};
