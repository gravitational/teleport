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
import { Attempt } from 'shared/hooks/useAttemptNext';
import cfg, { UrlDesktopParams } from 'teleport/config';
import TdpClient from 'teleport/lib/tdp/client';
import Ctx from 'teleport/teleportContext';
import { Desktop } from 'teleport/services/desktops';
import { stripRdpPort } from '../Desktops/DesktopList';

// Extends Attempt to allow for an additional 'disconnected' state,
// which allows us to display a non-error for instances where the connection
// was intentionally disconnected.
export type DesktopSessionAttempt = {
  status: Attempt['status'] | 'disconnected';
  statusText?: string;
};

export default function useDesktopSession(ctx: Ctx) {
  // Tracks combination of tdpclient/websocket and api call state,
  // as well as whether the tdp client for this session was intentionally disconnected.
  const [attempt, setAttempt] = useState<DesktopSessionAttempt>({
    status: 'processing',
  });
  const { clusterId, username, desktopId } = useParams<UrlDesktopParams>();
  const [userHost, setUserHost] = useState<string>('user@hostname');

  // creates user@hostname string from list of desktops based on url's desktopId
  const makeUserHost = (desktops: Desktop[]) => {
    const desktop = desktops.find(d => d.name === desktopId);
    if (!desktop) {
      // throw error here so that runFetchDesktopAttempt knows to set the attempt to failed
      throw new Error('Desktop not found');
    }
    setUserHost(`${username}@${stripRdpPort(desktop.addr)}`);
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
    Promise.all([
      ctx.desktopService.fetchDesktops(clusterId),
      // This Promise effectively takes the events emmitted during the tdpClient's
      // initial attempted connection to the websocket and converts them into a standard
      // promise that resolves on success and rejects on failure.
      new Promise<void>((resolve, reject) => {
        tdpClient.on('open', () => {
          resolve();
        });

        tdpClient.on('error', (err: Error) => {
          reject(err);
        });

        tdpClient.connect();
      }),
    ])
      .then(vals => {
        makeUserHost(vals[0]);

        // Remove the listeners from the Promise.all call and then initialize
        // many of the listeners that will last the client's lifetime.
        tdpClient.removeAllListeners('open');
        tdpClient.removeAllListeners('error');

        // Triggered in the event of a websocket protocol error.
        tdpClient.on('error', () => {
          setAttempt({
            status: 'failed',
            statusText: 'websocket protocol error',
          });

          tdpClient.removeAllListeners();
        });

        // If the websocket is closed remove all listeners that depend on it.
        tdpClient.on('close', (message: { userDisconnected: boolean }) => {
          // If it was closed intentionally by the user, set attempt to disconnected,
          // otherwise assume a server or network error caused an unexpected disconnect.
          if (message.userDisconnected) {
            setAttempt({ status: 'disconnected' });
          } else {
            setAttempt({
              status: 'failed',
              statusText: 'server or network error',
            });
          }

          tdpClient.removeAllListeners();
        });

        setAttempt({ status: 'success' });
      })
      .catch(err => {
        // Catches errors from the initial attempted websocket connection or api call(s).
        setAttempt({ status: 'failed', statusText: err.message });
        tdpClient.removeAllListeners();
      });
  }, [clusterId, username, desktopId]);

  return {
    tdpClient,
    userHost,
    attempt,
    // clipboard and recording settings will eventuall come from backend, hardcoded for now
    clipboard: false,
    recording: false,
  };
}

export type State = ReturnType<typeof useDesktopSession>;
