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

import { useEffect, useState, useMemo } from 'react';
import { useParams } from 'react-router';
import useAttempt from 'shared/hooks/useAttemptNext';
import { UrlDesktopParams } from 'teleport/config';
import Ctx from 'teleport/teleportContext';
import useTdpClientCanvas from './useTdpClientCanvas';

export default function useDesktopSession(ctx: Ctx) {
  const { attempt: fetchAttempt, run } = useAttempt('processing');

  // tdpConnection tracks the state of the tdpClient's TDP connection
  // tdpConnection.status ===
  // - 'processing' at first
  // - 'success' once the first TdpClientEvent.IMAGE_FRAGMENT is seen
  // - 'failed' if a TdpClientEvent.TDP_ERROR is encountered
  const { attempt: tdpConnection, setAttempt: setTdpConnection } =
    useAttempt('processing');

  // wsConnection track's the state of the tdpClient's websocket connection.
  // 'closed' to start, 'open' when TdpClientEvent.WS_OPEN is encountered, then 'closed'
  // again when TdpClientEvent.WS_CLOSE is encountered.
  const [wsConnection, setWsConnection] = useState<'open' | 'closed'>('closed');

  // disconnected tracks whether the user intentionally disconnected the client
  const [disconnected, setDisconnected] = useState(false);

  const { username, desktopName, clusterId } = useParams<UrlDesktopParams>();
  const [hostname, setHostname] = useState<string>('');
  const clientCanvasProps = useTdpClientCanvas({
    username,
    desktopName,
    clusterId,
    setTdpConnection,
    setWsConnection,
  });

  document.title = useMemo(
    () => `${clusterId} • ${username}@${hostname}`,
    [hostname]
  );

  useEffect(() => {
    run(() =>
      ctx.desktopService
        .fetchDesktop(clusterId, desktopName)
        .then(desktop => setHostname(desktop.addr))
    );
  }, [clusterId, desktopName]);

  return {
    hostname,
    username,
    // clipboard and recording settings will eventuall come from backend, hardcoded for now
    clipboard: false,
    recording: false,
    fetchAttempt,
    tdpConnection,
    wsConnection,
    disconnected,
    setDisconnected,
    ...clientCanvasProps,
  };
}

export type State = ReturnType<typeof useDesktopSession>;
