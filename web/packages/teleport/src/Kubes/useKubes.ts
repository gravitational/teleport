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

import { useState, useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import { Kube } from 'teleport/services/kube';
import TeleportContext from 'teleport/teleportContext';
import useStickyClusterId from 'teleport/useStickyClusterId';

export default function useKubes(ctx: TeleportContext) {
  const { clusterId, isLeafCluster } = useStickyClusterId();
  const { username, authType } = ctx.storeUser.state;
  const canCreate = ctx.storeUser.getTokenAccess().create;
  const { run, attempt } = useAttempt('processing');
  const [kubes, setKubes] = useState([] as Kube[]);

  useEffect(() => {
    run(() =>
      ctx.kubeService
        .fetchKubernetes(clusterId)
        .then(res => setKubes(res.kubes))
    );
  }, [clusterId]);

  return {
    kubes,
    attempt,
    username,
    authType,
    isLeafCluster,
    clusterId,
    canCreate,
  };
}

export type State = ReturnType<typeof useKubes>;
