import React from 'react';

import { Main } from 'teleport/Main/Main';

import { useBanner } from 'e-teleport/Banner/useBanner';
import useTeleport from 'e-teleport/useTeleportE';
import SwitchBack from 'e-teleport/Banner/Switchback';

export default function Container() {
  const ctx = useTeleport();
  const { license } = useBanner();

  // TODO(hatch): In the near future the license warning will come over the same cluster
  //              alerts endpoint and this check along with the `useBanner` call can be
  //              removed.
  const initialAlerts = [];
  if (license) {
    initialAlerts.push({
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

  const customBanners = [];
  if (ctx.storeAccessRequests.getSessionExpiry()) {
    customBanners.push(
      <SwitchBack key="access-request-banner" data-testid="banner" />
    );
  }

  return <Main initialAlerts={initialAlerts} customBanners={customBanners} />;
}
