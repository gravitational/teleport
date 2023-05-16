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

import * as Features from 'teleport/features';

import type { TeleportFeature } from 'teleport/types';

export function buildACL(features: TeleportFeature[]) {
  const apps = features.some(f => f instanceof Features.FeatureApps);
  const nodes = features.some(f => f instanceof Features.FeatureNodes);
  const audit = features.some(f => f instanceof Features.FeatureAudit);
  const kubes = features.some(f => f instanceof Features.FeatureKubes);
  const databases = features.some(f => f instanceof Features.FeatureDatabases);
  const recordings = features.some(
    f => f instanceof Features.FeatureRecordings
  );
  const desktops = features.some(f => f instanceof Features.FeatureDesktops);

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
