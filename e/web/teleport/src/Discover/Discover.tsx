import React, { useState } from 'react';

import { Discover as DiscoverComponent } from 'teleport/Discover/Discover';
import { useDiscover } from 'teleport/Discover/useDiscover';

import useTeleport from 'e-teleport/useTeleportE';
import { useBanner } from 'e-teleport/Banner/useBanner';
import SwitchBack from 'e-teleport/Banner/Switchback';

import getFeatures from 'e-teleport/features';

export function Discover() {
  // Init with enterprise features.
  const [features] = useState(() => getFeatures());
  const ctx = useTeleport();
  const state = useDiscover(ctx, features);
  const { license } = useBanner();

  // XXX In the near future the license warning will come over the same cluster
  // alerts endpoint and this check along with the `useBanner` call can be
  // removed.
  if (license) {
    state.alerts.push({
      kind: 'license-warning',
      version: 'v1',
      metadata: {
        name: 'license-warning',
        labels: {},
      },
      expires: '',
      spec: {
        severity: 10,
        message: license.text,
        created: '',
      },
    });
  }

  let customBanners = [];
  if (ctx.storeAccessRequests.getSessionExpiry()) {
    customBanners.push(
      <SwitchBack key="access-request-banner" data-testid="banner" />
    );
  }

  return <DiscoverComponent {...state} />;
}
