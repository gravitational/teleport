/**
 * Copyright 2020 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { useState, useEffect } from 'react';
import { App } from 'teleport/services/apps';
import useAttempt from 'shared/hooks/useAttemptNext';
import useTeleport from 'teleport/useTeleport';
import useStickyClusterId from 'teleport/useStickyClusterId';

export default function useApps() {
  const ctx = useTeleport();
  const canCreate = ctx.storeUser.getTokenAccess().create;
  const [isAddAppVisible, setAppAddVisible] = useState(false);
  const { clusterId } = useStickyClusterId();
  const { attempt, run } = useAttempt('processing');
  const [apps, setApps] = useState([] as App[]);
  const isEnterprise = ctx.isEnterprise;

  function refresh() {
    return run(() => ctx.appService.fetchApps(clusterId).then(setApps));
  }

  const hideAddApp = () => {
    setAppAddVisible(false);
    refresh();
  };

  const showAddApp = () => {
    setAppAddVisible(true);
  };

  useEffect(() => {
    refresh();
  }, [clusterId]);

  return {
    clusterId,
    isEnterprise,
    isAddAppVisible,
    hideAddApp,
    showAddApp,
    canCreate,
    attempt,
    apps,
  };
}

export type State = ReturnType<typeof useApps>;
