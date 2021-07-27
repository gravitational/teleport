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
    resolveRedirectUrl(params)
      .then(url => {
        window.location.replace(url);
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

function resolveRedirectUrl(params: UrlLauncherParams) {
  const location = window.location;
  const port = location.port ? ':' + location.port : '';
  const state = getUrlParameter('state', location.search);
  const arn = getUrlParameter('awsrole', location.search);

  // no state value: let the target app know of a new auth exchange
  if (!state) {
    return service.getAppFqdn(params).then(result => {
      const url = new URL(`https://${result.fqdn}${port}/x-teleport-auth`);
      if (params.clusterId) {
        url.searchParams.set('cluster', params.clusterId);
      }
      if (params.publicAddr) {
        url.searchParams.set('addr', params.publicAddr);
      }
      if (params.arn) {
        url.searchParams.set('awsrole', decodeURIComponent(params.arn));
      }

      return url.toString();
    });
  }

  // state value received: create new session for the target app
  if (arn) {
    params.arn = arn;
  }
  return service.createAppSession(params).then(result => {
    const url = new URL(`https://${result.fqdn}${port}/x-teleport-auth`);
    url.searchParams.set('state', state);
    url.hash = `#value=${result.value}`;
    return url.toString();
  });
}
