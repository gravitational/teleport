import React, { useState } from 'react';
import useTeleport from 'teleport/useTeleport';

import { Discover as DiscoverComponent } from 'teleport/Discover/Discover';
import { useDiscover } from 'teleport/Discover/useDiscover';

import Banner from 'e-teleport/Banner';
import getFeatures from 'e-teleport/features';

export function Discover() {
  // Init with enterprise features.
  const [features] = useState(() => getFeatures());
  const ctx = useTeleport();
  const state = useDiscover(ctx, features);

  return (
    <Banner>
      <DiscoverComponent {...state} />
    </Banner>
  );
}
