/*
Copyright 2019-2022 Gravitational, Inc.

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

import { useEffect } from 'react';

import Ctx from 'teleport/teleportContext';
import useStickyClusterId from 'teleport/useStickyClusterId';
import { useUrlFiltering } from 'teleport/components/hooks';
import { useInfiniteScroll } from 'teleport/components/hooks/useInfiniteScroll';

/**
 * Retrieves a batch of unified resources from the server, taking into
 * consideration URL filter. Use the returned `fetch` function to fetch the
 * initial batch, and `fetchMore` to support infinite scrolling.
 */
export function useResources(ctx: Ctx) {
  const { clusterId } = useStickyClusterId();

  const { params, search, ...filteringProps } = useUrlFiltering({
    fieldName: 'name',
    dir: 'ASC',
  });

  const { fetch, fetchedData, attempt, fetchMore } = useInfiniteScroll({
    fetchFunc: ctx.resourceService.fetchResources,
    clusterId,
    params,
  });

  useEffect(() => {
    fetch();
  }, [clusterId, search]);

  return {
    ...filteringProps,
    fetchedData,
    clusterId,
    params,
    fetchMore,
    attempt,
  };
}

export type State = ReturnType<typeof useResources>;
