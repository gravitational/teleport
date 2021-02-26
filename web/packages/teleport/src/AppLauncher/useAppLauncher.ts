/*
Copyright 2020 Gravitational, Inc.

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
import { useParams } from 'react-router';
import service from 'teleport/services/apps';
import useAttempt from 'shared/hooks/useAttemptNext';
import { UrlLauncherParams } from 'teleport/config';
import { getUrlParameter } from 'teleport/services/history';

export default function useAppLauncher() {
  const params = useParams<UrlLauncherParams>();
  const { attempt, setAttempt } = useAttempt('processing');

  React.useEffect(() => {
    service
      .createAppSession(params)
      .then(result => {
        // make a redirect to the requested app auth endpoint
        const location = window.location;
        const port = location.port ? ':' + location.port : '';
        const state = getUrlParameter('state', location.search);
        const authUrl = `https://${result.fqdn}${port}/x-teleport-auth`;
        if (state === '') {
          const clusterId = params.clusterId ? params.clusterId : '';
          const publicAddr = params.publicAddr ? params.publicAddr : '';
          window.location.replace(`${authUrl}?cluster=${clusterId}&addr=${publicAddr}`);
        } else {
          window.location.replace(`${authUrl}?state=${state}#value=${result.value}`);
        }
      })
      .catch((err: Error) => {
        setAttempt({
          status: 'failed',
          statusText: err.message,
        });
      });
  }, []);

  return {
    ...attempt,
  };
}
