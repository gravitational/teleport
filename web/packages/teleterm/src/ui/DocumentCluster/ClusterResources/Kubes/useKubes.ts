/*
Copyright 2019 Gravitational, Inc.

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

import { useClusterContext } from 'teleterm/ui/DocumentCluster/clusterContext';

export function useKubes() {
  const ctx = useClusterContext();
  const kubes = ctx.getKubes();
  const syncStatus = ctx.getSyncStatus().kubes;

  return {
    pageSize: 100,
    kubes,
    syncStatus,
    connect: ctx.connectKube,
  };
}

export type State = ReturnType<typeof useKubes>;
