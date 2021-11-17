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
import { useTdpClientCanvas } from './TdpClientCanvas';

export default function useDesktopSession(ctx: Ctx) {
  // Tracks combination of tdpclient/websocket and api call state,
  // as well as whether the tdp client for this session was intentionally disconnected.
  const { attempt: fetchAttempt, run } = useAttempt('processing');
  const { username, desktopId, clusterId } = useParams<UrlDesktopParams>();
  const [hostname, setHostname] = useState<string>('');
  const clientCanvasProps = useTdpClientCanvas({
    username,
    desktopId,
    clusterId,
  });

  document.title = useMemo(() => `${clusterId} • ${username}@${hostname}`, [
    hostname,
  ]);

  useEffect(() => {
    run(() =>
      ctx.desktopService
        .fetchDesktop(clusterId, desktopId)
        .then(desktop => setHostname(desktop.addr))
    );
  }, [clusterId, desktopId]);

  return {
    hostname,
    // clipboard and recording settings will eventuall come from backend, hardcoded for now
    clipboard: false,
    recording: false,
    fetchAttempt,
    ...clientCanvasProps,
  };
}

export type State = ReturnType<typeof useDesktopSession>;
