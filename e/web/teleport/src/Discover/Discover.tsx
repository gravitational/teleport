import SwitchBack from 'e-teleport/Banner/Switchback';
import useTeleportE from 'e-teleport/useTeleportE';
import { DiscoverComponent } from 'teleport/Discover/Discover';
import { SelectResourceSpec } from 'teleport/Discover/SelectResource/resources';
import { AgentMeta } from 'teleport/Discover/useDiscover';

import { resourceViewConfigs } from './resourceViewConfig';

export function Discover() {
  const ctx = useTeleportE();

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
  resourceSpec: SelectResourceSpec;
};

export function DiscoverUpdate({ agentMeta, resourceSpec }: Props) {
  return (
    <DiscoverComponent
      eViewConfigs={resourceViewConfigs}
      updateFlow={{ resourceSpec: resourceSpec, agentMeta: agentMeta }}
    />
  );
}
