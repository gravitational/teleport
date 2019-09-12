/*
Copyright 2019 Gravitational, Inc.

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

import React from 'react';
import { useAttempt } from 'shared/hooks';
import { Indicator, Box } from 'design';
import * as Alerts from 'design/Alert';
import cfg from 'teleport/config';
import { Redirect } from 'teleport/components/Router';
import { useConsole } from '../../console';

export default function SessionCreator({ match }) {
  const { clusterId, serverId, login } = match.params;
  const [sid, setSid] = React.useState();
  const [attempt, { error }] = useAttempt({
    isProcessing: true,
  });

  const console = useConsole();

  React.useEffect(() => {
    console.storeSession
      .createSession({ clusterId, serverId, login })
      .then(sid => {
        setSid(sid);
      })
      .catch(err => {
        error(err);
      });
  }, [clusterId]);

  // after obtaining the session id, redirect to a terminal
  if (sid) {
    const route = cfg.getConsoleSessionRoute({ clusterId, sid });
    return <Redirect to={route} />;
  }

  const { isProcessing, isFailed } = attempt;

  if (isProcessing) {
    return (
      <Box textAlign="center" m={10}>
        <Indicator />
      </Box>
    );
  }

  if (isFailed) {
    return (
      <Alerts.Danger m={10}>Connection error: {status.errorText}</Alerts.Danger>
    );
  }

  return null;
}
