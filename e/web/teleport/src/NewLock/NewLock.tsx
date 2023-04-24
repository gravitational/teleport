/*
Copyright 2023 Gravitational, Inc.

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

import React from 'react';

import { NewLockContent } from 'teleport/Locks/NewLock';

import { lockTargets } from 'teleport/Locks/useGetTargetData';

import useTeleportE from 'e-teleport/useTeleportE';

lockTargets.push({ label: 'Access Request', value: 'access_request' });

export function NewLock() {
  const { workflowService } = useTeleportE();

  return (
    <NewLockContent
      additionalTargets={{
        access_request: {
          fetchData: async () => {
            const requests = await workflowService.fetchAccessRequests({});
            return requests.map(r => ({
              id: r.id,
              user: r.user,
              roles: r.roles.join(', '),
              created: r.created.toDateString(),
              reason: r.requestReason,
              targetValue: r.id,
            }));
          },
        },
      }}
    />
  );
}
