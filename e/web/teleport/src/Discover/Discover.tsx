import React from 'react';

import { DiscoverComponent } from 'teleport/Discover/Discover';
import { AgentMeta } from 'teleport/Discover/useDiscover';

import useTeleportE from 'e-teleport/useTeleportE';
import { useBanner } from 'e-teleport/Banner/useBanner';
import SwitchBack from 'e-teleport/Banner/Switchback';

import { resourceViewConfigs } from './resourceViewConfig';

import type { ResourceSpec } from 'teleport/Discover/SelectResource';

export function Discover() {
  const ctx = useTeleportE();
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

  return <DiscoverComponent eViewConfigs={resourceViewConfigs} />;
}

type Props = {
  agentMeta: AgentMeta;
  resourceSpec: ResourceSpec;
};

export function DiscoverUpdate({ agentMeta, resourceSpec }: Props) {
  return (
    <DiscoverComponent
      eViewConfigs={resourceViewConfigs}
      updateFlow={{ resourceSpec: resourceSpec, agentMeta: agentMeta }}
    />
  );
}
