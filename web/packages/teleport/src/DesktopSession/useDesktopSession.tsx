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

import { useEffect, useState } from 'react';
import { useParams } from 'react-router';
import useAttempt from 'shared/hooks/useAttemptNext';
import { UrlDesktopParams } from 'teleport/config';
import Ctx from 'teleport/teleportContext';
import { Desktop } from 'teleport/services/desktops';
import { stripRdpPort } from '../Desktops/DesktopList';
import useTdpClientCanvas from './useTdpClientCanvas';

export default function useDesktopSession(ctx: Ctx) {
  // Tracks combination of tdpclient/websocket and api call state,
  // as well as whether the tdp client for this session was intentionally disconnected.
  const { attempt: fetchAttempt, run } = useAttempt('processing');
  const { clusterId, desktopId } = useParams<UrlDesktopParams>();
  const [hostname, setHostname] = useState<string>('');
  const clientCanvasProps = useTdpClientCanvas();

  // creates hostname string from list of desktops based on url's desktopId
  const makeHostname = (desktops: Desktop[]) => {
    const desktop = desktops.find(d => d.name === desktopId);
    if (!desktop) {
      // throw error here so that runFetchDesktopAttempt knows to set the attempt to failed
      throw new Error('Desktop not found');
    }
    setHostname(stripRdpPort(desktop.addr));
  };

  useEffect(() => {
    run(() => ctx.desktopService.fetchDesktops(clusterId).then(makeHostname));
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
