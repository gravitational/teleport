/*
Copyright 2019-2022 Gravitational, Inc.

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

import React, { useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';

import useTeleport from 'teleport/useTeleport';
import { useAlerts } from 'teleport/components/BannerList/useAlerts';

import { useFeatures } from 'teleport/FeaturesContext';

import type { ClusterAlert } from 'teleport/services/alerts';

export interface UseMainConfig {
  customBanners?: React.ReactNode[];
  initialAlerts?: ClusterAlert[];
}

export default function useMain(config: UseMainConfig) {
  const ctx = useTeleport();
  const { attempt, setAttempt, run } = useAttempt('processing');
  const { alerts, dismissAlert } = useAlerts(config.initialAlerts);

  const features = useFeatures();

  useEffect(() => {
    // Two routes that uses this hook that can trigger this effect:
    //  - cfg.root
    //  - cfg.discover
    // These two routes both require top user nav dropdown items
    // to be in sync and requires fetching of user context state before
    // rendering.
    //
    // When one route calls init() e.g: if user redirects to discover on login,
    // it isn't required to refetch context and reinit features with the other
    // route and vice versa.
    if (ctx.storeUser.state) {
      setAttempt({ status: 'success' });
      return;
    }

    run(() => ctx.init(features));
  }, []);

  return {
    alerts,
    customBanners: config.customBanners || [],
    ctx,
    dismissAlert,
    status: attempt.status,
    statusText: attempt.statusText,
  };
}

export type State = ReturnType<typeof useMain>;
