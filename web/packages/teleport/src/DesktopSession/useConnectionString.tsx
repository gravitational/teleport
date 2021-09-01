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

import { useMemo } from 'react';
import { useParams } from 'react-router';
import cfg, { UrlDesktopParams } from 'teleport/config';
import { getAccessToken, getHostName } from 'teleport/services/api';

export default function useConnectionString() {
  const { clusterId, username, desktopId } = useParams<UrlDesktopParams>();
  const addr = useMemo(() => {
    return cfg.api.desktopWsAddr
      .replace(':fqdm', getHostName())
      .replace(':clusterId', clusterId)
      .replace(':desktopId', desktopId)
      .replace(':token', getAccessToken());
  }, [clusterId, username, desktopId]);

  return {
    addr,
    username,
  };
}

export type State = ReturnType<typeof useConnectionString>;
