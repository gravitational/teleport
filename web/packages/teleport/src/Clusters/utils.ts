/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
