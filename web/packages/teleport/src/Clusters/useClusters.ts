/*
Copyright 2020 Gravitational, Inc.

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

import { useEffect, useState } from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';

import Ctx from 'teleport/teleportContext';
import * as Features from 'teleport/features';

export default function useClusters(ctx: Ctx) {
  const [clusters, setClusters] = useState([]);
  const { attempt, run } = useAttempt();

  function init() {
    run(() => ctx.clusterService.fetchClusters().then(setClusters));
  }

  const [enabledFeatures] = useState(() => buildACL(ctx));

  useEffect(() => {
    init();
  }, []);

  return {
    init,
    initAttempt: attempt,
    clusters,
    enabledFeatures,
  };
}

function buildACL(ctx: Ctx) {
  const apps = ctx.features.some(f => f instanceof Features.FeatureApps);
  const nodes = ctx.features.some(f => f instanceof Features.FeatureNodes);
  const audit = ctx.features.some(f => f instanceof Features.FeatureAudit);
  const kubes = ctx.features.some(f => f instanceof Features.FeatureKubes);
  const databases = ctx.features.some(
    f => f instanceof Features.FeatureDatabases
  );
  const recordings = ctx.features.some(
    f => f instanceof Features.FeatureRecordings
  );
  const desktops = ctx.features.some(
    f => f instanceof Features.FeatureDesktops
  );

  return {
    nodes,
    audit,
    recordings,
    apps,
    kubes,
    databases,
    desktops,
  };
}
