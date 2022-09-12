import React, { useMemo } from 'react';
import useMain from 'teleport/Main/useMain';
import { Main } from 'teleport/Main/Main';

import { useBanner } from 'e-teleport/Banner/useBanner';
import useTeleport from 'e-teleport/useTeleportE';
import SwitchBack from 'e-teleport/Banner/Switchback';

import getFeatures from '../features';

export default function Container() {
  const ctx = useTeleport();
  const features = useMemo(() => getFeatures(), []);
  const state = useMain(features);
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

  return <Main {...state} customBanners={customBanners} />;
}
